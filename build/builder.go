package build

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/thepwagner/hermit/proxy"
)

type Builder struct {
	log       logr.Logger
	cloner    *GitCloner
	vm        *Firecracker
	selfExe   string
	runDir    string
	outputDir string
}

func NewBuilder(log logr.Logger, cloner *GitCloner, vm *Firecracker, outputDir string) (*Builder, error) {
	selfExe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	b := &Builder{
		log:       log,
		cloner:    cloner,
		vm:        vm,
		runDir:    "/mnt/run",
		outputDir: outputDir,
		selfExe:   selfExe,
	}

	if err := os.MkdirAll(b.runDir, 0750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(b.outputDir, 0750); err != nil {
		return nil, err
	}
	return b, nil
}

type BuildParams struct {
	Owner        string
	Repo         string
	Ref          string
	OutputSizeMB int
}

func (b *Builder) Build(ctx context.Context, params *BuildParams) (*proxy.Snapshot, error) {
	clone, err := b.cloner.Clone(ctx, params.Owner, params.Repo, params.Ref)
	if err != nil {
		return nil, fmt.Errorf("cloning repo: %w", err)
	}
	b.log.Info("source volume created", "src", clone.VolumePath)

	buildTmp, err := ioutil.TempDir(b.runDir, "build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(buildTmp)

	// Use outputDir instead of buildTmp, in case they are on different volumes
	outputTmp, err := TempFile(b.outputDir, fmt.Sprintf("%s-*", params.Ref))
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(outputTmp) }()
	var outputSize int
	if params.OutputSizeMB > 0 {
		outputSize = params.OutputSizeMB
	} else {
		outputSize = 64 // 64MB ought to be enough for anybody
	}
	if err := CreateVolume(ctx, outputTmp, outputSize); err != nil {
		return nil, err
	}
	b.log.Info("output volume created", "src", outputTmp)

	prx, err := b.startProxy(ctx, buildTmp, clone)
	if err != nil {
		return nil, err
	}
	if err := b.vm.BootVM(ctx, clone.VolumePath, outputTmp, buildTmp); err != nil {
		_ = prx.Process.Kill()
		return nil, err
	}
	if err := prx.Process.Signal(syscall.SIGTERM); err != nil {
		_ = prx.Process.Kill()
		return nil, err
	}

	outputMnt, err := MountVolume(ctx, outputTmp, "")
	if err != nil {
		return nil, err
	}
	defer outputMnt.Close(ctx)

	// TODO: actually use image - push to registry?
	outputTar, err := os.Open(filepath.Join(outputMnt.Path(), "image.tar"))
	if err != nil {
		return nil, err
	}
	defer outputTar.Close()
	empty := true
	for tarReader := tar.NewReader(outputTar); ; {
		if _, err := tarReader.Next(); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}
		empty = false
		break
	}
	if empty {
		return nil, fmt.Errorf("no output generated")
	}

	// Rename to the final name, and avoid the deferred deletion.
	if err := os.Rename(outputTmp, filepath.Join(b.outputDir, fmt.Sprintf("%s.img", params.Ref))); err != nil {
		return nil, err
	}

	if err := prx.Wait(); err != nil {
		return nil, err
	}
	b.log.Info("proxy shut down")

	snap, err := proxy.LoadSnapshot(filepath.Join(buildTmp, "proxy-out.json"))
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (b *Builder) startProxy(ctx context.Context, buildTmp string, clone *Clone) (*exec.Cmd, error) {
	vsp := fmt.Sprintf("%s_1024", vsockPath(buildTmp))
	indexOut := filepath.Join(buildTmp, "proxy-out.json")
	args := []string{
		"proxy",
		"--socket", vsp,
		"--index-out", indexOut,
	}

	if !clone.Snapshot.Empty() {
		indexIn := filepath.Join(buildTmp, "proxy-in.json")
		if err := clone.Snapshot.Save(indexIn); err != nil {
			return nil, err
		}
		args = append(args, "--index-in", indexIn)
	}
	if clone.Config != nil {
		config := filepath.Join(buildTmp, "config.yaml")
		if err := clone.Config.Save(config); err != nil {
			return nil, err
		}
		args = append(args, "--config", config)
	}

	b.log.Info("starting proxy", "args", args[1:])
	cmd := exec.CommandContext(ctx, b.selfExe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

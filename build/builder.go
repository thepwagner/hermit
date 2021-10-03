package build

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

type Params struct {
	Owner        string
	Repo         string
	Ref          string
	Hermetic     bool
	OutputSizeMB int
}

type Result struct {
	Snapshot *proxy.Snapshot
	Summary  string
	Output   string
}

func (b *Builder) Build(ctx context.Context, params *Params) (*Result, error) {
	res := &Result{}
	clone, err := b.cloner.Clone(ctx, params.Owner, params.Repo, params.Ref)
	if err != nil {
		res.Summary = "could not clone repo"
		return res, fmt.Errorf("cloning repo: %w", err)
	}
	b.log.Info("source volume created", "src", clone.VolumePath)

	buildTmp, err := ioutil.TempDir(b.runDir, "build-*")
	if err != nil {
		res.Summary = "could not create tempdir for build"
		return res, err
	}
	defer os.RemoveAll(buildTmp)

	// Use outputDir instead of buildTmp, in case they are on different volumes
	outputTmp, err := TempFile(b.outputDir, fmt.Sprintf("%s-*", params.Ref))
	if err != nil {
		res.Summary = "could not create tempfile for output"
		return res, err
	}
	defer func() { _ = os.Remove(outputTmp) }()
	var outputSize int
	if params.OutputSizeMB > 0 {
		outputSize = params.OutputSizeMB
	} else {
		outputSize = 64 // 64MB ought to be enough for anybody
	}
	if err := CreateVolume(ctx, outputTmp, outputSize); err != nil {
		res.Summary = "could not create output voulume"
		return res, err
	}
	b.log.Info("output volume created", "src", outputTmp)

	prx, err := b.startProxy(ctx, buildTmp, clone, params.Hermetic)
	if err != nil {
		res.Summary = "could not start proxy"
		return res, err
	}
	var vmOutput bytes.Buffer
	if err := b.vm.BootVM(ctx, clone.VolumePath, outputTmp, buildTmp, &vmOutput); err != nil {
		_ = prx.Process.Kill()
		res.Summary = "could not boot VM"
		return res, err
	}

	// TODO: silencing logs in the VM would be better than trimming the output
	vmLogs := vmOutput.String()
	if buildStart := strings.Index(vmLogs, "#1 [internal] load build definition from Dockerfile"); buildStart > 0 {
		vmLogs = vmLogs[buildStart:]
	}
	if rebootMsg := strings.LastIndex(vmLogs, "reboot: Restarting system"); rebootMsg > 0 {
		vmLogs = vmLogs[:strings.LastIndex(vmLogs, "[ ")]
	}

	res.Output = fmt.Sprintf("```\n%s\n```\n", vmLogs)

	if err := prx.Process.Signal(syscall.SIGTERM); err != nil {
		_ = prx.Process.Kill()
		res.Summary = "could not stop proxy"
		return res, err
	}

	if output, err := b.checkOutput(ctx, outputTmp); err != nil {
		res.Summary = "could not check VM output"
		return res, err
	} else if !output {
		res.Summary = "build produced no output"
		return res, fmt.Errorf("build did not produce output")
	}

	// Rename to the final name, and avoid the deferred deletion.
	outputFile := buildOutput(b.outputDir, params)
	if err := os.MkdirAll(filepath.Dir(outputFile), 0750); err != nil {
		res.Summary = "build cleanup error"
		return res, err
	}
	if err := os.Rename(outputTmp, outputFile); err != nil {
		res.Summary = "build cleanup error"
		return res, err
	}

	if err := prx.Wait(); err != nil {
		res.Summary = "build cleanup error"
		return res, err
	}
	b.log.Info("proxy shut down")

	snap, err := proxy.LoadSnapshot(filepath.Join(buildTmp, "proxy-out.json"))
	if err != nil {
		res.Summary = "build cleanup error"
		return res, err
	}

	res.Snapshot = snap
	res.Summary = "build succesful"
	return res, nil
}

func buildOutput(outputDir string, p *Params) string {
	return filepath.Join(outputDir, p.Owner, p.Repo, fmt.Sprintf("%s.img", p.Ref))
}

func (b *Builder) startProxy(ctx context.Context, buildTmp string, clone *Clone, hermetic bool) (*exec.Cmd, error) {
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

	var proxyCfg *proxy.Config
	if hermetic {
		proxyCfg = &hermeticCfg
		b.log.Info("proxy using hermetic config")
	} else if clone.Config != nil {
		proxyCfg = clone.Config
		b.log.Info("proxy using repository config")
	}

	if proxyCfg != nil {
		config := filepath.Join(buildTmp, "config.yaml")
		if err := proxyCfg.Save(config); err != nil {
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

func (b *Builder) checkOutput(ctx context.Context, outputTmp string) (bool, error) {
	outputMnt, err := MountVolume(ctx, outputTmp, "")
	if err != nil {
		return false, err
	}
	defer outputMnt.Close(ctx)
	outputTar, err := os.Open(filepath.Join(outputMnt.Path(), "image.tar"))
	if err != nil {
		return false, err
	}
	defer outputTar.Close()

	for tarReader := tar.NewReader(outputTar); ; {
		if _, err := tarReader.Next(); errors.Is(err, io.EOF) {
			return false, nil
		} else if err != nil {
			return false, fmt.Errorf("reading tar: %w", err)
		}

		return true, nil
	}
}

var hermeticCfg proxy.Config

func init() {
	cageMatch, err := proxy.NewRule(".*", proxy.Locked)
	if err != nil {
		panic(err)
	}
	hermeticCfg.Rules = append(hermeticCfg.Rules, cageMatch)
}

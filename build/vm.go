package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-logr/logr"
)

type Firecracker struct {
	log      logr.Logger
	runDir   string
	kernel   string
	rootImg  string
	vsockCID uint32
}

func NewFirecracker(l logr.Logger) (*Firecracker, error) {
	f := &Firecracker{
		log:      l,
		runDir:   "/mnt/run",
		kernel:   "/home/pwagner/hermit/tmp/kernel/vmlinux",
		rootImg:  "/mnt/root.img",
		vsockCID: 2,
	}
	if err := os.MkdirAll(f.runDir, 0750); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Firecracker) BootVM(ctx context.Context, inVolume, outVolume string) error {
	vmDir, err := ioutil.TempDir(f.runDir, "vm-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(vmDir)

	vmRoot := filepath.Join(vmDir, "root.img")
	if err := CopyVolume(ctx, f.rootImg, vmRoot); err != nil {
		return err
	}
	f.log.Info("created vm root", "path", vmRoot)

	vmSrc := filepath.Join(vmDir, "input.img")
	if err := CopyVolume(ctx, inVolume, vmSrc); err != nil {
		return err
	}
	vsockPath := filepath.Join(vmDir, "firecracker-vsock.sock")
	proxy, err := f.startProxy(ctx, vsockPath)
	if err != nil {
		return err
	}
	defer proxy.Process.Kill()

	return f.bootVM(ctx, vmDir, vmRoot, vmSrc, outVolume, vsockPath)
}

func (f *Firecracker) startProxy(ctx context.Context, vsockPath string) (*exec.Cmd, error) {
	f.log.Info("starting proxy", "path", vsockPath)

	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	args := []string{
		"proxy",
		"--socket", fmt.Sprintf("%s_1024", vsockPath),
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	if err := cmd.Start(); err != err {
		return nil, err
	}
	return cmd, nil
}

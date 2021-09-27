package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/go-logr/logr"
)

type Firecracker struct {
	log      logr.Logger
	runDir   string
	kernel   string
	rootImg  string
	vsockCID uint32
}

func NewFirecracker(l logr.Logger) *Firecracker {
	return &Firecracker{
		log:      l,
		runDir:   "/mnt/run",
		kernel:   "/home/pwagner/hermit/tmp/kernel/vmlinux",
		rootImg:  "/mnt/root.img",
		vsockCID: 2,
	}
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
	defer proxy.Kill()

	return f.bootVM(ctx, vmDir, vmRoot, vmSrc, outVolume, vsockPath)
}

func (f *Firecracker) startProxy(ctx context.Context, vsockPath string) (*os.Process, error) {
	f.log.Info("starting proxy", "path", vsockPath)
	attr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys: &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(65534),
				Gid: uint32(65534),
			},
		},
	}
	args := []string{
		os.Args[0],
		"proxy",
		"--socket",
		fmt.Sprintf("%s_1024", vsockPath),
	}
	return os.StartProcess(os.Args[0], args, attr)
}

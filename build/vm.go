package build

import (
	"context"
	"path/filepath"

	"github.com/go-logr/logr"
)

type Firecracker struct {
	log      logr.Logger
	kernel   string
	rootImg  string
	vsockCID uint32
}

func NewFirecracker(l logr.Logger) *Firecracker {
	return &Firecracker{
		log:      l,
		kernel:   "/mnt/vm/vmlinux",
		rootImg:  "/mnt/vm/root.img",
		vsockCID: 2,
	}
}

func vsockPath(buildTmp string) string {
	return filepath.Join(buildTmp, "firecracker-vsock.sock")
}

func (f *Firecracker) BootVM(ctx context.Context, inVolume, outVolume, buildTmp string) error {
	// Make a writable copy of the root image
	vmRoot := filepath.Join(buildTmp, "root.img")
	if err := CopyVolume(ctx, f.rootImg, vmRoot); err != nil {
		return err
	}
	f.log.Info("created vm root", "path", vmRoot)

	return f.bootVM(ctx, buildTmp, vmRoot, inVolume, outVolume)
}

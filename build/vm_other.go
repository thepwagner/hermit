//go:build !linux

package build

import (
	"context"
	"time"
)

func (f *Firecracker) bootVM(ctx context.Context, vmDir, vmRoot, vmSrc, outVolume, vsockPath string) error {
	f.log.Info("faking VM while not on linux")
	time.Sleep(5 * time.Second)
	return nil
}

//go:build !linux

package build

import (
	"context"
	"io"
	"time"
)

func (f *Firecracker) bootVM(ctx context.Context, buildTmp, vmRoot, inVolume, outVolume string, out io.Writer) error {
	f.log.Info("faking VM while not on linux")
	time.Sleep(5 * time.Second)
	return nil
}

//go:build linux

package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

func (f *Firecracker) bootVM(ctx context.Context, vmDir, vmRoot, vmSrc, outVolume, vsockPath string) error {
	fcSockPath := filepath.Join(vmDir, "firecracker.sock")
	vsockCID := atomic.AddUint32(&f.vsockCID, 1)
	cfg := firecracker.Config{
		SocketPath:      fcSockPath,
		KernelImagePath: f.kernel,
		KernelArgs:      "console=ttyS0 noapic reboot=k panic=1 pci=off random.trust_cpu=on nomodules quiet",
		Drives: firecracker.NewDrivesBuilder(vmRoot).
			AddDrive(vmSrc, false).
			AddDrive(outVolume, false).
			Build(),
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(16),
			MemSizeMib: firecracker.Int64(8192),
			HtEnabled:  firecracker.Bool(true),
		},
		VsockDevices: []firecracker.VsockDevice{
			{
				ID:   filepath.Base(vmDir),
				Path: vsockPath,
				CID:  vsockCID,
			},
		},
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin("firecracker").
		WithSocketPath(fcSockPath).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithStdin(os.Stdin).
		Build(ctx)

	m, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		return fmt.Errorf("failed to create firecracker machine: %w", err)
	}
	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("failed to start firecracker machine: %w", err)
	}

	// wait for VMM to execute
	if err := m.Wait(ctx); err != nil {
		return fmt.Errorf("error waiting for firecracker machine: %w", err)
	}
	return nil
}
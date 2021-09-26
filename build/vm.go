package build

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/go-logr/logr"
)

type Firecracker struct {
	log      logr.Logger
	runDir   string
	rootImg  string
	vsockCID uint32
}

func NewFirecracker(l logr.Logger) *Firecracker {
	return &Firecracker{
		log:      l,
		runDir:   "/mnt/run",
		rootImg:  "/mnt/root.img",
		vsockCID: 2,
	}
}

func (f *Firecracker) BootVM(ctx context.Context, srcVolume string) error {
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

	vmSrc := filepath.Join(vmDir, "src.img")
	if err := CopyVolume(ctx, srcVolume, vmSrc); err != nil {
		return err
	}

	fcSockPath := filepath.Join(vmDir, "firecracker.sock")
	// FIXME: hardcoded until proxies are launched
	vsockPath := "/tmp/firecracker-vsock.sock"
	defer os.Remove(vsockPath)
	vsockCID := atomic.AddUint32(&f.vsockCID, 1)

	cfg := firecracker.Config{
		SocketPath:      fcSockPath,
		KernelImagePath: "/home/pwagner/hermit/tmp/kernel/vmlinux",
		KernelArgs:      "console=ttyS0 noapic reboot=k panic=1 pci=off random.trust_cpu=on nomodules quiet",
		Drives:          firecracker.NewDrivesBuilder(vmRoot).AddDrive(vmSrc, false).Build(),
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

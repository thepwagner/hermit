package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/go-logr/logr"
	"github.com/thepwagner/hermit/log"
)

func run(l logr.Logger) error {
	const (
		socketPath = "/tmp/firecracker.sock"
		vsockPath  = "/tmp/firecracker-vsock.sock"
	)

	// Create a temporary directory
	jobDir, err := os.MkdirTemp("/mnt", "boot-*")
	if err != nil {
		return err
	}
	rootImage := filepath.Join(jobDir, "root.img")
	if err := exec.Command("/usr/bin/cp", "--reflink=always", "/mnt/root.img", rootImage).Run(); err != nil {
		return err
	}
	l.Info("created root image", "path", rootImage)
	defer os.RemoveAll(jobDir)

	cfg := firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: "/home/pwagner/hermit/tmp/kernel/vmlinux",
		KernelArgs:      "console=ttyS0 noapic reboot=k panic=1 pci=off random.trust_cpu=on nomodules quiet",
		Drives:          firecracker.NewDrivesBuilder(rootImage).Build(),
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(16),
			MemSizeMib: firecracker.Int64(8192),
			HtEnabled:  firecracker.Bool(true),
		},
		VsockDevices: []firecracker.VsockDevice{
			{
				ID:   "test",
				Path: vsockPath,
				CID:  3,
			},
		},
	}
	defer os.Remove(vsockPath)

	ctx := context.Background()
	cmd := firecracker.VMCommandBuilder{}.
		WithBin("firecracker").
		WithSocketPath(socketPath).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithStdin(os.Stdin).
		Build(ctx)

	m, err := firecracker.NewMachine(ctx, cfg, firecracker.WithProcessRunner(cmd))
	if err != nil {
		panic(fmt.Errorf("failed to create new machine: %v", err))
	}
	if err := m.Start(ctx); err != nil {
		panic(fmt.Errorf("failed to initialize machine: %v", err))
	}

	// wait for VMM to execute
	if err := m.Wait(ctx); err != nil {
		panic(err)
	}

	return nil
}

func main() {
	l := log.New()
	if err := run(l); err != nil {
		l.Error(err, "error")
	}
}

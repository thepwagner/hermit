package main

import (
	"context"
	"fmt"
	"os"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/go-logr/logr"
	"github.com/thepwagner/hermit/log"
)

func run(l logr.Logger) error {
	const socketPath = "/tmp/firecracker.sock"

	cfg := firecracker.Config{
		SocketPath:      socketPath,
		KernelImagePath: "/home/pwagner/git/thepwagner/hermit/tmp/kernel/vmlinux",
		KernelArgs:      "console=ttyS0 noapic reboot=k panic=1 pci=off random.trust_cpu=on nomodules",
		Drives:          firecracker.NewDrivesBuilder("/home/pwagner/git/thepwagner/hermit/tmp/root.img").Build(),
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(1),
			MemSizeMib: firecracker.Int64(512),
			HtEnabled:  firecracker.Bool(true),
		},
	}
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

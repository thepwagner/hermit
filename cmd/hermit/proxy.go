package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

const (
	fileStorageDir = "fileStore"
	proxyIndex     = "index"
	proxySocket    = "socket"
)

// proxyCmd is an internal helper which binds the intercepting proxy to a unix socket.
// Called by other commands to provide a network to virtual machines.
var proxyCmd = &cobra.Command{
	Use:    "proxy",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		fsDir, err := flags.GetString(fileStorageDir)
		if err != nil {
			return err
		}
		indexFile, err := flags.GetString(proxyIndex)
		if err != nil {
			return err
		}
		socketPath, err := flags.GetString(proxySocket)
		if err != nil {
			return err
		}

		l := log.New()
		pk, err := proxy.PrivateKey()
		if err != nil {
			return err
		}
		indexDir := filepath.Join(fsDir, "index")
		if err := os.MkdirAll(indexDir, 0750); err != nil {
			return err
		}
		snap, err := loadSnapshot(indexDir, indexFile)
		if err != nil {
			return err
		}
		defer func() {
			fn, err := snap.Save(indexDir)
			if err != nil {
				l.Error(err, "error saving snapshot")
			} else {
				l.Info("saved snapshot", "file", fn)
			}
		}()

		storage, err := proxy.NewFileStorage(l, filepath.Join(fsDir, "blobs"))
		if err != nil {
			return err
		}
		cachedStorage, err := proxy.NewLRUStorage(128, storage)
		if err != nil {
			return err
		}

		proxyOpts := []proxy.ProxyOpt{
			proxy.ProxyWithLog(l),
			proxy.ProxyWithPrivateKey(pk),
		}
		if socketPath != "" {
			proxyOpts = append(proxyOpts, proxy.ProxyWithSocketPath(socketPath))
		}

		p, err := proxy.NewProxy(proxy.NewSnapshotter(l, snap, cachedStorage), proxyOpts...)
		if err != nil {
			return err
		}
		defer p.Close()

		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
		<-sigC
		return nil
	},
}

func loadSnapshot(indexDir, indexFile string) (*proxy.Snapshot, error) {
	if indexFile == "" {
		return proxy.NewSnapshot(), nil
	}
	indexPath := filepath.Join(indexDir, fmt.Sprintf("%s.json", indexFile))
	return proxy.LoadSnapshot(indexPath)
}

func init() {
	flags := proxyCmd.Flags()
	flags.String(fileStorageDir, "/mnt/storage", "directory for file storage")
	flags.StringP(proxyIndex, "f", "", "index to load")
	flags.String(proxySocket, "", "index to load")
	rootCmd.AddCommand(proxyCmd)
}

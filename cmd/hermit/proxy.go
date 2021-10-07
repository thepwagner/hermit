package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

const (
	fileStorageDir = "fileStore"
	proxyConfig    = "config"
	proxyIndexIn   = "index-in"
	proxyIndexOut  = "index-out"
	proxySocket    = "socket"
)

// proxyCmd is an internal helper which binds the intercepting proxy to a unix socket.
// Called by other commands to provide a network to virtual machines.
var proxyCmd = &cobra.Command{
	Use:    "proxy",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		redis, err := redisClient(cmd)
		if err != nil {
			return err
		}
		cfgFile, err := flags.GetString(proxyConfig)
		if err != nil {
			return err
		}
		indexIn, err := flags.GetString(proxyIndexIn)
		if err != nil {
			return err
		}
		indexOut, err := flags.GetString(proxyIndexOut)
		if err != nil {
			return err
		}
		socketPath, err := flags.GetString(proxySocket)
		if err != nil {
			return err
		}

		l := log.New().WithName("proxy")
		pk, err := proxy.PrivateKey()
		if err != nil {
			return err
		}
		proxyCfg, err := proxy.LoadConfigFile(cfgFile)
		if err != nil {
			return err
		}
		snap, err := proxy.LoadSnapshot(indexIn)
		if err != nil {
			return err
		}
		l.Info("loaded snapshot", "size", snap.Size())
		if indexOut != "" {
			defer func() {
				if err := snap.Save(indexOut); err != nil {
					l.Error(err, "error saving snapshot")
				} else {
					l.Info("saved snapshot", "file", indexOut, "size", snap.Size())
				}
			}()
		}

		proxyOpts := []proxy.ProxyOpt{
			proxy.ProxyWithLog(l),
			proxy.ProxyWithPrivateKey(pk),
		}
		if socketPath != "" {
			proxyOpts = append(proxyOpts, proxy.ProxyWithSocketPath(socketPath))
		}

		storage := proxy.NewRedisStorage(redis, "")
		var h http.Handler = proxy.NewSnapshotter(l, snap, storage)
		ruleCount := len(proxyCfg.Rules)
		l.Info("toggling rules filter", "rules", ruleCount)
		if ruleCount > 0 {
			h = proxy.NewFilter(l, h, proxyCfg.Rules...)
		}

		p, err := proxy.NewProxy(h, proxyOpts...)
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

func init() {
	flags := proxyCmd.Flags()
	flags.String(fileStorageDir, "/mnt/storage", "directory for file storage")
	flags.String(proxyConfig, "", "configuration/rules file to load")
	flags.StringP(proxyIndexIn, "i", "", "index to load")
	flags.StringP(proxyIndexOut, "o", "", "index to write")
	flags.String(proxySocket, "", "unix socket path to bind")
	rootCmd.AddCommand(proxyCmd)
}

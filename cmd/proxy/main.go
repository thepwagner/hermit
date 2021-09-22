package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func run(l logr.Logger) error {
	pk, err := proxy.LoadPrivateKey("key.der")
	if err != nil {
		return err
	}

	// snap := proxy.NewSnapshot()
	snap, err := proxy.LoadSnapshot("cage/index/6a11d7f3641af775ffd8a761cfb2425c51242d389eb9b6dd82e949d6cc7b04da.json")
	if err != nil {
		return err
	}
	defer snap.Save("cage/index")

	storage := proxy.NewFileStorage(l, "cage/blobs")
	p, err := proxy.NewProxy(
		proxy.NewSnapshotter(l, snap, storage),
		proxy.ProxyWithPrivateKey(pk),
		proxy.ProxyWithLog(l),
		proxy.ProxyWithAddr("0.0.0.0:3128"),
	)
	if err != nil {
		return err
	}
	defer p.Close()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	<-sigC
	return nil
}

func main() {
	l := log.New()
	if err := run(l); err != nil {
		l.Error(err, "error")
	}
}

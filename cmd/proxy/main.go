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

	// snap := proxy.NewSnapshot("cage/blobs")
	// defer snap.Save("cage/index")
	snap, err := proxy.LoadSnapshot("cage/index/d68fedc58f95e419f2215491eb1f3f2e09eb260227c51d2979b7597ac4b6471c.json")
	if err != nil {
		return err
	}
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

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
	// pk, err := proxy.LoadPrivateKey("key.der")
	pk, err := proxy.PrivateKey()
	if err != nil {
		return err
	}

	snap := proxy.NewSnapshot()
	// snap, err := proxy.LoadSnapshot("cage/index/3f823020f68c73c037c729039d51bf294d2e79119fffad5419ffc8810c15af95.json")
	// if err != nil {
	// 	return err
	// }
	defer snap.Save("cage/index")

	storage := proxy.NewFileStorage(l, "cage/blobs")
	cachedStorage, err := proxy.NewLRUStorage(128, storage)
	if err != nil {
		return err
	}

	p, err := proxy.NewProxy(
		proxy.NewSnapshotter(l, snap, cachedStorage),
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

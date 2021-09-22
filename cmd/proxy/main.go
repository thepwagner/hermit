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
	snap := proxy.NewSnapshot("cage/blobs")
	defer snap.Save("cage/index")
	// snap, err := proxy.LoadSnapshot("cage/blobs", "cage/index/be5926419d6ba5f3bc5d00480f8843134f078836c7f8ea99e13a216dca47a68b.json")
	// if err != nil {
	// 	return err
	// }

	p, err := proxy.NewProxy(
		proxy.NewSnapshotter(l, snap),
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

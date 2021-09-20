package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func main() {
	l := log.New()

	goog := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "google")
	})

	p, err := proxy.NewProxy(
		proxy.ProxyWithLog(l),
		proxy.ProxyWithAddr("0.0.0.0:3128"),
		proxy.ProxyWithHost("google.ca", goog),
		proxy.ProxyWithHost("google.ca:443", goog),
	)
	if err != nil {
		l.Error(err, "proxy error")
	}
	defer p.Close()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	<-sigC
}

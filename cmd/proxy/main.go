package main

import (
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func main() {
	l := log.New()

	u, _ := url.Parse("http://deb.debian.org")
	deb := httputil.NewSingleHostReverseProxy(u)
	s, _ := url.Parse("http://security.debian.org")
	sec := httputil.NewSingleHostReverseProxy(s)

	p, err := proxy.NewProxy(
		proxy.ProxyWithLog(l),
		proxy.ProxyWithAddr("0.0.0.0:3128"),
		proxy.ProxyWithHost("deb.debian.org", deb),
		proxy.ProxyWithHost("security.debian.org", sec),
	)
	if err != nil {
		l.Error(err, "proxy error")
	}
	defer p.Close()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	<-sigC
}

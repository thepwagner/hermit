package main

import (
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func run(l logr.Logger) error {
	// deb, _ := url.Parse("http://deb.debian.org")
	// sec, _ := url.Parse("http://security.debian.org")
	ms, _ := url.Parse("https://packages.microsoft.com")
	dockerauth, _ := url.Parse("https://auth.docker.io")

	dh, err := proxy.NewDocker(l, "./cage")
	if err != nil {
		return err
	}

	p, err := proxy.NewProxy(
		proxy.ProxyWithLog(l),
		proxy.ProxyWithAddr("0.0.0.0:3128"),
		proxy.ProxyWithHost("deb.debian.org", dh),
		proxy.ProxyWithHost("security.debian.org", dh),
		proxy.ProxyWithHost("packages.microsoft.com:443", httputil.NewSingleHostReverseProxy(ms)),
		proxy.ProxyWithHost("auth.docker.io:443", httputil.NewSingleHostReverseProxy(dockerauth)),
		proxy.ProxyWithHost("registry-1.docker.io:443", dh),
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

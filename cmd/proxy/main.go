package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func main() {
	l := log.New()

	deb, _ := url.Parse("http://deb.debian.org")
	sec, _ := url.Parse("http://security.debian.org")
	ms, _ := url.Parse("https://packages.microsoft.com")

	dh := http.FileServer(http.Dir("/home/pwagner/git/thepwagner/hermit/cage/docker/registry-1.docker.io"))

	p, err := proxy.NewProxy(
		proxy.ProxyWithLog(l),
		proxy.ProxyWithAddr("0.0.0.0:3128"),
		proxy.ProxyWithHost("deb.debian.org", httputil.NewSingleHostReverseProxy(deb)),
		proxy.ProxyWithHost("security.debian.org", httputil.NewSingleHostReverseProxy(sec)),
		proxy.ProxyWithHost("packages.microsoft.com:443", httputil.NewSingleHostReverseProxy(ms)),
		proxy.ProxyWithHost("registry-1.docker.io:443", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.Info("dockerhub", "method", r.Method, "path", r.URL.Path)
			if r.Method == "HEAD" {
				w.WriteHeader(http.StatusOK)
				return
			}

			if strings.HasPrefix(r.URL.Path, "/v2/") {
				if strings.HasPrefix(r.URL.Path, "/v2/") {
					r.URL.Path = r.URL.Path[3:]
				}
				l.Info("file", "path", r.URL.Path, "fn", fmt.Sprintf("%s%s", "/home/pwagner/git/thepwagner/hermit/cage/docker/registry-1.docker.io", r.URL.Path))
				if strings.Contains(r.URL.Path, "/manifests/") {
					w.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
				}
				dh.ServeHTTP(w, r)
			}
		})),
	)
	if err != nil {
		l.Error(err, "proxy error")
	}
	defer p.Close()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	<-sigC
}

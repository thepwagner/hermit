package proxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

type Proxy struct {
	addr string

	log   logr.Logger
	srv   *http.Server
	url   *url.URL
	hosts map[string]http.Handler
	Certs *CertIssuer
}

func NewProxy(opts ...ProxyOpt) (*Proxy, error) {
	pk, err := PrivateKey()
	if err != nil {
		return nil, err
	}
	certs, err := NewCertIssuer(pk)
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		log:   logr.Discard(),
		addr:  "localhost:0",
		hosts: map[string]http.Handler{},
		Certs: certs,
	}
	for _, opt := range opts {
		opt(p)
	}

	l, err := net.Listen("tcp4", p.addr)
	if err != nil {
		return nil, fmt.Errorf("binding listener: %w", err)
	}
	p.log.Info("proxy listening", "addr", l.Addr().String())

	p.srv = &http.Server{Handler: p}
	p.url, _ = url.Parse(fmt.Sprintf("http://%s", l.Addr().(*net.TCPAddr).String()))
	go func() {
		if err := p.srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.log.Error(err, "proxy server error")
		}
	}()
	return p, nil
}

type ProxyOpt func(*Proxy)

func ProxyWithLog(log logr.Logger) ProxyOpt {
	return func(p *Proxy) {
		p.log = log
	}
}

func ProxyWithAddr(addr string) ProxyOpt {
	return func(p *Proxy) {
		p.addr = addr
	}
}

func ProxyWithHost(host string, handler http.Handler) ProxyOpt {
	return func(p *Proxy) {
		p.hosts[host] = handler
	}
}

func (p *Proxy) URL() *url.URL {
	return p.url
}

func (p *Proxy) Close() error {
	p.log.Info("proxy closing")
	return p.srv.Close()
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/.well-known/hermit/proxy-cert" {
		w.Header().Add("Content-Type", "application/x-pem-file")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(p.Certs.CertPEM())
		return
	}

	h := p.handler(r)
	if h == nil {
		http.Error(w, "invalid host", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodConnect {
		host, port, err := net.SplitHostPort(r.Host)
		if err != nil || port != "443" {
			http.Error(w, "bad host", http.StatusServiceUnavailable)
			return
		}
		cert, err := p.Certs.Issue(host)
		if err != nil {
			p.log.Error(err, "proxy issue cert")
			http.Error(w, "could not issue certificate", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)

		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			p.log.Error(err, "proxy issue cert")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		tlsConn := tls.Server(conn, &tls.Config{
			Certificates: []tls.Certificate{*cert},
		})
		if err := tlsConn.Handshake(); err != nil {
			p.log.Error(err, "proxy issue cert")
			return
		}
		defer tlsConn.Close()

		wrapped, sig := wrapHandler(h)
		http.Serve(newConnListener(tlsConn), wrapped)
		<-sig
		return
	}

	h.ServeHTTP(w, r)
}

func (p *Proxy) handler(r *http.Request) http.Handler {
	if h, ok := p.hosts[r.Host]; ok {
		p.log.Info("proxy by host", "host", r.Host)
		return h
	}
	if h, ok := p.hosts["*"]; ok {
		p.log.Info("proxy by wildcard", "host", r.Host)
		return h
	}
	p.log.Info("proxy rejected", "host", r.Host)
	return nil
}

type connListener struct {
	conn chan net.Conn
}

func newConnListener(c net.Conn) *connListener {
	ch := make(chan net.Conn, 1)
	ch <- c
	return &connListener{conn: ch}
}

func (l *connListener) Accept() (c net.Conn, err error) {
	c, ok := <-l.conn
	if !ok {
		err = errors.New("done")
	}
	return
}

func (l *connListener) Close() error {
	return nil
}

func (l *connListener) Addr() net.Addr { return nil }

func wrapHandler(h http.Handler) (http.Handler, <-chan struct{}) {
	sig := make(chan struct{})
	return &chanHandlerWrapper{h: h, signal: sig}, sig
}

type chanHandlerWrapper struct {
	signal chan struct{}
	h      http.Handler
}

func (c *chanHandlerWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Connection", "close")
	c.h.ServeHTTP(w, r)
	close(c.signal)
}

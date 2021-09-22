package proxy

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

type Proxy struct {
	log  logr.Logger
	addr string
	pk   *ecdsa.PrivateKey

	srv         *http.Server
	url         *url.URL
	certs       *CertIssuer
	Snapshotter *Snapshotter
}

func NewProxy(snap *Snapshotter, opts ...ProxyOpt) (*Proxy, error) {
	// pk, err := PrivateKey()
	// if err != nil {
	// 	return nil, err
	// }
	// b, _ := x509.MarshalECPrivateKey(pk)
	// _ = ioutil.WriteFile("key.der", b, 0600)

	b, err := ioutil.ReadFile("key.der")
	if err != nil {
		return nil, err
	}
	pk, err := x509.ParseECPrivateKey(b)
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		log:         logr.Discard(),
		addr:        "localhost:0",
		Snapshotter: snap,
	}
	for _, opt := range opts {
		opt(p)
	}
	certs, err := NewCertIssuer(pk)
	if err != nil {
		return nil, err
	}
	p.certs = certs
	p.log.Info("certificate issuer", "key", fmt.Sprintf("%x", pk.PublicKey.X.Bytes()))

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

func ProxyWithPrivateKey(pk *ecdsa.PrivateKey) ProxyOpt {
	return func(p *Proxy) {
		p.pk = pk
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
	p.log.Info("received request", "method", r.Method, "url", r.URL.String())

	if r.URL.Path == "/.well-known/hermit/proxy-cert" {
		w.Header().Add("Content-Type", "application/x-pem-file")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(p.certs.CertPEM())
		return
	}

	if r.Method != http.MethodConnect {
		p.Snapshotter.ServeHTTP(w, r)
		return
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || port != "443" {
		http.Error(w, "bad host", http.StatusServiceUnavailable)
		return
	}
	cert, err := p.certs.Issue(host)
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

	wrapped, sig := p.signallingHandler(p.Snapshotter, host)
	http.Serve(newConnListener(tlsConn), wrapped)
	<-sig
	return
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

func (p *Proxy) signallingHandler(h http.Handler, host string) (http.Handler, <-chan struct{}) {
	sig := make(chan struct{})
	return &chanHandlerWrapper{h: h, signal: sig, log: p.log, host: host}, sig
}

type chanHandlerWrapper struct {
	signal chan struct{}
	log    logr.Logger
	host   string
	h      http.Handler
}

func (c *chanHandlerWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Scheme = "https"
	r.URL.Host = c.host
	r.Host = c.host
	c.log.Info("intercepted request", "method", r.Method, "url", r.URL.String())
	c.h.ServeHTTP(w, r)
	if w.Header().Get("Connection") == "close" {
		close(c.signal)
	}
}

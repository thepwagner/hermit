package proxy

import (
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

// Proxy acts as an HTTP proxy, and TLS interceptor.
// HTTP CONNECT requests, as a client would issue for an HTTPS server, are intercepted by a self-signed CA.
type Proxy struct {
	handler http.Handler
	log     logr.Logger
	addr    string
	pk      *ecdsa.PrivateKey

	certs *CertIssuer
	srv   *http.Server
	url   *url.URL
}

func NewProxy(handler http.Handler, opts ...ProxyOpt) (*Proxy, error) {
	p := &Proxy{
		log:     logr.Discard(),
		addr:    "localhost:0",
		handler: handler,
	}
	for _, opt := range opts {
		opt(p)
	}

	// Prepare the CA
	if p.pk == nil {
		pk, err := PrivateKey()
		if err != nil {
			return nil, err
		}
		p.pk = pk
	}
	certs, err := NewCertIssuer(p.pk)
	if err != nil {
		return nil, err
	}
	p.certs = certs
	p.log.Info("certificate issuer", "key", fmt.Sprintf("%x", p.pk.PublicKey.X.Bytes()))

	// Bind first, so we can read a randomly assigned port
	l, err := net.Listen("tcp4", p.addr)
	if err != nil {
		return nil, fmt.Errorf("binding listener: %w", err)
	}
	p.log.Info("proxy listening", "addr", l.Addr().String())

	// Embed and start a HTTP server:
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

func (p *Proxy) IssuerCertPEM() []byte {
	return p.certs.CertPEM()
}

func (p *Proxy) Close() error {
	p.log.Info("proxy closing")
	return p.srv.Close()
}

const wellKnownCertPath = "/.well-known/hermit/proxy-cert"

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.log.Info("received request", "method", r.Method, "url", r.URL.String())

	// Handle the internal route to provide the proxy's certificate
	if r.URL.Path == wellKnownCertPath {
		p.serveIssuerCert(w)
		return
	}

	// HTTP requests are passed directly to handler:
	if r.Method != http.MethodConnect {
		p.handler.ServeHTTP(w, r)
		return
	}

	// Issue a certificate for the requested host:
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

	// Pretend to CONNECT, using our certificate. "hey it's me ur brother"
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

	// Serve requests on the intercepted connection using our handler:
	wrapped, sig := p.signallingHandler(p.handler, host)
	http.Serve(newConnListener(tlsConn), wrapped)
	<-sig
	return
}

func (p *Proxy) serveIssuerCert(w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/x-pem-file")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(p.IssuerCertPEM())
}

// signallingHandler wraps the given handler, and returns a channel that is closed when the handler closes the connection.
func (p *Proxy) signallingHandler(h http.Handler, host string) (http.Handler, <-chan struct{}) {
	sig := make(chan struct{})
	return &chanHandlerWrapper{h: h, signal: sig, log: p.log, host: host}, sig
}

type chanHandlerWrapper struct {
	h      http.Handler
	host   string
	signal chan struct{}
	log    logr.Logger
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

// connListener is Accept()s connections from a channel instead of binding a port.
type connListener struct {
	conn chan net.Conn
}

// newConnListener returns a net.Listener that will return a single existing connection.
// Used to bridge an upgraded TLS connection to http.Serve()
func newConnListener(c net.Conn) net.Listener {
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

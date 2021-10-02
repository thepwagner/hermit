package proxy_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

type teapot struct {
	count int64
}

func (t *teapot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&t.count, 1)
	w.WriteHeader(http.StatusTeapot)
	fmt.Fprintln(w, `{"status":["short","stout"]}`)
}

func newTestProxy(t *testing.T, opts ...proxy.ProxyOpt) (*proxy.Proxy, *http.Client) {
	p, err := proxy.NewProxy(&teapot{}, append(opts, proxy.ProxyWithLog(log.New()))...)
	require.NoError(t, err)

	certs := x509.NewCertPool()
	certs.AppendCertsFromPEM(p.IssuerCertPEM())
	c := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(p.URL()),
			TLSClientConfig: &tls.Config{RootCAs: certs},
		},
	}
	return p, c
}

func TestProxy_Http(t *testing.T) {
	p, c := newTestProxy(t)
	defer p.Close()

	res, err := c.Get("http://teapot")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusTeapot, res.StatusCode)

	b, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, b)
}

func TestProxy_Https(t *testing.T) {
	p, c := newTestProxy(t)
	defer p.Close()

	res, err := c.Get("https://teapot")
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusTeapot, res.StatusCode)

	b, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, b)
}

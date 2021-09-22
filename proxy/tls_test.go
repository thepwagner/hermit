package proxy_test

import (
	"crypto/x509"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/proxy"
)

func TestNewCertIssuer(t *testing.T) {
	k, err := proxy.PrivateKey()
	require.NoError(t, err)
	ca, err := proxy.NewCertIssuer(k)
	require.NoError(t, err)

	tlsCert, err := ca.Issue("localhost")
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	require.NoError(t, err)

	// verify cert was issued by CA
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(ca.CertPEM())
	_, err = cert.Verify(x509.VerifyOptions{Roots: caCertPool})
	require.NoError(t, err)
}

func TestLoadSavePrivateKey(t *testing.T) {
	k, err := proxy.PrivateKey()
	require.NoError(t, err)

	fn := filepath.Join(t.TempDir(), "key.der")
	err = proxy.SavePrivateKey(k, fn)
	require.NoError(t, err)

	k2, err := proxy.LoadPrivateKey(fn)
	require.NoError(t, err)
	assert.True(t, k.Equal(k2))
}

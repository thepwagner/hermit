package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"
)

// PrivateKey generates a new key.
func PrivateKey() (*ecdsa.PrivateKey, error) {
	// why not rsa? performance
	// why not ed25519? i fear compatibility
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

type CertIssuer struct {
	caPrivKey *ecdsa.PrivateKey
	caCert    *x509.Certificate
	caCertPEM []byte
	serial    int64
}

func NewCertIssuer(pk *ecdsa.PrivateKey) (*CertIssuer, error) {
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "jonproxley",
			Locality:   []string{"Cincinnati"},
			Province:   []string{"OH"},
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(6 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
	}

	// https://twitter.com/varcharr/status/1438550325280088064
	selfSignedCA, err := x509.CreateCertificate(rand.Reader, ca, ca, &pk.PublicKey, pk)
	if err != nil {
		return nil, fmt.Errorf("self-signing CA certificate: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: selfSignedCA})
	return &CertIssuer{
		caPrivKey: pk,
		caCert:    ca,
		caCertPEM: caCertPEM,
		serial:    2,
	}, nil
}

func (i *CertIssuer) Issue(cn string) (*tls.Certificate, error) {
	now := time.Now().UTC()
	serial := atomic.AddInt64(&i.serial, 1)
	cert := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(2 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
	}

	pk, err := PrivateKey()
	if err != nil {
		return nil, err
	}

	signedCert, err := x509.CreateCertificate(rand.Reader, cert, i.caCert, pk.Public(), i.caPrivKey)
	if err != nil {
		return nil, err
	}

	ret := &tls.Certificate{
		Certificate: [][]byte{signedCert},
		PrivateKey:  pk,
	}
	return ret, nil
}

func (i *CertIssuer) CertPEM() []byte {
	return i.caCertPEM
}

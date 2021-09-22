package proxy

import (
	"crypto/sha256"
	"fmt"
	"net/http/httptest"

	"golang.org/x/crypto/sha3"
)

type URLData struct {
	ContentType   string `json:"contentType,omitempty"`
	ContentLength int    `json:"contentLength,omitempty"`
	Sha256        string `json:"sha256"`
	Shake256      string `json:"shake256"`
}

func NewURLData(r *httptest.ResponseRecorder) *URLData {
	sha256Hash := sha256.Sum256(r.Body.Bytes())
	sha3Hash := make([]byte, 64)
	sha3.ShakeSum256(sha3Hash, r.Body.Bytes())
	return &URLData{
		ContentType:   r.Header().Get("Content-Type"),
		ContentLength: r.Body.Len(),
		Sha256:        fmt.Sprintf("%x", sha256Hash),
		Shake256:      fmt.Sprintf("%x", sha3Hash),
	}
}

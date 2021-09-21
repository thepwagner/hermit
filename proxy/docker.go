package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
)

type Docker struct {
	log   logr.Logger
	proxy http.Handler
	http  *http.Client

	dir     string
	capture bool
	prune   bool // TODO
}

func NewDocker(log logr.Logger, dir string) (*Docker, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Docker{
		log:     log,
		dir:     dir,
		capture: true,
		http:    &http.Client{},
		proxy: &httputil.ReverseProxy{
			Director: func(r *http.Request) {},
		},
	}, nil
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
func (d *Docker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v2/" {
		apiVersionCheck(w)
		return
	}

	if err := d.blobs(w, r); err != nil {
		d.log.Error(err, "handling blob")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func apiVersionCheck(w http.ResponseWriter) {
	w.Header().Add("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (d *Docker) blobs(w http.ResponseWriter, r *http.Request) error {
	// Check local cache for resource
	localPath, err := d.path(r.Host, r.URL.Path)
	if err != nil {
		return err
	}
	if f, err := os.Open(localPath); err == nil {
		d.log.Info("found in cache", "path", localPath)
		defer f.Close()
		if r.Method == http.MethodGet {
			b, err := ioutil.ReadFile(fmt.Sprintf("%s.meta.json", localPath))
			if err == nil {
				var meta ResponseMetadata
				if err := json.Unmarshal(b, &meta); err == nil {
					if meta.ContentType != "" {
						w.Header().Set("Content-Type", meta.ContentType)
					}
				}
			}
			io.Copy(w, f)
		}
		return nil
	}
	d.log.Info("not found in cache", "path", localPath)
	return d.captureResponse(w, r, localPath)
}

func (d *Docker) path(host, path string) (string, error) {
	p := filepath.Join(d.dir, host, path)
	if abs, err := filepath.Abs(p); err != nil {
		return "", fmt.Errorf("resolving abs: %w", err)
	} else if !strings.HasPrefix(abs, d.dir) {
		return "", errors.New("path escapes dir")
	}
	return p, nil
}

type ResponseMetadata struct {
	ContentType string `json:"contentType"`
	Sha256      []byte `json:"sha256"`
}

func (d *Docker) captureResponse(w http.ResponseWriter, r *http.Request, path string) error {
	// Direct proxy if not capturing
	if !d.capture {
		d.proxy.ServeHTTP(w, r)
		return nil
	}

	bufW := httptest.NewRecorder()
	d.proxy.ServeHTTP(bufW, r)
	d.log.Info("proxied request", "url", r.URL.String(), "status", bufW.Code)
	if bufW.Code == http.StatusTemporaryRedirect {
		loc := bufW.Header().Get("Location")
		d.log.Info("following redirect", "location", loc)
		req, _ := http.NewRequest("GET", loc, nil)
		req = req.WithContext(r.Context())
		res, err := d.http.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		var buf bytes.Buffer
		io.Copy(&buf, res.Body)
		bufW.Code = res.StatusCode
		h := bufW.Header()
		for k, v := range res.Header {
			h[k] = v
		}
		bufW.Body = &buf
	}

	var out io.Writer
	if r.Method == http.MethodGet {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return fmt.Errorf("capturing resource: %w", err)
		}

		h := sha256.New()
		h.Write(bufW.Body.Bytes())

		meta, err := json.Marshal(&ResponseMetadata{
			ContentType: bufW.Header().Get("Content-Type"),
			Sha256:      h.Sum(nil),
		})
		if err := ioutil.WriteFile(fmt.Sprintf("%s.meta.json", path), meta, 0600); err != nil {
			return fmt.Errorf("capturing resource meta: %w", err)
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("capturing resource: %w", err)
		}
		defer f.Close()
		out = io.MultiWriter(f, w)
		d.log.Info("captured resource", "code", bufW.Code, "path", path, "len", len(bufW.Body.Bytes()))
	} else {
		out = w
	}

	h := w.Header()
	for k, v := range bufW.Header() {
		h[k] = v
	}
	w.WriteHeader(bufW.Code)
	out.Write(bufW.Body.Bytes())
	return nil
}

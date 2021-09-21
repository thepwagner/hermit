package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/go-logr/logr"
)

var (
	namePattern = regexp.MustCompile("[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*")
	manifest    = regexp.MustCompile(fmt.Sprintf(`^/v2/(%s)/manifests/([^/]+)$`, namePattern))
	blobs       = regexp.MustCompile(fmt.Sprintf(`^/v2/(%s)/blobs/([^/]+)$`, namePattern))
)

type Docker struct {
	log  logr.Logger
	hub  http.Handler
	http *http.Client

	dir     string
	capture bool
	prune   bool // TODO
}

func NewDocker(log logr.Logger, dir string) (*Docker, error) {
	hubURL, _ := url.Parse("https://registry.docker.io")
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Docker{
		log:     log,
		dir:     dir,
		capture: true,
		http:    &http.Client{},
		hub:     httputil.NewSingleHostReverseProxy(hubURL),
	}, nil
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
func (d *Docker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.log.Info("received request", "method", r.Method, "url", r.URL.String())
	if r.URL.Path == "/v2/" {
		apiVersionCheck(w)
		return
	}

	if m := manifest.FindStringSubmatch(r.URL.Path); m != nil {
		img := m[1]
		ref := m[2]
		if err := d.manifests(w, r, img, ref); err != nil {
			d.log.Error(err, "handling manifest")
			http.Error(w, "", http.StatusInternalServerError)
		}
		return
	}

	if m := blobs.FindStringSubmatch(r.URL.Path); m != nil {
		img := m[1]
		ref := m[2]
		if err := d.blobs(w, r, img, ref); err != nil {
			d.log.Error(err, "handling blob")
			http.Error(w, "", http.StatusInternalServerError)
		}
		return
	}
}

func apiVersionCheck(w http.ResponseWriter) {
	w.Header().Add("Docker-Distribution-API-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}

func (d *Docker) manifests(w http.ResponseWriter, r *http.Request, img, ref string) error {
	// Check local cache for manifest
	mfPath, err := d.path(r.Host, img, "manifests", ref)
	if err != nil {
		return err
	}
	if f, err := os.Open(mfPath); err == nil {
		d.log.Info("manifest found in cache", "path", mfPath)
		defer f.Close()
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", schema2.MediaTypeManifest)
			io.Copy(w, f)
		}
		return nil
	}

	w.Header().Set("Content-Type", schema2.MediaTypeManifest)
	return d.captureResponse(w, r, mfPath)
}

func (d *Docker) blobs(w http.ResponseWriter, r *http.Request, img, ref string) error {
	// Check local cache for manifest
	blobPath, err := d.path(r.Host, img, "blobs", ref)
	if err != nil {
		return err
	}
	if f, err := os.Open(blobPath); err == nil {
		d.log.Info("blob found in cache", "path", blobPath)
		defer f.Close()
		if r.Method == http.MethodGet {
			io.Copy(w, f)
		}
		return nil
	}
	d.log.Info("blob not found in cache", "path", blobPath)
	return d.captureResponse(w, r, blobPath)
}

func (d *Docker) path(host, img, resource, ref string) (string, error) {
	p := filepath.Join(d.dir, "docker", host, img, resource, ref)
	if abs, err := filepath.Abs(p); err != nil {
		return "", fmt.Errorf("resolving abs: %w", err)
	} else if !strings.HasPrefix(abs, d.dir) {
		return "", errors.New("path escapes dir")
	}
	return p, nil
}

func (d *Docker) captureResponse(w http.ResponseWriter, r *http.Request, path string) error {
	// Direct proxy if not capturing
	if !d.capture {
		d.hub.ServeHTTP(w, r)
		return nil
	}

	bufW := httptest.NewRecorder()
	d.hub.ServeHTTP(bufW, r)
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
		bufW.Body = &buf
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("capturing resource: %w", err)
	}
	defer f.Close()

	w.WriteHeader(bufW.Code)
	io.MultiWriter(f, w).Write(bufW.Body.Bytes())
	return nil
}

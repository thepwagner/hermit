package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"

	"github.com/go-logr/logr"
)

type Snapshotter struct {
	log     logr.Logger
	snap    *Snapshot
	storage Storage

	capturing bool
	proxy     *httputil.ReverseProxy
	http      *http.Client
}

func NewSnapshotter(log logr.Logger, snap *Snapshot, storage Storage) *Snapshotter {
	return &Snapshotter{
		log:     log,
		snap:    snap,
		storage: storage,
		http:    &http.Client{},
		proxy: &httputil.ReverseProxy{
			Director: func(r *http.Request) {},
		},
		capturing: true,
	}
}

func (s *Snapshotter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := fmt.Sprintf("%s%s", r.Host, r.URL.Path)
	if !(refreshRequest(r) || noStoreRequest(r)) {
		if stored := s.snap.Get(key); stored != nil {
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", stored.ContentType)
				b, err := s.storage.Load(stored)
				if err != nil {
					s.log.Error(err, "failed to get content")
					return
				}
				w.WriteHeader(stored.StatusCode)
				w.Write(b)
			}
			return
		}
	}

	if lockedRequest(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if err := s.captureResponse(w, r, key); err != nil {
		s.log.Error(err, "capturing response")
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (s *Snapshotter) captureResponse(w http.ResponseWriter, r *http.Request, key string) error {
	// Direct proxy if not capturing
	if !s.capturing {
		s.proxy.ServeHTTP(w, r)
		return nil
	}

	bufW := httptest.NewRecorder()
	s.proxy.ServeHTTP(bufW, r)
	s.log.Info("proxied request", "url", r.URL.String(), "status", bufW.Code)
	switch bufW.Code {
	case http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		if err := s.followRedirect(bufW, r); err != nil {
			return err
		}
	default:
		// passthrough
	}
	if r.Method == http.MethodGet && !noStoreRequest(r) {
		data := NewURLData(bufW)
		s.snap.Set(key, data)

		if err := s.storage.Store(data, bufW.Body.Bytes()); err != nil {
			return err
		}
		s.log.Info("captured resource", "code", bufW.Code, "len", len(bufW.Body.Bytes()))
	}

	h := w.Header()
	for k, v := range bufW.Header() {
		h[k] = v
	}
	w.WriteHeader(bufW.Code)
	w.Write(bufW.Body.Bytes())
	return nil
}

func (s *Snapshotter) followRedirect(bufW *httptest.ResponseRecorder, r *http.Request) error {
	loc := bufW.Header().Get("Location")
	s.log.Info("following redirect", "location", loc)
	req, _ := http.NewRequest("GET", loc, nil)
	req = req.WithContext(r.Context())
	res, err := s.http.Do(req)
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
	return nil
}

package proxy_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func TestSnapshotter(t *testing.T) {
	l := log.New()
	snap := proxy.NewSnapshot()

	tmpDir := t.TempDir()
	storage, err := proxy.NewFileStorage(l, tmpDir)
	require.NoError(t, err)
	snapshotter := proxy.NewSnapshotter(l, snap, storage)

	srv := httptest.NewServer(teapot)
	defer srv.Close()

	req, err := http.NewRequest("GET", srv.URL, nil)
	require.NoError(t, err)
	rsp := httptest.NewRecorder()
	snapshotter.ServeHTTP(rsp, req)

	assert.Equal(t, http.StatusTeapot, rsp.Code)

	files, err := ioutil.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

package proxy_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/proxy"
)

func newRule(t *testing.T, pattern string, action proxy.Action) *proxy.Rule {
	rule, err := proxy.NewRule(pattern, action)
	require.NoError(t, err)
	return rule
}

func TestFilter(t *testing.T) {
	l := log.New()
	snap := proxy.NewSnapshot()
	tmpDir := t.TempDir()
	storage, err := proxy.NewFileStorage(l, tmpDir)
	require.NoError(t, err)
	snapshotter := proxy.NewSnapshotter(l, snap, storage)

	filter := proxy.NewFilter(l, snapshotter)
	filter.Rules = append(filter.Rules,
		newRule(t, ".*/reject", proxy.Reject),
		newRule(t, ".*/locked", proxy.Locked),
		newRule(t, ".*/allow", proxy.Allow),
		newRule(t, ".*/refresh", proxy.Refresh),
		newRule(t, ".*/nostore", proxy.RefreshNoStore),
	)

	filterRequest := func(t *testing.T, srvURL, srvPath string) *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", srvURL, srvPath), nil)
		require.NoError(t, err)
		rsp := httptest.NewRecorder()
		filter.ServeHTTP(rsp, req)
		return rsp
	}

	t.Run("reject", func(t *testing.T) {
		srv := httptest.NewServer(&teapot{})
		t.Cleanup(srv.Close)

		rsp := filterRequest(t, srv.URL, "reject")
		assert.Equal(t, http.StatusForbidden, rsp.Code)
	})

	t.Run("locked", func(t *testing.T) {
		teapot := &teapot{}
		srv := httptest.NewServer(teapot)
		t.Cleanup(srv.Close)
		snap.Clear()

		rsp := filterRequest(t, srv.URL, "locked")
		assert.Equal(t, http.StatusForbidden, rsp.Code)
		cnt := atomic.LoadInt64(&teapot.count)
		assert.Equal(t, int64(0), cnt)

		// Request directly through the snapshotter, recording to the cache
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", srv.URL, "locked"), nil)
		require.NoError(t, err)
		snapshotter.ServeHTTP(httptest.NewRecorder(), req)
		cnt = atomic.LoadInt64(&teapot.count)
		assert.Equal(t, int64(1), cnt)

		// Now response is served from storage:
		rsp = filterRequest(t, srv.URL, "locked")
		assert.Equal(t, http.StatusTeapot, rsp.Code)
		cnt = atomic.LoadInt64(&teapot.count)
		assert.Equal(t, int64(1), cnt)
	})

	t.Run("allow", func(t *testing.T) {
		teapot := &teapot{}
		srv := httptest.NewServer(teapot)
		t.Cleanup(srv.Close)
		snap.Clear()

		rsp := filterRequest(t, srv.URL, "allow")
		assert.Equal(t, http.StatusTeapot, rsp.Code)
		cnt := atomic.LoadInt64(&teapot.count)
		assert.Equal(t, int64(1), cnt)

		rsp = filterRequest(t, srv.URL, "allow")
		assert.Equal(t, http.StatusTeapot, rsp.Code)
		cnt = atomic.LoadInt64(&teapot.count)
		assert.Equal(t, int64(1), cnt)
	})

	t.Run("refresh", func(t *testing.T) {
		teapot := &teapot{}
		srv := httptest.NewServer(teapot)
		t.Cleanup(srv.Close)
		snap.Clear()

		for i := 0; i < 2; i++ {
			rsp := filterRequest(t, srv.URL, "refresh")
			assert.Equal(t, http.StatusTeapot, rsp.Code)
			cnt := atomic.LoadInt64(&teapot.count)
			assert.Equal(t, int64(i+1), cnt)
			assert.False(t, snap.Empty())
		}
	})

	t.Run("refresh no store", func(t *testing.T) {
		teapot := &teapot{}
		srv := httptest.NewServer(teapot)
		t.Cleanup(srv.Close)
		snap.Clear()

		snap.Clear()
		for i := 0; i < 2; i++ {
			rsp := filterRequest(t, srv.URL, "nostore")
			assert.Equal(t, http.StatusTeapot, rsp.Code)
			cnt := atomic.LoadInt64(&teapot.count)
			assert.Equal(t, int64(i+1), cnt)
			assert.True(t, snap.Empty())
		}
	})
}

func TestLoadRules(t *testing.T) {
	f, err := os.Open("testdata/rules.yaml")
	require.NoError(t, err)
	defer f.Close()
	rules, err := proxy.LoadRules(f)
	require.NoError(t, err)
	assert.Equal(t, 4, len(rules))

	primeDirective := rules[0]
	assert.True(t, primeDirective.MatchString("auth.docker.io/token"))
	assert.Equal(t, proxy.RefreshNoStore, primeDirective.Action)
}

package proxy_test

import (
	"crypto/rand"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/proxy"
)

func TestRedisStorage(t *testing.T) {
	r := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	s := proxy.NewRedisStorage(r)

	b := make([]byte, 64)
	rand.Read(b)

	d := &proxy.URLData{
		Sha256: "foo",
	}
	err := s.Store(d, b)
	require.NoError(t, err)

	b2, err := s.Load(d)
	require.NoError(t, err)
	assert.Equal(t, b, b2)
}

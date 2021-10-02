package proxy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/proxy"
)

func TestLoadConfigFile(t *testing.T) {
	cfg, err := proxy.LoadConfigFile("testdata/rules.yaml")
	require.NoError(t, err)
	assert.Equal(t, 4, len(cfg.Rules))

	primeDirective := cfg.Rules[0]
	assert.True(t, primeDirective.MatchString("auth.docker.io/token"))
	assert.Equal(t, proxy.RefreshNoStore, primeDirective.Action)
}

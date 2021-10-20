package scan_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/scan"
)

func TestKustomizationFile_NewTag(t *testing.T) {
	k, err := scan.KustomizationFile("testdata/newtag/kustomization.yaml")
	require.NoError(t, err)
	assert.Len(t, k.Images, 1)

	i := k.Images[0]
	assert.Equal(t, "redis", i.Name)
	assert.Empty(t, i.NewName)
	assert.Contains(t, i.NewTag, "@sha256:")
	m, err := regexp.MatchString("^redis:[0-9].*@sha256:[0-9a-f]*$", i.Image())
	require.NoError(t, err)
	assert.True(t, m)
}

func TestKustomizationFile_NewName(t *testing.T) {
	k, err := scan.KustomizationFile("testdata/newname/kustomization.yaml")
	require.NoError(t, err)
	assert.Len(t, k.Images, 1)

	i := k.Images[0]
	assert.Equal(t, "redis", i.Name)
	assert.Equal(t, "redislabs/redis", i.NewName)
	assert.Contains(t, i.NewTag, "@sha256:")
	m, err := regexp.MatchString("^redislabs/redis:[0-9].*@sha256:[0-9a-f]*$", i.Image())
	require.NoError(t, err)
	assert.True(t, m)
}

func TestWalkKustomizations(t *testing.T) {
	ks, err := scan.WalkKustomizations("testdata")
	require.NoError(t, err)
	assert.Len(t, ks, 2)

	images := ks.Images()
	assert.Len(t, images, 2)
}

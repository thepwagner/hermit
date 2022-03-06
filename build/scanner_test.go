package build_test

import (
	"fmt"
	"testing"

	"github.com/aquasecurity/trivy/pkg/types"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/build"
)

func TestRenderReports(t *testing.T) {
	reports := map[string]*types.Report{
		"foo": {
			Metadata: types.Metadata{
				ImageID: "sha256:foobar",
			},
			Results: []types.Result{
				{},
			},
		},
	}

	s, err := build.RenderReports(reports)
	require.NoError(t, err)
	fmt.Println(s)
}

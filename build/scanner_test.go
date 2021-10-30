package build_test

import (
	"fmt"
	"testing"

	"github.com/aquasecurity/trivy/pkg/report"
	"github.com/stretchr/testify/require"
	"github.com/thepwagner/hermit/build"
)

func TestRenderReports(t *testing.T) {
	reports := map[string]*report.Report{
		"foo": {
			Metadata: report.Metadata{
				ImageID: "sha256:foobar",
			},
			Results: []report.Result{
				{},
			},
		},
	}

	s, err := build.RenderReports(reports)
	require.NoError(t, err)
	fmt.Println(s)
}

package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/template"

	"github.com/aquasecurity/trivy/pkg/types"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog/log"
)

type Scanner struct {
	log          logr.Logger
	containerd   *containerd.Client
	scannerImage string
	outputDir    string
}

func NewScanner(log logr.Logger, containerd *containerd.Client, outputDir string) *Scanner {
	return &Scanner{
		log:          log,
		containerd:   containerd,
		scannerImage: "registry.k8s.pwagner.net/library/trivy:latest",
		outputDir:    outputDir,
	}
}

func (s *Scanner) ScanBuildOutput(ctx context.Context, params *Params) (*types.Report, error) {
	outputImg := buildOutput(s.outputDir, params)
	s.log.Info("mounting output volume for scan", "img", outputImg)
	mnt, err := MountVolume(ctx, outputImg, "")
	if err != nil {
		return nil, err
	}
	defer mnt.Close(ctx)

	return s.Scan(ctx, filepath.Join(mnt.Path(), "image.tar"))
}

func (s *Scanner) Scan(ctx context.Context, targetImage string) (*types.Report, error) {
	ctr, err := s.startContainer(ctx, targetImage)
	if err != nil {
		return nil, fmt.Errorf("creating container: %w", err)
	}
	defer func() {
		if err := ctr.Close(context.Background()); err != nil {
			s.log.Error(err, "failed to kill task")
		}
	}()

	statusCh, err := ctr.task.Wait(ctx)
	if err != nil {
		return nil, err
	}
	select {
	case <-statusCh:
	}

	var r types.Report
	if err := json.NewDecoder(ctr.stdout).Decode(&r); err != nil {
		log.Error().Str("out", "stderr").Msg(ctr.stderr.String())
		log.Error().Str("out", "stdout").Msg(ctr.stdout.String())
		return nil, fmt.Errorf("decoding report: %w", err)
	}
	return &r, nil
}

type scanTask struct {
	container containerd.Container
	task      containerd.Task
	stdout    *bytes.Buffer
	stderr    *bytes.Buffer
}

func (s *Scanner) startContainer(ctx context.Context, targetImage string) (*scanTask, error) {
	scannerImg, err := s.containerd.Pull(ctx, s.scannerImage, containerd.WithPullUnpack)
	if err != nil {
		return nil, err
	}
	s.log.Info("starting scan container", "target", targetImage, "scannerImg", scannerImg.Name())

	processArgs := []string{
		"/usr/local/bin/trivy",
		"--cache-dir", "/trivy-db",
		"--quiet",
		"image",
		"--skip-update",
		"--ignore-unfixed",
		"--format", "json",
		"--input", filepath.Join("/input", filepath.Base(targetImage)),
	}
	mounts := []specs.Mount{{
		Type:        "rbind",
		Source:      filepath.Dir(targetImage),
		Destination: "/input",
		Options:     []string{"rbind", "ro"},
	}}

	containerName := fmt.Sprintf("scanner-%s", uuid.NewString())
	imageSpec := []oci.SpecOpts{
		oci.WithProcessArgs(processArgs...),
		oci.WithEnv([]string{"TRIVY_NEW_JSON_SCHEMA=true"}),
		oci.WithMounts(mounts),
	}

	ctr, err := s.containerd.NewContainer(ctx, containerName, containerd.WithNewSnapshot(containerName, scannerImg), containerd.WithNewSpec(imageSpec...))
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	task, err := ctr.NewTask(ctx, cio.NewCreator(cio.WithStreams(nil, &stdout, &stderr)))
	if err != nil {
		_ = ctr.Delete(ctx)
		return nil, err
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx)
		_ = ctr.Delete(ctx)
		return nil, err
	}
	return &scanTask{
		container: ctr,
		task:      task,
		stdout:    &stdout,
		stderr:    &stderr,
	}, nil
}

func (s *scanTask) Close(ctx context.Context) (retErr error) {
	if err := s.task.Kill(ctx, syscall.SIGKILL); err != nil && !strings.Contains(err.Error(), "process already finished") {
		retErr = err
	}
	if _, err := s.task.Delete(ctx); err != nil {
		retErr = err
	}
	if err := s.container.Delete(ctx); err != nil {
		retErr = err
	}
	return
}

var reportMarkdown = template.Must(template.New("report").Parse(`
# Scan Results

` + "`" + `{{.Metadata.ImageID}}` + "`" + `

{{if .Metadata.OS}}
{{.Metadata.OS.Family}} {{.Metadata.OS.Name}} {{if .Metadata.OS.Eosl}}⚠️ End of Life!{{end}}
{{end}}
{{range $result := .Results}}

### {{$result.Type}}

{{$result.Target}}

{{if $result.Vulnerabilities}}
⚠️ {{$result.Vulnerabilities | len}} fixable vulnerabilities found

| Package | Version | FixedVersion | Severity | Description |
|---------|---------|--------------|----------|-------------|
{{range $result.Vulnerabilities}}| {{.PkgName}} | {{.InstalledVersion}} | {{.FixedVersion}} | {{.Severity}} | {{.Description}} |
{{end}}
{{else}}
✅ All good!
{{end}}

{{end}}
`))

func RenderReport(r *types.Report) (string, error) {
	sort.Slice(r.Results, func(i, j int) bool { return r.Results[i].Type < r.Results[j].Type })

	var buf bytes.Buffer
	if err := reportMarkdown.Execute(&buf, r); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type labeledReport struct {
	*types.Report
	Image string
}

var reportsMarkdown = template.Must(template.New("report").Parse(`
# Scan Results

{{range $image := .}}
` + "`" + `{{$image.Image}}` + "`" + `

{{if $image.Metadata.OS}}
{{$image.Metadata.OS.Family}} {{.Metadata.OS.Name}} {{if .Metadata.OS.Eosl}}⚠️ End of Life!{{end}}
{{end}}
{{range $result := $image.Results}}

### {{$result.Type}}

{{$result.Target}}

{{if $result.Vulnerabilities}}
⚠️ {{$result.Vulnerabilities | len}} fixable vulnerabilities found

| Package | Version | FixedVersion | Severity | Description |
|---------|---------|--------------|----------|-------------|
{{range $result.Vulnerabilities}}| {{.PkgName}} | {{.InstalledVersion}} | {{.FixedVersion}} | {{.Severity}} | {{.Description}} |
{{end}}
{{else}}
✅ All good!
{{end}}

{{end}}
{{end}}
`))

func RenderReports(reports map[string]*types.Report) (string, error) {
	labeled := make([]labeledReport, 0, len(reports))
	for image, r := range reports {
		sort.Slice(r.Results, func(i, j int) bool { return r.Results[i].Type < r.Results[j].Type })
		labeled = append(labeled, labeledReport{Report: r, Image: image})
	}
	sort.Slice(labeled, func(i, j int) bool { return labeled[i].Image < labeled[j].Image })

	var buf bytes.Buffer
	if err := reportsMarkdown.Execute(&buf, labeled); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func fullID(ctx context.Context, c containerd.Container) string {
	id := c.ID()
	ns, ok := namespaces.Namespace(ctx)
	if !ok {
		return id
	}
	return fmt.Sprintf("%s-%s", ns, id)
}

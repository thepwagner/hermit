package guest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/util/progress/progressui"
	"golang.org/x/sync/errgroup"
)

type Builder struct {
	log       logr.Logger
	bk        *buildkit.Client
	outputDir string
}

func NewBuilder(ctx context.Context, l logr.Logger, outputDir string) (*Builder, error) {
	bk, err := buildkit.New(ctx, "unix:///run/buildkit/buildkitd.sock")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, err
	}
	return &Builder{
		log:       l,
		bk:        bk,
		outputDir: outputDir,
	}, nil
}

func (b *Builder) Build(ctx context.Context, path string) error {
	// so the "right" thing to do here is dockerfile2llb, then do our dirty work before passing to buildkit
	// aint nobody got time for that, so let's hack some strings together weeeeee
	hackedDockerfile, err := hackDockerfile(path)
	if err != nil {
		return err
	}

	out, err := os.Create(filepath.Join(b.outputDir, "image.tar"))
	if err != nil {
		return err
	}
	defer out.Close()

	rng := make([]byte, 16)
	rand.Read(rng)
	imageName := fmt.Sprintf("hermit-build-%s", hex.EncodeToString(rng))

	solveOpt := buildkit.SolveOpt{
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"build-arg:http_proxy":  "http://127.0.0.1:3128",
			"build-arg:https_proxy": "http://127.0.0.1:3128",
		},
		LocalDirs: map[string]string{
			builder.DefaultLocalNameContext:    path,
			builder.DefaultLocalNameDockerfile: hackedDockerfile,
		},
		Exports: []buildkit.ExportEntry{
			{
				Type: buildkit.ExporterDocker,
				Attrs: map[string]string{
					"name": imageName,
				},
				Output: func(map[string]string) (io.WriteCloser, error) {
					return out, nil
				},
			},
		},
	}
	b.log.Info("running build", "input", path, "output", out, "dockerfile", hackedDockerfile)
	ch := make(chan *buildkit.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		_, err := b.bk.Solve(ctx, nil, solveOpt, ch)
		return err
	})
	eg.Go(func() error {
		return progressui.DisplaySolveStatus(context.Background(), "", nil, os.Stdout, ch)
	})
	return eg.Wait()
}

const (
	hermitCertURL = "http://mitm-me-please/.well-known/hermit/proxy-cert"
	hermitCert    = "/usr/local/share/ca-certificates/hermit.crt"
)

func hackDockerfile(path string) (string, error) {
	df, err := ioutil.ReadFile("Dockerfile")
	if err != nil {
		return "", fmt.Errorf("reading Dockerfile: %w", err)
	}

	var hacked bytes.Buffer
	var open bool
	for _, l := range strings.Split(string(df), "\n") {
		from := strings.HasPrefix(l, "FROM ")
		if from && open {
			fmt.Fprintf(&hacked, "RUN (rm %s && update-ca-certificates) || true\n", hermitCert)
		}
		hacked.WriteString(l + "\n")
		if !from {
			continue
		}
		open = true

		if !strings.Contains(l, "scratch") { // awkwardly bad
			fmt.Fprintf(&hacked, "ADD %s %s\n", hermitCertURL, hermitCert)
			// fmt.Fprintf(&hacked, "ARG GONOSUMDB=*\n")
			// fmt.Fprintf(&hacked, "RUN update-ca-certificates || true\n")
		} else {
			open = false
		}
	}
	if open {
		fmt.Fprintf(&hacked, "RUN (rm %s && update-ca-certificates) || true\n", hermitCert)
	}

	tmpDir, err := ioutil.TempDir("", "hermit-Dockerfile-")
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(filepath.Join(tmpDir, "Dockerfile"), hacked.Bytes(), 0600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}
	return tmpDir, nil
}

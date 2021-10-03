package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	"github.com/go-logr/logr"
)

type Pusher struct {
	log       logr.Logger
	ctr       *containerd.Client
	outputDir string
	registry  string
	resolver  remotes.Resolver
}

func NewPusher(ctx context.Context, log logr.Logger, ctr *containerd.Client, secret, outputDir string) *Pusher {
	hostOptions := config.HostOptions{
		Credentials: func(host string) (string, string, error) {
			fmt.Println("lookup", host)
			if host == "registry.k8s.pwagner.net" {
				return "pwagner", secret, nil
			}
			return "", "", nil
		},
	}
	options := docker.ResolverOptions{Hosts: config.ConfigureHosts(ctx, hostOptions)}
	return &Pusher{
		log:       log,
		ctr:       ctr,
		outputDir: outputDir,
		registry:  "registry.k8s.pwagner.net/library",
		resolver:  docker.NewResolver(options),
	}
}

func (p *Pusher) Push(ctx context.Context, params *Params) error {
	buildVolume := buildOutput(p.outputDir, params)
	mnt, err := MountVolume(ctx, buildVolume, "")
	if err != nil {
		return err
	}
	defer mnt.Close(ctx)

	f, err := os.Open(filepath.Join(mnt.Path(), "image.tar"))
	if err != nil {
		return err
	}
	defer f.Close()
	repoName := fmt.Sprintf("%s/%s:latest", p.registry, params.Repo)
	images, err := p.ctr.Import(ctx, f, containerd.WithImageRefTranslator(archive.AddRefPrefix(repoName)), containerd.WithAllPlatforms(true))
	if err != nil {
		return err
	}
	image := images[0]
	imageService := p.ctr.ImageService()
	imgName := image.Name
	defer func() {
		if err := imageService.Delete(ctx, imgName); err != nil {
			p.log.Error(err, "failed to imported image")
		}
	}()

	// Apply remote tag to image:
	image.Name = repoName
	if _, err := imageService.Create(ctx, image); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return err
		}
		if err := imageService.Delete(ctx, repoName); err != nil {
			return err
		}
		if _, err := imageService.Create(ctx, image); err != nil {
			return err
		}
	}
	defer func() {
		if err := imageService.Delete(ctx, repoName); err != nil {
			p.log.Error(err, "failed to imported image")
		}
	}()

	// Push remote image
	p.log.Info("pushing image", "image", repoName, "digest", image.Target.Digest.String())
	return p.ctr.Push(ctx, repoName, image.Target, containerd.WithResolver(p.resolver))
}

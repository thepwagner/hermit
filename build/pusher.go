package build

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/go-logr/logr"
)

type Pusher struct {
	log       logr.Logger
	docker    *client.Client
	outputDir string
	registry  string
	auth      string
}

func NewPusher(ctx context.Context, log logr.Logger, docker *client.Client, secret, outputDir string) (*Pusher, error) {
	authJSON, err := json.Marshal(types.AuthConfig{
		Username: "pwagner",
		Password: secret,
	})
	if err != nil {
		return nil, err
	}
	return &Pusher{
		log:       log,
		docker:    docker,
		outputDir: outputDir,
		registry:  "registry.k8s.pwagner.net/library",
		auth:      base64.URLEncoding.EncodeToString(authJSON),
	}, nil
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

	// Load image from tar:
	loadRes, err := p.docker.ImageLoad(ctx, f, true)
	if err != nil {
		return err
	}
	defer loadRes.Body.Close()
	var buf bytes.Buffer
	if err := jsonmessage.DisplayJSONMessagesStream(loadRes.Body, &buf, 0, false, nil); err != nil {
		return err
	}
	loadOutput := buf.String()
	if !strings.HasPrefix(loadOutput, "Loaded image: ") {
		return fmt.Errorf("unexpected response: %s", loadOutput)
	}
	imageName := strings.TrimSpace(loadOutput[len("Loaded image: "):])
	p.log.Info("Loaded image", "image", imageName)
	rmOpts := types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	}
	defer func() {
		if _, err := p.docker.ImageRemove(ctx, imageName, rmOpts); err != nil {
			p.log.Error(err, "failed to remove imported image")
		}
	}()

	repoName := fmt.Sprintf("%s/%s:latest", p.registry, params.Repo)
	if err := p.docker.ImageTag(ctx, imageName, repoName); err != nil {
		return err
	}
	defer func() {
		if _, err := p.docker.ImageRemove(ctx, repoName, rmOpts); err != nil {
			p.log.Error(err, "failed to remove tagged image")
		}
	}()

	pushRes, err := p.docker.ImagePush(ctx, repoName, types.ImagePushOptions{RegistryAuth: p.auth})
	if err != nil {
		return err
	}
	defer pushRes.Close()
	if err := jsonmessage.DisplayJSONMessagesStream(pushRes, os.Stdout, 0, false, nil); err != nil {
		return err
	}
	// // Push remote image
	// p.log.Info("pushing image", "image", repoName, "digest", image.Target.Digest.String())
	// return p.ctr.Push(ctx, repoName, image.Target, containerd.WithResolver(p.resolver))
	return nil
}

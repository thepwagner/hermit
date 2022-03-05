package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aquasecurity/trivy/pkg/types"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v43/github"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/scan"
)

// buildRequestQueue is a redis key for the build request queue
const buildRequestQueue = "github-hook-build"

// BuildRequest are parameters to perform a build. What is sent on the build queue.
type BuildRequest struct {
	// TODO: protobuf? your one job is to be stable on the wire

	RepoOwner       string `json:"owner"`
	RepoName        string `json:"name"`
	Ref             string `json:"ref"`
	SHA             string `json:"sha"`
	Tree            string `json:"tree"`
	BuildCheckRunID int64  `json:"buildCheckRunID"`
	DefaultBranch   string `json:"defaultBranch"`
	FromHermit      bool   `json:"fromHermit"`
}

func (r BuildRequest) OnDefaultBranch() bool {
	return r.DefaultBranch == r.Ref
}

// Listener consumes build requests from a Redis queue and performs builds.
type Listener struct {
	log            logr.Logger
	redis          *redis.Client
	gh             *github.Client
	builder        *build.Builder
	ctr            *containerd.Client
	scanner        *build.Scanner
	pusher         *build.Pusher
	snapshotPusher *SnapshotPusher
}

func NewListener(log logr.Logger, redisC *redis.Client, ctr *containerd.Client, gh *github.Client, builder *build.Builder, scanner *build.Scanner, pusher *build.Pusher, snapshotPusher *SnapshotPusher) *Listener {
	return &Listener{
		log:            log,
		redis:          redisC,
		ctr:            ctr,
		gh:             gh,
		builder:        builder,
		scanner:        scanner,
		pusher:         pusher,
		snapshotPusher: snapshotPusher,
	}
}

func (l *Listener) BuildListener(ctx context.Context) {
	for {
		data, err := l.redis.BLPop(ctx, 0, buildRequestQueue).Result()
		if err != nil {
			l.log.Error(err, "failed to get build request")
			continue
		}

		var req BuildRequest
		if err := json.Unmarshal([]byte(data[1]), &req); err != nil {
			l.log.Error(err, "failed to unmarshal build request")
			continue
		}

		if err := l.BuildRequested(ctx, &req); err != nil {
			l.log.Error(err, "failed to process push")
		}
	}
}

func (l *Listener) BuildRequested(ctx context.Context, req *BuildRequest) error {
	l.log.Info("build requested", "repo", fmt.Sprintf("%s/%s", req.RepoOwner, req.RepoName), "sha", req.SHA)
	// Delegate everything: this function should be focused on the CheckRun result

	if err := l.buildCheckRunInProgress(ctx, req); err != nil {
		return err
	}

	if req.RepoName == "gitops" {
		return l.monorepoBuild(ctx, req)
	}

	params := &build.Params{
		Owner:    req.RepoOwner,
		Repo:     req.RepoName,
		Ref:      req.SHA,
		Hermetic: req.OnDefaultBranch() || req.FromHermit,
	}
	result, err := l.builder.Build(ctx, params)
	if err != nil {
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			// Log, but the build error is more interesting to return
			l.log.Error(err, "failed to update checkrun status")
		}
		return err
	}

	defer func() {
		if err := l.builder.Cleanup(params); err != nil {
			l.log.Error(err, "failed to clean up build")
		}
	}()

	pushed, err := l.snapshotPusher.Push(ctx, &SnapshotPushRequest{
		RepoOwner:       req.RepoOwner,
		RepoName:        req.RepoName,
		Ref:             req.Ref,
		DefaultBranch:   req.OnDefaultBranch(),
		BaseTree:        req.Tree,
		ParentCommitSHA: req.SHA,
	}, result.Snapshot)
	if err != nil {
		result.Summary = "snapshot error"
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			l.log.Error(err, "failed to update checkrun status")
		}
		return err
	} else if pushed {
		result.Summary = "snapshot out of date"
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			l.log.Error(err, "failed to update checkrun status")
		}
		return nil
	}

	scan, err := l.scanner.ScanBuildOutput(ctx, params)
	if err != nil {
		return err
	}
	rendered, err := build.RenderReport(scan)
	if err != nil {
		return err
	}

	if err := l.reportScanResult(ctx, req, rendered); err != nil {
		result.Summary = "scan error"
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			l.log.Error(err, "failed to update checkrun status")
		}
		return err
	}

	if req.OnDefaultBranch() {
		l.log.Info("push to default branch, publishing image")
		if err := l.pusher.Push(ctx, params); err != nil {
			result.Summary = "push error"
			if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
				l.log.Error(err, "failed to update checkrun status")
			}
			return err
		}
		l.log.Info("pushed")
	}

	if err := l.buildCheckRunComplete(ctx, req, "success", result); err != nil {
		return err
	}
	return nil
}

func (h *Listener) buildCheckRunInProgress(ctx context.Context, e *BuildRequest) error {
	_, _, err := h.gh.Checks.UpdateCheckRun(ctx, e.RepoOwner, e.RepoName, e.BuildCheckRunID, github.UpdateCheckRunOptions{
		Name:   buildCheckRunName,
		Status: github.String("in_progress"),
	})
	return err
}

func (h *Listener) buildCheckRunComplete(ctx context.Context, e *BuildRequest, conclusion string, res *build.Result) error {
	opts := github.UpdateCheckRunOptions{
		Name:   buildCheckRunName,
		Status: github.String("completed"),
	}
	if conclusion != "" {
		opts.Conclusion = &conclusion
	}
	if res.Summary != "" {
		opts.Output = &github.CheckRunOutput{
			Title:   github.String("Build Output"),
			Summary: &res.Summary,
			Text:    &res.Output,
		}
	}

	_, _, err := h.gh.Checks.UpdateCheckRun(ctx, e.RepoOwner, e.RepoName, e.BuildCheckRunID, opts)
	return err
}

func (l *Listener) monorepoBuild(ctx context.Context, req *BuildRequest) error {
	if req.OnDefaultBranch() {
		return nil
	}

	result := &build.Result{}

	// Get the diff to the default branch
	commitDiff, _, err := l.gh.Repositories.CompareCommits(ctx, req.RepoOwner, req.RepoName, req.DefaultBranch, req.SHA, &github.ListOptions{})
	if err != nil {
		result.Summary = "diff error"
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			// Log, but the build error is more interesting to return
			l.log.Error(err, "failed to update checkrun status")
		}
		return fmt.Errorf("failed to get commit diff: %w", err)
	}
	kustomizations := make([]string, 0, len(commitDiff.Files))
	for _, f := range commitDiff.Files {
		if strings.HasSuffix(f.GetFilename(), "kustomization.yaml") {
			kustomizations = append(kustomizations, f.GetFilename())
		}
	}
	l.log.Info("queried diff", "files", len(commitDiff.Files), "kustomizations", len(kustomizations))

	images := make([]scan.KustomizeImage, 0, len(kustomizations))
	for _, k := range kustomizations {
		fc, _, _, err := l.gh.Repositories.GetContents(ctx, req.RepoOwner, req.RepoName, k, &github.RepositoryContentGetOptions{
			Ref: req.SHA,
		})
		if err != nil {
			if err := l.buildCheckRunComplete(ctx, req, "fetch error", result); err != nil {
				// Log, but the build error is more interesting to return
				l.log.Error(err, "failed to update checkrun status")
			}
			return fmt.Errorf("failed to get kustomization: %w", err)
		}
		kustRaw, err := fc.GetContent()
		if err != nil {
			result.Summary = "fetch error"
			if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
				// Log, but the build error is more interesting to return
				l.log.Error(err, "failed to update checkrun status")
			}
			return fmt.Errorf("failed to get kustomization content: %w", err)
		}
		k, err := scan.ParseKustomization(strings.NewReader(kustRaw))
		if err != nil {
			result.Summary = "fetch error"
			if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
				// Log, but the build error is more interesting to return
				l.log.Error(err, "failed to update checkrun status")
			}
			return err
		}
		images = append(images, k.Images...)
	}
	l.log.Info("identified images", "images", len(images), "kustomizations", len(kustomizations))

	scanResults := make(map[string]*types.Report)
	for _, i := range images {
		img := i.Image()
		scan, err := l.scanImage(ctx, img)
		if err != nil {
			result.Summary = "scan error"
			if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
				// Log, but the build error is more interesting to return
				l.log.Error(err, "failed to update checkrun status")
			}
			return fmt.Errorf("scanning image %q: %w", img, err)
		}
		scanResults[img] = scan
	}

	rendered, err := build.RenderReports(scanResults)
	if err != nil {
		return err
	}

	if err := l.reportScanResult(ctx, req, rendered); err != nil {
		return err
	}

	if err := l.buildCheckRunComplete(ctx, req, "success", result); err != nil {
		return err
	}
	return nil
}

func (l *Listener) scanImage(ctx context.Context, img string) (*types.Report, error) {
	imgHash := sha256.Sum256([]byte(img))
	imageTar := filepath.Join(l.builder.OutputDir, "scan-images", fmt.Sprintf("%x", imgHash), "image.tar")

	l.log.Info("checking for existing image", "image_tar", imageTar)
	if _, err := os.Stat(imageTar); errors.Is(err, os.ErrNotExist) {
		lanImage := scan.LanImage(img)
		l.log.Info("pulling image...", "lan_image", lanImage)
		if ctrImage, err := l.ctr.Pull(ctx, lanImage); err != nil {
			return nil, fmt.Errorf("pulling %q: %w", img, err)
		} else {
			imageSize, err := ctrImage.Size(ctx)
			if err != nil {
				return nil, fmt.Errorf("getting image size: %w", err)
			}
			l.log.Info("pulled image", "image_size", imageSize)

			defer func() {
				if err := l.ctr.ImageService().Delete(ctx, ctrImage.Name()); err != nil {
					l.log.Error(err, "deleting image")
				} else {
					l.log.Info("deleted image")
				}
			}()
		}
		if err := os.MkdirAll(filepath.Dir(imageTar), 0750); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(imageTar, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		if err := l.ctr.Export(ctx, f, archive.WithPlatform(platforms.OnlyStrict(platforms.MustParse("linux/amd64"))), archive.WithImage(l.ctr.ImageService(), lanImage)); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("exporting %q: %w", img, err)
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return l.scanner.Scan(ctx, imageTar)
}

func (l *Listener) reportScanResult(ctx context.Context, params *BuildRequest, rendered string) error {
	branchName := strings.TrimPrefix(params.Ref, "refs/heads/")
	head := fmt.Sprintf("%s:%s", params.RepoOwner, branchName)
	prs, _, err := l.gh.PullRequests.List(ctx, params.RepoOwner, params.RepoName, &github.PullRequestListOptions{
		Head: head,
	})
	if err != nil {
		return err
	}
	l.log.Info("found prs", "prs", len(prs), "head", head)

	for _, pr := range prs {
		comments, _, err := l.gh.Issues.ListComments(ctx, params.RepoOwner, params.RepoName, pr.GetNumber(), &github.IssueListCommentsOptions{})
		if err != nil {
			return err
		}
		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), "# Scan Results") {
				l.log.Info("found existing scan results comment", "pr", pr.GetNumber(), "comment_id", comment.GetID())
				_, _, err = l.gh.Issues.EditComment(ctx, params.RepoOwner, params.RepoName, comment.GetID(), &github.IssueComment{
					Body: &rendered,
				})
				return err
			}
		}

		comment, _, err := l.gh.Issues.CreateComment(ctx, params.RepoOwner, params.RepoName, pr.GetNumber(), &github.IssueComment{
			Body: &rendered,
		})
		if err != nil {
			return err
		}
		l.log.Info("created scan results comment", "pr", pr.GetNumber(), "comment_id", comment.GetID())
		return nil
	}
	return nil
}

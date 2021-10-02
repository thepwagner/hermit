package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/proxy"
	"gopkg.in/yaml.v3"
)

type Handler struct {
	log     logr.Logger
	redis   *redis.Client
	gh      *github.Client
	builder *build.Builder
}

func NewHandler(log logr.Logger, redisC *redis.Client, gh *github.Client, builder *build.Builder) *Handler {
	return &Handler{
		log:     log,
		redis:   redisC,
		gh:      gh,
		builder: builder,
	}
}

func (h *Handler) PushListener(ctx context.Context) {
	for {
		data, err := h.redis.BLPop(ctx, 0, buildRequestQueue).Result()
		if err != nil {
			h.log.Error(err, "failed to get build request")
			continue
		}

		var req BuildRequest
		if err := json.Unmarshal([]byte(data[1]), &req); err != nil {
			h.log.Error(err, "failed to unmarshal build request")
			continue
		}

		if err := h.OnPush(ctx, &req); err != nil {
			h.log.Error(err, "failed to process push")
		}
	}
}

func (h *Handler) OnPush(ctx context.Context, e *BuildRequest) error {
	h.log.Info("building push", "repo", fmt.Sprintf("%s/%s", e.RepoOwner, e.RepoName), "sha", e.SHA)
	if err := h.buildCheckRunStatus(ctx, e, "in_progress", ""); err != nil {
		return err
	}
	snap, err := h.builder.Build(ctx, &build.BuildParams{
		Owner: e.RepoOwner,
		Repo:  e.RepoName,
		Ref:   e.SHA,
	})
	if err != nil {
		if err := h.buildCheckRunStatus(ctx, e, "completed", "failure"); err != nil {
			h.log.Error(err, "failed to update checkrun status")
		}
		return err
	}

	if err := h.pushSnapshot(ctx, e, snap); err != nil {
		if err := h.buildCheckRunStatus(ctx, e, "completed", "failure"); err != nil {
			h.log.Error(err, "failed to update checkrun status")
		}
		return err
	}

	if err := h.buildCheckRunStatus(ctx, e, "completed", "success"); err != nil {
		return err
	}

	// TODO: run your scan sir

	if e.DefaultBranch {
		h.log.Info("push to default branch, publishing")
	}
	return nil
}

func (h *Handler) buildCheckRunStatus(ctx context.Context, e *BuildRequest, status, conclusion string) error {
	opts := github.UpdateCheckRunOptions{
		Name:   buildCheckRunName,
		Status: &status,
	}
	if conclusion != "" {
		opts.Conclusion = &conclusion
	}

	_, _, err := h.gh.Checks.UpdateCheckRun(ctx, e.RepoOwner, e.RepoName, e.BuildCheckRunID, opts)
	return err
}

func (h *Handler) pushSnapshot(ctx context.Context, e *BuildRequest, snap *proxy.Snapshot) error {
	h.gh.Git.GetTree(ctx, e.RepoOwner, e.RepoName, e.Tree, true)

	var entries []*github.TreeEntry
	for host, index := range snap.ByHost() {
		b, err := yaml.Marshal(index)
		if err != nil {
			return err
		}

		entries = append(entries, &github.TreeEntry{
			Path:    github.String(fmt.Sprintf(".hermit/network/%s.yaml", host)),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(string(b)),
		})
	}

	tree, _, err := h.gh.Git.CreateTree(ctx, e.RepoOwner, e.RepoName, e.Tree, entries)
	if err != nil {
		return err
	}
	h.log.Info("created tree", "tree", tree.GetSHA(), "base_tree", e.Tree)
	if tree.GetSHA() == e.Tree {
		return nil
	}

	date := time.Now()
	commit, _, err := h.gh.Git.CreateCommit(ctx, e.RepoOwner, e.RepoName, &github.Commit{
		Tree:    tree,
		Message: github.String("Hermit network snapshot"),
		Author:  &github.CommitAuthor{Name: github.String("Hermit"), Email: github.String("70587923+wapwagner@users.noreply.github.com"), Date: &date},
		Parents: []*github.Commit{{SHA: &e.SHA}},
	})
	if err != nil {
		return err
	}
	h.log.Info("created commit", "commit", commit.GetSHA())

	_, _, err = h.gh.Git.UpdateRef(ctx, e.RepoOwner, e.RepoName, &github.Reference{
		Ref: &e.Ref,
		Object: &github.GitObject{
			SHA: commit.SHA,
		},
	}, false)
	if err != nil {
		return err
	}
	h.log.Info("updated ref, push complete")

	// Still return an error, so the build is mark as failed
	return fmt.Errorf("snapshot out of date")
}

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/thepwagner/hermit/build"
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
	if err := h.pushCheckRunStatus(ctx, e, "in_progress", ""); err != nil {
		return err
	}
	bp := &build.BuildParams{
		Owner: e.RepoOwner,
		Repo:  e.RepoName,
		Ref:   e.SHA,
	}
	if err := h.builder.Build(ctx, bp); err != nil {
		if err := h.pushCheckRunStatus(ctx, e, "completed", "failure"); err != nil {
			h.log.Error(err, "failed to update  checkrun status")
		}
		return err
	}
	if err := h.pushCheckRunStatus(ctx, e, "completed", "success"); err != nil {
		return err
	}

	// TODO: run your scan sir

	if e.DefaultBranch {
		h.log.Info("push to default branch, publishing")
	}
	return nil
}

func (h *Handler) pushCheckRunStatus(ctx context.Context, e *BuildRequest, status, conclusion string) error {
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

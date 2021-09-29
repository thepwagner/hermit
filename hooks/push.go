package hooks

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v39/github"
	"github.com/thepwagner/hermit/build"
)

type Handler struct {
	log logr.Logger

	gh      *github.Client
	builder *build.Builder
}

func NewHandler(log logr.Logger, gh *github.Client, builder *build.Builder) *Handler {
	return &Handler{
		log:     log,
		gh:      gh,
		builder: builder,
	}
}

func (h *Handler) OnPush(ctx context.Context, e *github.PushEvent) error {
	sha := e.GetAfter()
	repo := e.GetRepo()
	owner := repo.GetOwner().GetLogin()
	repoName := repo.GetName()
	h.log.Info("building push", "repo", fmt.Sprintf("%s/%s", owner, repoName), "sha", sha)

	buildCheckRun, _, err := h.gh.Checks.CreateCheckRun(ctx, owner, repoName, github.CreateCheckRunOptions{
		Name:    "build",
		Status:  github.String("in_progress"),
		HeadSHA: sha,
	})
	if err != nil {
		return err
	}

	bp := &build.BuildParams{
		Owner: owner,
		Repo:  repoName,
		Ref:   sha,
	}
	if err := h.builder.Build(ctx, bp); err != nil {
		if _, _, err := h.gh.Checks.UpdateCheckRun(ctx, owner, repoName, buildCheckRun.GetID(), github.UpdateCheckRunOptions{
			Name:       "hermit build",
			Status:     github.String("completed"),
			Conclusion: github.String("failure"),
		}); err != nil {
			return err
		}
		return err
	}
	if _, _, err := h.gh.Checks.UpdateCheckRun(ctx, owner, repoName, buildCheckRun.GetID(), github.UpdateCheckRunOptions{
		Name:       "build",
		Status:     github.String("completed"),
		Conclusion: github.String("success"),
	}); err != nil {
		return err
	}

	// TODO: run your scan sir
	if e.GetRef() == e.GetRepo().GetDefaultBranch() {
		h.log.Info("push to default branch, publishing", "ref", e.GetRef())
	}
	return nil
}

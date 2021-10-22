package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/thepwagner/hermit/build"
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
	DefaultBranch   bool   `json:"defaultBranch"`
	FromHermit      bool   `json:"fromHermit"`
}

// Listener consumes build requests from a Redis queue and performs builds.
type Listener struct {
	log            logr.Logger
	redis          *redis.Client
	gh             *github.Client
	builder        *build.Builder
	scanner        *build.Scanner
	pusher         *build.Pusher
	snapshotPusher *SnapshotPusher
}

func NewListener(log logr.Logger, redisC *redis.Client, gh *github.Client, builder *build.Builder, scanner *build.Scanner, pusher *build.Pusher, snapshotPusher *SnapshotPusher) *Listener {
	return &Listener{
		log:            log,
		redis:          redisC,
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
	params := &build.Params{
		Owner:    req.RepoOwner,
		Repo:     req.RepoName,
		Ref:      req.SHA,
		Hermetic: req.DefaultBranch || req.FromHermit,
	}
	result, err := l.builder.Build(ctx, params)
	if err != nil {
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			// Log, but the build error is more interesting to return
			l.log.Error(err, "failed to update checkrun status")
		}
		return err
	}

	pushed, err := l.snapshotPusher.Push(ctx, &SnapshotPushRequest{
		RepoOwner:       req.RepoOwner,
		RepoName:        req.RepoName,
		Ref:             req.Ref,
		DefaultBranch:   req.DefaultBranch,
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

	if err := l.scanAndReport(ctx, req.Ref, params); err != nil {
		result.Summary = "scan error"
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			l.log.Error(err, "failed to update checkrun status")
		}
		return err
	}

	if req.DefaultBranch {
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

	if err := l.builder.Cleanup(params); err != nil {
		return err
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

func (l *Listener) scanAndReport(ctx context.Context, ref string, params *build.Params) error {
	scan, err := l.scanner.ScanBuildOutput(ctx, params)
	if err != nil {
		return err
	}
	rendered, err := build.RenderReport(scan)
	if err != nil {
		return err
	}

	prs, _, err := l.gh.PullRequests.List(ctx, params.Owner, params.Repo, &github.PullRequestListOptions{
		Head: ref,
	})
	if err != nil {
		return err
	}
	l.log.Info("found prs", "prs", len(prs), "ref", ref)

	for _, pr := range prs {
		comments, _, err := l.gh.Issues.ListComments(ctx, params.Owner, params.Repo, pr.GetNumber(), &github.IssueListCommentsOptions{})
		if err != nil {
			return err
		}
		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), "# Scan Results") {
				l.log.Info("found existing scan results comment", "pr", pr.GetNumber(), "comment_id", comment.GetID())
				_, _, err = l.gh.Issues.EditComment(ctx, params.Owner, params.Repo, comment.GetID(), &github.IssueComment{
					Body: &rendered,
				})
				return err
			}
		}

		comment, _, err := l.gh.Issues.CreateComment(ctx, params.Owner, params.Repo, pr.GetNumber(), &github.IssueComment{
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

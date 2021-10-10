package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/proxy"
	"gopkg.in/yaml.v3"
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
	log     logr.Logger
	redis   *redis.Client
	gh      *github.Client
	builder *build.Builder
	scanner *build.Scanner
	pusher  *build.Pusher
}

func NewListener(log logr.Logger, redisC *redis.Client, gh *github.Client, builder *build.Builder, scanner *build.Scanner, pusher *build.Pusher) *Listener {
	return &Listener{
		log:     log,
		redis:   redisC,
		gh:      gh,
		builder: builder,
		scanner: scanner,
		pusher:  pusher,
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

	if err := l.pushSnapshot(ctx, req, result.Snapshot); err != nil {
		result.Summary = "snapshot error"
		if err := l.buildCheckRunComplete(ctx, req, "failure", result); err != nil {
			l.log.Error(err, "failed to update checkrun status")
		}
		return err
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

func (h *Listener) pushSnapshot(ctx context.Context, req *BuildRequest, snap *proxy.Snapshot) error {
	h.gh.Git.GetTree(ctx, req.RepoOwner, req.RepoName, req.Tree, true)

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

	tree, _, err := h.gh.Git.CreateTree(ctx, req.RepoOwner, req.RepoName, req.Tree, entries)
	if err != nil {
		return err
	}
	h.log.Info("created tree", "tree", tree.GetSHA(), "base_tree", req.Tree)
	if tree.GetSHA() == req.Tree {
		return nil
	}

	date := time.Now()
	commit, _, err := h.gh.Git.CreateCommit(ctx, req.RepoOwner, req.RepoName, &github.Commit{
		Tree:    tree,
		Message: github.String("Hermit network snapshot"),
		Author:  &github.CommitAuthor{Name: github.String("Hermit"), Email: github.String("70587923+wapwagner@users.noreply.github.com"), Date: &date},
		Parents: []*github.Commit{{SHA: &req.SHA}},
	})
	if err != nil {
		return err
	}
	h.log.Info("created commit", "commit", commit.GetSHA())

	_, _, err = h.gh.Git.UpdateRef(ctx, req.RepoOwner, req.RepoName, &github.Reference{
		Ref: &req.Ref,
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

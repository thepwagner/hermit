package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v42/github"
	"github.com/thepwagner/hermit/proxy"
	"gopkg.in/yaml.v3"
)

type SnapshotPusher struct {
	log logr.Logger
	gh  *github.Client
}

func NewSnapshotPusher(log logr.Logger, gh *github.Client) *SnapshotPusher {
	return &SnapshotPusher{
		log: log,
		gh:  gh,
	}
}

type SnapshotPushRequest struct {
	RepoOwner       string
	RepoName        string
	ParentCommitSHA string
	Ref             string
	BaseTree        string
	DefaultBranch   bool
}

func (p *SnapshotPusher) Push(ctx context.Context, req *SnapshotPushRequest, snap *proxy.Snapshot) (pushed bool, err error) {
	// Split the snapshot by host and render as GitHub tree entries:
	var entries []*github.TreeEntry
	for host, index := range snap.ByHost() {
		b, err := yaml.Marshal(index)
		if err != nil {
			return false, err
		}

		entries = append(entries, &github.TreeEntry{
			Path:    github.String(fmt.Sprintf(".hermit/network/%s.yaml", host)),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(string(b)),
		})
	}

	// Create a git tree containing the rendered snapshot
	tree, _, err := p.gh.Git.CreateTree(ctx, req.RepoOwner, req.RepoName, req.BaseTree, entries)
	if err != nil {
		return false, err
	}
	p.log.Info("created tree", "tree", tree.GetSHA(), "base_tree", req.BaseTree)
	if tree.GetSHA() == req.BaseTree {
		// Since the tree hasn't changed, the snapshot matches.
		return false, nil
	}

	// Create a commit from the tree
	date := time.Now()
	commit, _, err := p.gh.Git.CreateCommit(ctx, req.RepoOwner, req.RepoName, &github.Commit{
		Tree:    tree,
		Message: github.String("Hermit network snapshot"),
		Author:  &github.CommitAuthor{Name: github.String("Hermit"), Email: github.String("70587923+wapwagner@users.noreply.github.com"), Date: &date},
		Parents: []*github.Commit{{SHA: &req.ParentCommitSHA}},
	})
	if err != nil {
		return false, err
	}
	p.log.Info("created commit", "commit", commit.GetSHA())

	// Don't push snapshots directly to the default branch, open a PR:
	if req.DefaultBranch {
		branchRef := github.String("refs/heads/hermit-snapshot")
		_, _, err = p.gh.Git.CreateRef(ctx, req.RepoOwner, req.RepoName, &github.Reference{
			Ref: branchRef,
			Object: &github.GitObject{
				SHA: commit.SHA,
			},
		})
		if err != nil {
			return false, err
		}
		p.log.Info("created ref")
		pr, _, err := p.gh.PullRequests.Create(ctx, req.RepoOwner, req.RepoName, &github.NewPullRequest{
			Title: github.String("Hermit snapshot update"),
			Body:  github.String("Your network dependencies changed, here you go."),
			Base:  &req.Ref,
			Head:  branchRef,
		})
		if err != nil {
			return false, err
		}
		p.log.Info("created pull request, push complete", "pr", pr.GetNumber())
		return true, nil
	}

	// Push directly to non-default branches
	_, _, err = p.gh.Git.UpdateRef(ctx, req.RepoOwner, req.RepoName, &github.Reference{
		Ref: &req.Ref,
		Object: &github.GitObject{
			SHA: commit.SHA,
		},
	}, false)
	if err != nil {
		return false, err
	}
	p.log.Info("updated ref, push complete")
	return true, nil
}

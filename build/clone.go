package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v39/github"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type GitCloner struct {
	log         logr.Logger
	gh          *github.Client
	cloneDir    string
	cloneSizeMB int
}

func NewGitCloner(log logr.Logger, gh *github.Client, cloneDir string) *GitCloner {
	return &GitCloner{
		log:         log,
		gh:          gh,
		cloneDir:    cloneDir,
		cloneSizeMB: 512,
	}
}

func (g *GitCloner) Clone(ctx context.Context, owner, repo, commit string) (string, error) {
	f := g.refFile(owner, repo, commit)
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}

	parent, err := g.existingParent(ctx, owner, repo, commit)
	if err != nil {
		return "", nil
	}
	if parent != "" {
		g.log.Info("found existing parent", "parent", parent)
		if err := CopyVolume(parent, f); err != nil {
			return "", err
		}
	} else {
		if err := CreateVolume(ctx, f, g.cloneSizeMB); err != nil {
			return "", err
		}
	}

	mnt, err := MountVolume(ctx, f, "")
	if err != nil {
		return "", err
	}
	defer mnt.Close(ctx)

	var wt *git.Worktree
	if parent != "" {
		r, err := git.PlainOpen(mnt.Path())
		if err != nil {
			return "", err
		}
		wt, err = r.Worktree()
		if err != nil {
			return "", err
		}
		if err := wt.Pull(&git.PullOptions{}); err != nil {
			return "", fmt.Errorf("pulling to update: %w", err)
		}
	} else {
		r, err := git.PlainClone(mnt.Path(), false, &git.CloneOptions{
			URL: fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		})
		if err != nil {
			return "", err
		}
		wt, err = r.Worktree()
		if err != nil {
			return "", err
		}
	}

	if err := wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	}); err != nil {
		return "", fmt.Errorf("checking out: %w", err)
	}

	return "", nil
}

func (g *GitCloner) refFile(owner, repo, ref string) string {
	return filepath.Join(g.cloneDir, owner, repo, fmt.Sprintf("%s.img", ref))
}

func (g *GitCloner) existingParent(ctx context.Context, owner, repo, commit string) (string, error) {
	commits, _, err := g.gh.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{
		SHA: commit,
	})
	if err != nil {
		return "", fmt.Errorf("listing commits: %w", err)
	}
	g.log.Info("listed commits", "commits", len(commits))
	for _, c := range commits {
		f := g.refFile(owner, repo, c.GetSHA())
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}
	return "", nil
}

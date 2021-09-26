package build

import (
	"context"
	"errors"
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
	// Has this commit already been checked out?
	f := g.refFile(owner, repo, commit)
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}

	// Build in a temporary file that will be renamed to `f` when complete, to avoid races.
	tmpFile, err := TempFile(filepath.Dir(f), "volume-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile)

	// Skim the git history for any parents that are already checked out:
	parent, err := g.existingParent(ctx, owner, repo, commit)
	if err != nil {
		return "", nil
	}
	if parent != "" {
		g.log.Info("found existing parent", "src", f, "parent", parent)
		if err := CopyVolume(ctx, parent, tmpFile); err != nil {
			return "", err
		}
	} else {
		g.log.Info("creating volume", "src", f)
		if err := CreateVolume(ctx, tmpFile, g.cloneSizeMB); err != nil {
			return "", err
		}
	}

	// Mount the volume, which may be empty or seeded from a parent:
	mnt, err := MountVolume(ctx, tmpFile, "")
	if err != nil {
		return "", err
	}
	defer mnt.Close(ctx)
	g.log.Info("mounted volume", "src", f, "mnt", mnt.Path())

	// Refresh the git clone therein.
	var r *git.Repository
	if parent != "" {
		g.log.Info("fetching to update volume", "mnt", mnt.Path(), "parent", parent)
		r, err = git.PlainOpen(mnt.Path())
		if err != nil {
			return "", err
		}
		if err := r.FetchContext(ctx, &git.FetchOptions{}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "", fmt.Errorf("fetching to update: %w", err)
		}
	} else {
		g.log.Info("cloning in to volume", "mnt", mnt.Path())
		r, err = git.PlainClone(mnt.Path(), false, &git.CloneOptions{
			URL: fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
		})
		if err != nil {
			return "", err
		}
	}

	// Finally, check out the target commit:
	wt, err := r.Worktree()
	if err != nil {
		return "", err
	}
	if err := wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	}); err != nil {
		return "", fmt.Errorf("checking out: %w", err)
	}

	// Finalize image:
	if err := os.Rename(tmpFile, f); err != nil {
		return "", fmt.Errorf("renaming: %w", err)
	}
	return f, nil
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

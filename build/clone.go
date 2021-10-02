package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v39/github"
	"github.com/thepwagner/hermit/proxy"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

type GitCloner struct {
	log         logr.Logger
	gh          *github.Client
	auth        transport.AuthMethod
	cloneDir    string
	cloneSizeMB int
}

func NewGitCloner(log logr.Logger, gh *github.Client, ghToken, cloneDir string) *GitCloner {
	return &GitCloner{
		log:      log,
		gh:       gh,
		cloneDir: cloneDir,
		auth: &http.BasicAuth{
			Username: "x-access-token",
			Password: ghToken,
		},
		cloneSizeMB: 512,
	}
}

type Clone struct {
	VolumePath string
	Snapshot   *proxy.Snapshot
	Config     *proxy.Config
}

func (g *GitCloner) Clone(ctx context.Context, owner, repo, commit string) (*Clone, error) {
	// Has this commit already been checked out?
	f := g.refFile(owner, repo, commit)
	if _, err := os.Stat(f); err == nil {
		mnt, err := MountVolume(ctx, f, "")
		if err != nil {
			return nil, err
		}
		defer mnt.Close(ctx)
		g.log.Info("mounted existing volume", "src", f, "mnt", mnt.Path())

		r, err := git.PlainOpen(mnt.Path())
		if err != nil {
			return nil, err
		}
		h, err := r.Head()
		if err != nil {
			return nil, err
		}
		g.log.Info("opened existing repo", "src", f, "name", h.Name(), "head", h.Hash().String())
		return g.loadCloneDetails(f, mnt.Path())
	}

	// Build in a temporary file that will be renamed to `f` when complete, to avoid races.
	repoStorageDir := filepath.Dir(f)
	if err := os.MkdirAll(repoStorageDir, 0750); err != nil {
		return nil, err
	}
	tmpFile, err := TempFile(repoStorageDir, "volume-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile)

	// Skim the git history for any parents that are already checked out:
	parent, err := g.existingParent(ctx, owner, repo, commit)
	if err != nil {
		return nil, err
	}
	if parent != "" {
		g.log.Info("found existing parent", "src", f, "parent", parent)
		if err := CopyVolume(ctx, parent, tmpFile); err != nil {
			return nil, err
		}
	} else {
		g.log.Info("creating volume", "src", f)
		if err := CreateVolume(ctx, tmpFile, g.cloneSizeMB); err != nil {
			return nil, err
		}
	}

	// Mount the volume, which may be empty or seeded from a parent:
	mnt, err := MountVolume(ctx, tmpFile, "")
	if err != nil {
		return nil, err
	}
	defer mnt.Close(ctx)
	g.log.Info("mounted volume", "src", f, "mnt", mnt.Path())

	// Refresh the git clone therein.
	var r *git.Repository
	if parent != "" {
		g.log.Info("fetching to update volume", "mnt", mnt.Path(), "parent", parent)
		r, err = git.PlainOpen(mnt.Path())
		if err != nil {
			return nil, err
		}
		if err := r.FetchContext(ctx, &git.FetchOptions{Auth: g.auth}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil, fmt.Errorf("fetching to update: %w", err)
		}
	} else {
		g.log.Info("cloning in to volume", "mnt", mnt.Path())
		r, err = git.PlainClone(mnt.Path(), false, &git.CloneOptions{
			URL:  fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
			Auth: g.auth,
		})
		if err != nil {
			return nil, err
		}
	}

	// Check out the target commit:
	wt, err := r.Worktree()
	if err != nil {
		return nil, err
	}
	if err := wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(commit),
	}); err != nil {
		return nil, fmt.Errorf("checking out: %w", err)
	}
	h, err := r.Head()
	if err != nil {
		return nil, err
	}

	g.log.Info("opened updated repo", "src", f, "name", h.Name(), "head", h.Hash().String())
	clone, err := g.loadCloneDetails(f, mnt.Path())
	if err != nil {
		return nil, err
	}

	// Finalize image:
	if err := os.Rename(tmpFile, f); err != nil {
		return nil, fmt.Errorf("renaming: %w", err)
	}
	return clone, nil
}

func (g *GitCloner) loadCloneDetails(volumePath, mntPath string) (*Clone, error) {
	snap, err := proxy.LoadSnapshot(filepath.Join(mntPath, ".hermit", "network"))
	if err != nil {
		return nil, err
	}
	g.log.Info("loaded snapshot", "urls", len(snap.Data))

	cfg, err := proxy.LoadConfigFile(filepath.Join(mntPath, ".hermit", "rules.yaml"))
	if err != nil {
		return nil, err
	}
	g.log.Info("loaded config", "urls", len(cfg.Rules))

	return &Clone{
		VolumePath: volumePath,
		Snapshot:   snap,
		Config:     cfg,
	}, nil
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

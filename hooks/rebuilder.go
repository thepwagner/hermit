package hooks

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v39/github"
	"github.com/robfig/cron/v3"
	"github.com/thepwagner/hermit/build"
)

// Rebuilder is a period rebuilder of project that opens pull requests when snapshots change
type Rebuilder struct {
	log          logr.Logger
	gh           *github.Client
	builder      *build.Builder
	snapshotPush *SnapshotPusher
	cron         *cron.Cron
}

func NewRebuilder(log logr.Logger, gh *github.Client, builder *build.Builder, snapshotPush *SnapshotPusher) *Rebuilder {
	return &Rebuilder{
		log:          log,
		gh:           gh,
		builder:      builder,
		snapshotPush: snapshotPush,
		cron:         cron.New(),
	}
}

func (r *Rebuilder) Cron(schedule, owner, repo, ref string) {
	r.cron.AddFunc(schedule, func() {
		if err := r.Rebuild(context.Background(), owner, repo, ref); err != nil {
			r.log.Error(err, "failed to rebuild", "owner", owner, "repo", repo)
		}
	})
}

func (r *Rebuilder) Start() {
	r.log.Info("starting rebuilder", "jobs", len(r.cron.Entries()))
	r.cron.Start()
	for _, e := range r.cron.Entries() {
		r.log.Info("job scheduled", "job", e.ID, "next", e.Next)
	}
}

func (r *Rebuilder) Stop() {
	r.cron.Stop()
}

func (r *Rebuilder) Rebuild(ctx context.Context, owner, repo, ref string) error {
	ghRepo, _, err := r.gh.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return err
	}
	branch, _, err := r.gh.Repositories.GetBranch(ctx, owner, repo, ref, true)
	if err != nil {
		return err
	}
	defaultBranch := ghRepo.GetDefaultBranch() == branch.GetName()
	sha := branch.GetCommit().GetSHA()
	r.log.Info("resolved ref", "owner", owner, "repo", repo, "ref", ref, "sha", sha, "default", defaultBranch)

	params := &build.Params{
		Owner:    owner,
		Repo:     repo,
		Ref:      sha,
		Hermetic: false,
	}
	result, err := r.builder.Build(ctx, params)
	if err != nil {
		return err
	}
	pushed, err := r.snapshotPush.Push(ctx, &SnapshotPushRequest{
		RepoOwner:       owner,
		RepoName:        repo,
		Ref:             ref,
		BaseTree:        branch.GetCommit().Commit.GetTree().GetSHA(),
		ParentCommitSHA: branch.GetCommit().GetSHA(),
		DefaultBranch:   defaultBranch,
	}, result.Snapshot)
	if err != nil {
		return err
	}
	if err := r.builder.Cleanup(params); err != nil {
		return err
	}
	r.log.Info("rebuild complete", "pushed_snapshot", pushed)
	return nil
}

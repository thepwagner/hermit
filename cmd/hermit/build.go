package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/hooks"
	"github.com/thepwagner/hermit/log"
)

const (
	repoOwner   = "owner"
	repoName    = "repo"
	repoRef     = "ref"
	rebuildFlag = "rebuild"

	srcDir    = "/mnt/src"
	outputDir = "/mnt/output"
)

var buildCmd = &cobra.Command{
	Use: "build",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := log.New().WithName("build")
		flags := cmd.Flags()
		owner, err := flags.GetString(repoOwner)
		if err != nil {
			return err
		}
		repo, err := flags.GetString(repoName)
		if err != nil {
			return err
		}
		ref, err := flags.GetString(repoRef)
		if err != nil {
			return err
		}
		rebuild, err := flags.GetBool(rebuildFlag)
		if err != nil {
			return err
		}

		pushSecret := os.Getenv("REGISTRY_PUSH_PASSWORD")
		l.Info("building", "owner", owner, "repo", repo, "ref", ref)

		// Resolve ref to SHA
		ctx := cmd.Context()
		_, gh, err := gitHubClient(cmd)
		if err != nil {
			return err
		}
		ghRepo, _, err := gh.Repositories.Get(ctx, owner, repo)
		if err != nil {
			return err
		}
		branch, _, err := gh.Repositories.GetBranch(ctx, owner, repo, ref, true)
		if err != nil {
			return err
		}
		defaultBranch := ghRepo.GetDefaultBranch() == branch.GetName()
		sha := branch.GetCommit().GetSHA()
		l.Info("resolved ref", "owner", owner, "repo", repo, "ref", ref, "sha", sha, "default", defaultBranch)

		docker, err := client.NewEnvClient()
		if err != nil {
			return err
		}
		imagePush, err := build.NewPusher(ctx, l, docker, pushSecret, outputDir)
		if err != nil {
			return err
		}
		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}
		snapshotPush := hooks.NewSnapshotPusher(l, gh)

		params := &build.Params{
			Owner:    owner,
			Repo:     repo,
			Ref:      sha,
			Hermetic: defaultBranch && !rebuild,
		}
		result, err := builder.Build(ctx, params)
		if result != nil {
			fmt.Println("---Build Result---")
			if result.Summary != "" {
				fmt.Println(result.Summary)
			}
			if result.Output != "" {
				fmt.Println(result.Output)
			}
		}
		if err != nil {
			return err
		}

		if rebuild {
			pushed, err := snapshotPush.Push(ctx, &hooks.SnapshotPushRequest{
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
			l.Info("rebuild complete", "pushed_snapshot", pushed)
		} else if defaultBranch {
			if err := imagePush.Push(ctx, params); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	flags := buildCmd.Flags()
	flags.String(repoOwner, "thepwagner-org", "GitHub repository owner")
	flags.StringP(repoName, "r", "sonarr", "GitHub repository name")
	flags.String(repoRef, "main", "GitHub repository ref")
	flags.Bool(rebuildFlag, false, "Rebuild image")
	rootCmd.AddCommand(buildCmd)
}

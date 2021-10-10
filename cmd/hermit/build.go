package main

import (
	"fmt"
	"os"

	"github.com/containerd/containerd"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
)

const (
	repoOwner = "owner"
	repoName  = "repo"
	repoRef   = "ref"

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

		ctr, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("hermit"))
		if err != nil {
			return err
		}
		pusher := build.NewPusher(ctx, l, ctr, pushSecret, outputDir)

		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}
		params := &build.Params{
			Owner:    owner,
			Repo:     repo,
			Ref:      sha,
			Hermetic: defaultBranch,
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

		if defaultBranch {
			if err := pusher.Push(ctx, params); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	flags := buildCmd.Flags()
	flags.String(repoOwner, "thepwagner-org", "GitHub repository owner")
	flags.StringP(repoName, "r", "debian-bullseye", "GitHub	 repository name")
	flags.String(repoRef, "main", "GitHub repository ref")
	rootCmd.AddCommand(buildCmd)
}

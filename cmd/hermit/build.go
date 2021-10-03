package main

import (
	"fmt"

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

		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}
		result, err := builder.Build(ctx, &build.Params{
			Owner:    owner,
			Repo:     repo,
			Ref:      sha,
			Hermetic: defaultBranch,
		})

		if result != nil {
			fmt.Println("---Build Result---")
			if result.Summary != "" {
				fmt.Println(result.Summary)
			}
			if result.Output != "" {
				fmt.Println(result.Output)
			}
		}
		return err
	},
}

func init() {
	flags := buildCmd.Flags()
	flags.String(repoOwner, "thepwagner-org", "GitHub repository owner")
	flags.StringP(repoName, "r", "debian-bullseye", "GitHub	 repository name")
	flags.String(repoRef, "main", "GitHub repository ref")
	rootCmd.AddCommand(buildCmd)
}

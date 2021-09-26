package main

import (
	"net/http"

	"github.com/google/go-github/v39/github"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
)

const (
	repoOwner = "owner"
	repoName  = "repo"
	repoRef   = "ref"

	srcDir = "/mnt/src"
)

var buildCmd = &cobra.Command{
	Use: "build",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := log.New()
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

		gh := github.NewClient(&http.Client{})
		cloner := build.NewGitCloner(l, gh, srcDir)
		ctx := cmd.Context()
		src, err := cloner.Clone(ctx, owner, repo, ref)
		if err != nil {
			return err
		}
		l.Info("source volume created", "src", src)

		fc := build.NewFirecracker(l)
		return fc.BootVM(ctx, src)
	},
}

func init() {
	buildCmd.Flags().StringP(repoOwner, "o", "thepwagner", "GitHub repository owner")
	buildCmd.Flags().StringP(repoName, "r", "archivist", "GitHub repository name")
	buildCmd.Flags().String(repoRef, "3817d505e8bb39f43287256f3086f82e4b56374b", "GitHub repository ref")
	rootCmd.AddCommand(buildCmd)
}

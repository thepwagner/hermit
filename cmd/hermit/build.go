package main

import (
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
		indexFile, err := flags.GetString(proxyIndexIn)
		if err != nil {
			return err
		}
		l.Info("building", "owner", owner, "repo", repo, "ref", ref)

		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}
		return builder.Build(cmd.Context(), &build.BuildParams{
			Owner:      owner,
			Repo:       repo,
			Ref:        ref,
			ProxyIndex: indexFile,
		})
	},
}

func init() {
	flags := buildCmd.Flags()
	flags.String(repoOwner, "thepwagner-org", "GitHub repository owner")
	flags.StringP(repoName, "r", "debian-bullseye", "GitHub repository name")
	flags.String(repoRef, "a055335207b183e2d0c4b9f8c04e0e9877d87eba", "GitHub repository ref")
	// TODO: this should be read from the ref instead
	flags.StringP(proxyIndexIn, "i", "", "index to load")
	rootCmd.AddCommand(buildCmd)
}

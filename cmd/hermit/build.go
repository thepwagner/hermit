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

		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}
		result, err := builder.Build(cmd.Context(), &build.BuildParams{
			Owner: owner,
			Repo:  repo,
			Ref:   ref,
		})
		if err != nil {
			return err
		}

		fmt.Println("---Build Result---")
		if result.Summary != "" {
			fmt.Println(result.Summary)
		}
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		return nil
	},
}

func init() {
	flags := buildCmd.Flags()
	flags.String(repoOwner, "thepwagner-org", "GitHub repository owner")
	flags.StringP(repoName, "r", "debian-bullseye", "GitHub	 repository name")
	flags.String(repoRef, "5e3e9ff889d68562d535f102ee17630e2c7f5117", "GitHub repository ref")
	rootCmd.AddCommand(buildCmd)
}

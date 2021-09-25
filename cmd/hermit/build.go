package main

import (
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/log"
)

const (
	buildFlagRepo = "repo"
)

var buildCmd = &cobra.Command{
	Use: "build",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := log.New()
		repo, err := cmd.Flags().GetString(buildFlagRepo)
		if err != nil {
			return err
		}
		l.Info("building", "repo", repo)
		return nil
	},
}

func init() {
	buildCmd.Flags().StringP(buildFlagRepo, "r", "thepwagner/archivist", "GitHub repository")

	rootCmd.AddCommand(buildCmd)
}

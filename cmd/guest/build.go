package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
)

var buildCmd = &cobra.Command{
	Use: "build",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		b, err := build.NewBuilder(ctx)
		if err != nil {
			return err
		}

		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		return b.Build(ctx, wd)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

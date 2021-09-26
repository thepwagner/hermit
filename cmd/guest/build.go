package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
)

var buildCmd = &cobra.Command{
	Use: "build",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		l := log.New()
		b, err := build.NewBuilder(ctx, l, "/output")
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

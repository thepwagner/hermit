package main

import (
	"os"

	"github.com/containerd/containerd"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/hooks"
	"github.com/thepwagner/hermit/log"
)

var listenerCmd = &cobra.Command{
	Use: "listener",
	RunE: func(cmd *cobra.Command, args []string) error {
		redis, err := redisClient(cmd)
		if err != nil {
			return err
		}
		_, gh, err := gitHubClient(cmd)
		if err != nil {
			return err
		}
		pushSecret := os.Getenv("REGISTRY_PUSH_PASSWORD")

		l := log.New()
		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}
		ctr, err := containerd.New("/run/containerd/containerd.sock", containerd.WithDefaultNamespace("hermit"))
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		pusher := build.NewPusher(ctx, l, ctr, pushSecret, outputDir)

		h := hooks.NewListener(l, redis, gh, builder, pusher)
		h.BuildListener(ctx)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listenerCmd)
}

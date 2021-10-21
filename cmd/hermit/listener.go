package main

import (
	"os"

	"github.com/containerd/containerd/namespaces"
	"github.com/docker/docker/client"
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
		ctr, err := newContainerd()
		if err != nil {
			return err
		}
		ctx := namespaces.WithNamespace(cmd.Context(), "hermit")
		docker, err := client.NewEnvClient()
		if err != nil {
			return err
		}
		pusher, err := build.NewPusher(ctx, l, docker, pushSecret, outputDir)
		if err != nil {
			return err
		}
		scanner := build.NewScanner(l, ctr, outputDir)
		snapshotPusher := hooks.NewSnapshotPusher(l, gh)

		h := hooks.NewListener(l, redis, gh, builder, scanner, pusher, snapshotPusher)
		h.BuildListener(ctx)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listenerCmd)
}

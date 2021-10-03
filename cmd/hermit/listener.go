package main

import (
	"github.com/spf13/cobra"
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

		l := log.New()
		builder, err := newBuilder(cmd, l)
		if err != nil {
			return err
		}

		h := hooks.NewListener(l, redis, gh, builder)
		h.BuildListener(cmd.Context())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listenerCmd)
}

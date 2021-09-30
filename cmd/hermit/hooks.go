package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/hooks"
	"github.com/thepwagner/hermit/log"
)

// hooksCmd is a server that listens for GitHub webhooks and pushes interesting events to Redis.
var hooksCmd = &cobra.Command{
	Use:    "hooks",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		token := []byte(os.Getenv("GITHUB_WEBHOOK_SECRET"))
		redis, err := redisClient(cmd)
		if err != nil {
			return err
		}

		// Create and start server:
		log := log.New()
		_, gh, err := gitHubClient(cmd)
		if err != nil {
			return err
		}
		hooks := hooks.NewServer(log, redis, gh, token)
		srv := &http.Server{
			Addr:    "127.0.0.1:8080",
			Handler: hooks,
		}

		log.Info("starting server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(hooksCmd)
}

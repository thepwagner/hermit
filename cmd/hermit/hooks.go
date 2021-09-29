package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/log"
)

// hooksCmd is a server that listens for GitHub webhooks and pushes interesting events to Redis.
var hooksCmd = &cobra.Command{
	Use:    "hooks",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		redisAddr, err := cmd.Flags().GetString(redisUrlFlag)
		if err != nil {
			return err
		}
		token := []byte(os.Getenv("GITHUB_WEBHOOK_SECRET"))

		// Start server
		log := log.New()
		queue := redis.NewClient(&redis.Options{Addr: redisAddr})
		srv := &http.Server{
			Addr: "127.0.0.1:8080",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payload, err := github.ValidatePayload(r, token)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				switch hook := github.WebHookType(r); hook {
				case "pull_request", "push":
					log.Info("enqueuing event", "event", hook, len(payload))
					queue.RPush(r.Context(), fmt.Sprintf("github-hook-%s", hook), payload)
				default:
					log.Info("ignored event", "event", hook)
				}
			}),
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

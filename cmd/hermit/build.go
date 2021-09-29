package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v39/github"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/hooks"
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
		indexFile, err := flags.GetString(proxyIndex)
		if err != nil {
			return err
		}
		l.Info("building", "owner", owner, "repo", repo, "ref", ref)

		const (
			appID          = 141544
			installationID = 19814209
		)

		ghTransport, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, "build-hermit.2021-09-29.private-key.pem")
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		ghToken, err := ghTransport.Token(ctx)
		if err != nil {
			return err
		}

		gh := github.NewClient(&http.Client{Transport: ghTransport})
		cloner := build.NewGitCloner(l, gh, ghToken, srcDir)
		fc := build.NewFirecracker(l)
		builder, err := build.NewBuilder(l, cloner, fc, outputDir)
		if err != nil {
			return err
		}

		// FIXME: test hookshandler
		var e github.PushEvent
		fixture, err := ioutil.ReadFile("hooks/testdata/push.json")
		if err != nil {
			return err
		}
		if err := json.Unmarshal(fixture, &e); err != nil {
			return err
		}
		h := hooks.NewHandler(l, gh, builder)
		if err := h.OnPush(ctx, &e); err != nil {
			return err
		}
		return nil

		return builder.Build(ctx, &build.BuildParams{
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
	flags.StringP(proxyIndex, "f", "", "index to load")
	rootCmd.AddCommand(buildCmd)
}

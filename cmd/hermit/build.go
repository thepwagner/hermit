package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-github/v39/github"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
	"golang.org/x/oauth2"
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
		indexFile, err := flags.GetString(fileIndex)
		if err != nil {
			return err
		}
		l.Info("building", "owner", owner, "repo", repo, "ref", ref)
		ghToken := os.Getenv("GITHUB_TOKEN")

		ctx := cmd.Context()
		ghTokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: ghToken})
		gh := github.NewClient(oauth2.NewClient(ctx, ghTokenSource))
		cloner := build.NewGitCloner(l, gh, ghToken, srcDir)

		src, err := cloner.Clone(ctx, owner, repo, ref)
		if err != nil {
			return err
		}
		l.Info("source volume created", "src", src)

		if err := os.MkdirAll(outputDir, 0750); err != nil {
			return err
		}
		outputTmp, err := build.TempFile(outputDir, fmt.Sprintf("%s-*", ref))
		if err != nil {
			return err
		}
		if err := build.CreateVolume(ctx, outputTmp, 256); err != nil {
			_ = os.Remove(outputTmp)
			return err
		}

		fc, err := build.NewFirecracker(l)
		if err != nil {
			_ = os.Remove(outputTmp)
			return err
		}
		if err := fc.BootVM(ctx, src, outputTmp, indexFile); err != nil {
			_ = os.Remove(outputTmp)
			return err
		}
		return os.Rename(outputTmp, filepath.Join(outputDir, fmt.Sprintf("%s.img", ref)))
	},
}

func init() {
	flags := buildCmd.Flags()
	flags.String(repoOwner, "thepwagner-org", "GitHub repository owner")
	flags.StringP(repoName, "r", "debian-bullseye", "GitHub repository name")
	flags.String(repoRef, "12eef3d14eaf08b1753b352feb3d552013171064", "GitHub repository ref")
	flags.StringP(fileIndex, "f", "", "index to load")
	rootCmd.AddCommand(buildCmd)
}

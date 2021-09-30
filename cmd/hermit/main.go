package main

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
)

const (
	redisUrlFlag                = "redis"
	gitHubAppIDFlag             = "github-app-id"
	githubInstallationIDFlag    = "github-installation-id"
	githubAppPrivateKeyFileFlag = "github-private-key-file"
)

var rootCmd = &cobra.Command{
	Use: "hermit",
}

func init() {
	pflags := rootCmd.PersistentFlags()

	pflags.String(redisUrlFlag, "localhost:6379", "redis url")
	pflags.Int64(gitHubAppIDFlag, 141544, "GitHub app ID")
	pflags.Int64(githubInstallationIDFlag, 19814209, "GitHub installation ID")
	pflags.String(githubAppPrivateKeyFileFlag, "build-hermit.2021-09-29.private-key.pem", "GitHub app private key file")
}

func redisClient(cmd *cobra.Command) (*redis.Client, error) {
	redisAddr, err := cmd.Flags().GetString(redisUrlFlag)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(&redis.Options{Addr: redisAddr}), nil
}

func gitHubClient(cmd *cobra.Command) (*ghinstallation.Transport, *github.Client, error) {
	flags := cmd.Flags()
	appID, err := flags.GetInt64(gitHubAppIDFlag)
	if err != nil {
		return nil, nil, err
	}
	installationID, err := flags.GetInt64(githubInstallationIDFlag)
	if err != nil {
		return nil, nil, err
	}
	pkeyFile, err := flags.GetString(githubAppPrivateKeyFileFlag)
	if err != nil {
		return nil, nil, err
	}

	ghTransport, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, pkeyFile)
	if err != nil {
		return nil, nil, err
	}
	return ghTransport, github.NewClient(&http.Client{Transport: ghTransport}), nil
}

func newBuilder(cmd *cobra.Command, l logr.Logger) (*build.Builder, error) {
	ghTransport, gh, err := gitHubClient(cmd)
	if err != nil {
		return nil, err
	}
	ctx := cmd.Context()
	ghToken, err := ghTransport.Token(ctx)
	if err != nil {
		return nil, err
	}

	cloner := build.NewGitCloner(l, gh, ghToken, srcDir)
	fc := build.NewFirecracker(l)
	return build.NewBuilder(l, cloner, fc, outputDir)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

package main

import (
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	"github.com/google/go-github/v39/github"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
)

const (
	redisUrlFlag                = "redis-url"
	redisPasswordFlag           = "redis-password"
	githubAppIDFlag             = "github-app-id"
	githubInstallationIDFlag    = "github-installation-id"
	githubAppPrivateKeyFileFlag = "github-private-key-file"
	githubBotIDFlag             = "github-bot-id"
)

var rootCmd = &cobra.Command{
	Use: "hermit",
}

func init() {
	pflags := rootCmd.PersistentFlags()

	pflags.String(redisUrlFlag, "localhost:6379", "redis url")
	pflags.Int64(githubAppIDFlag, 141544, "GitHub app ID")
	pflags.Int64(githubInstallationIDFlag, 19814209, "GitHub installation ID")
	pflags.Int64(githubBotIDFlag, 91640726, "GitHub bot ID")
	pflags.String(githubAppPrivateKeyFileFlag, "build-hermit.2021-09-29.private-key.pem", "GitHub app private key file")
}

func redisClient(cmd *cobra.Command) (*redis.Client, error) {
	redisAddr, err := cmd.Flags().GetString(redisUrlFlag)
	if err != nil {
		return nil, err
	}
	opts := &redis.Options{Addr: redisAddr}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		opts.Password = redisPassword
	}
	return redis.NewClient(opts), nil
}

func gitHubClient(cmd *cobra.Command) (build.TokenSource, *github.Client, error) {
	flags := cmd.Flags()
	appID, err := flags.GetInt64(githubAppIDFlag)
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
	return ghTransport.Token, github.NewClient(&http.Client{Transport: ghTransport}), nil
}

func newBuilder(cmd *cobra.Command, l logr.Logger) (*build.Builder, error) {
	tokens, gh, err := gitHubClient(cmd)
	if err != nil {
		return nil, err
	}

	cloner := build.NewGitCloner(l, gh, tokens, srcDir)
	fc := build.NewFirecracker(l)
	return build.NewBuilder(l, cloner, fc, outputDir)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

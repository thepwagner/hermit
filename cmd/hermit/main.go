package main

import "github.com/spf13/cobra"

const redisUrlFlag = "redis"

var rootCmd = &cobra.Command{
	Use: "hermit",
}

func main() {
	rootCmd.PersistentFlags().String(redisUrlFlag, "localhost:6379", "redis url")

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

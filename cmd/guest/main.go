package main

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use: "guest",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

package main

import "github.com/spf13/cobra"

// guestCmd is the root command inside the sandbox guest.
var guestCmd = &cobra.Command{
	Use:   "guest",
	Short: "Hermit Guest",
	Long:  "Hermit Guest runs inside the sandbox.",
}

func main() {
	if err := guestCmd.Execute(); err != nil {
		panic(err)
	}
}

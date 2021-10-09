package main

import (
	"fmt"

	"github.com/containerd/containerd/namespaces"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
)

var scanCmd = &cobra.Command{
	Use: "scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := log.New()
		ctr, err := newContainerd()
		if err != nil {
			return err
		}
		defer ctr.Close()

		scanner := build.NewScanner(l, ctr, outputDir)
		bp := &build.Params{
			Owner: "thepwagner-org",
			Repo:  "renovate",
			Ref:   "2e5e74d4817fac6a1c22865d9396aacd65c7918c",
		}
		ctx := namespaces.WithNamespace(cmd.Context(), "hermit")
		report, err := scanner.ScanBuildOutput(ctx, bp)
		if err != nil {
			return err
		}
		reportMD, err := build.RenderReport(report)
		if err != nil {
			return err
		}

		fmt.Println(reportMD)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

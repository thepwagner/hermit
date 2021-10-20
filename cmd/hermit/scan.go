package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aquasecurity/trivy/pkg/report"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
	"github.com/thepwagner/hermit/scan"
)

const scanRootFlag = "root"

var scanCmd = &cobra.Command{
	Use: "scan",
	RunE: func(cmd *cobra.Command, args []string) error {
		scanRoot, err := cmd.Flags().GetString(scanRootFlag)
		if err != nil {
			return err
		}

		l := log.New()
		ctr, err := newContainerd()
		if err != nil {
			return err
		}
		defer ctr.Close()
		scanner := build.NewScanner(l, ctr, "")

		kustomizations, err := scan.WalkKustomizations(scanRoot)
		if err != nil {
			return err
		}
		imageUsage := map[string][]string{}
		for _, k := range kustomizations {
			for _, i := range k.Images {
				img := i.Image()
				imageUsage[img] = append(imageUsage[img], k.Path)
			}
		}
		l.Info("crawled kustomizations", "kustomization_count", len(kustomizations), "image_count", len(imageUsage))

		ctx := namespaces.WithNamespace(cmd.Context(), "hermit")
		for img, usages := range imageUsage {
			imageLog := l.WithValues("image", img)
			imageLog.Info("scanning image", "usage_count", len(usages))
			report, err := scanImage(ctx, imageLog, ctr, scanner, img)
			if err != nil {
				return err
			}

			// FIXME: early exit
			reportMD, err := build.RenderReport(report)
			if err != nil {
				return err
			}
			fmt.Println(reportMD)
			return nil
		}

		return nil
	},
}

func scanImage(ctx context.Context, log logr.Logger, ctr *containerd.Client, scanner *build.Scanner, img string) (*report.Report, error) {
	imgHash := sha256.Sum256([]byte(img))
	imageTar := filepath.Join(outputDir, "scan-images", fmt.Sprintf("%x", imgHash), "image.tar")

	log.Info("checking for existing image", "image_tar", imageTar)
	if _, err := os.Stat(imageTar); errors.Is(err, os.ErrNotExist) {
		lanImage := scan.LanImage(img)
		log.Info("pulling image...", "lan_image", lanImage)
		if ctrImage, err := ctr.Pull(ctx, lanImage); err != nil {
			return nil, fmt.Errorf("pulling %q: %w", img, err)
		} else {
			imageSize, err := ctrImage.Size(ctx)
			if err != nil {
				return nil, fmt.Errorf("getting image size: %w", err)
			}
			log.Info("pulled image", "image_size", imageSize)

			defer func() {
				if err := ctr.ImageService().Delete(ctx, ctrImage.Name()); err != nil {
					log.Error(err, "deleting image")
				} else {
					log.Info("deleted image")
				}
			}()

		}
		if err := os.MkdirAll(filepath.Dir(imageTar), 0750); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(imageTar, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		if err := ctr.Export(ctx, f, archive.WithPlatform(platforms.OnlyStrict(platforms.MustParse("linux/amd64"))), archive.WithImage(ctr.ImageService(), lanImage)); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("exporting %q: %w", img, err)
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	report, err := scanner.Scan(ctx, imageTar)
	if err != nil {
		return nil, err
	}
	report.Metadata.ImageID = img
	return report, nil
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().String(scanRootFlag, "", "")
}

package main

import (
	"context"
	"os"

	"github.com/thepwagner/hermit/build"
	"github.com/thepwagner/hermit/log"
)

func run() error {
	ctx := context.Background()
	b, err := build.NewBuilder(ctx)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return b.Build(ctx, wd)
}

func main() {
	l := log.New()
	if err := run(); err != nil {
		l.Error(err, "error running build")
	}
}

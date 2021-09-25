package main

import (
	"github.com/thepwagner/hermit/log"
)

func run() error {

}

func main() {
	l := log.New()
	if err := run(); err != nil {
		l.Error(err, "error running build")
	}
}

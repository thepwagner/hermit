package main

import (
	"io"
	"net"

	"github.com/go-logr/logr"
	"github.com/mdlayher/vsock"
	"github.com/thepwagner/hermit/log"
)

func run(log logr.Logger) error {
	l, err := net.Listen("tcp", "localhost:3128")
	if err != nil {
		return err
	}
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		log.Info("accepted connection", "addr", c.RemoteAddr())

		s, err := vsock.Dial(2, 1024)
		if err != nil {
			return err
		}
		defer s.Close()
		log.Info("made connection", "remote_addr", c.RemoteAddr(), "local_addr", c.LocalAddr())
		go io.Copy(c, s)
		go io.Copy(s, c)
	}
	return nil
}

func main() {
	l := log.New()
	if err := run(l); err != nil {
		l.Error(err, "error")
	}
}

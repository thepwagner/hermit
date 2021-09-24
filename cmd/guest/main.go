package main

import (
	"io"
	"net"

	"github.com/go-logr/logr"
	"github.com/mdlayher/vsock"
	"github.com/thepwagner/hermit/log"
)

const (
	bindAddress    = "127.0.0.1:3128"
	vsockHost      = 2
	vsockProxyPort = 1024
)

func run(log logr.Logger) error {
	l, err := net.Listen("tcp4", bindAddress)
	if err != nil {
		return err
	}
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		log.Info("accepted tcp connection", "addr", c.RemoteAddr().String())
		go handle(log, c)
	}
	return nil
}

func handle(log logr.Logger, c net.Conn) {
	s, err := vsock.Dial(vsockHost, vsockProxyPort)
	if err != nil {
		log.Error(err, "opening vsock")
		return
	}
	defer s.Close()

	localAddr := s.LocalAddr().String()
	log.Info("made connection", "remote_addr", s.RemoteAddr().String(), "local_addr", localAddr)
	go io.Copy(c, s)
	io.Copy(s, c)
	log.Info("closed connection", "remote_addr", s.RemoteAddr().String(), "local_addr", localAddr)
}

func main() {
	l := log.New()
	if err := run(l); err != nil {
		l.Error(err, "error")
	}
}

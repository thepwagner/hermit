package main

import (
	"io"
	"net"

	"github.com/go-logr/logr"
	"github.com/mdlayher/vsock"
	"github.com/spf13/cobra"
	"github.com/thepwagner/hermit/log"
)

const (
	vsockBindAddress = "127.0.0.1:3128"
	vsockCIDHost     = 2
	vsockProxyPort   = 1024
)

// proxyCmd forwards TCP requests to the host proxy via VSOCK.
var proxyCmd = &cobra.Command{
	Use: "proxy",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := log.New()
		listener, err := net.Listen("tcp4", vsockBindAddress)
		if err != nil {
			return err
		}
		defer listener.Close()

		for {
			c, err := listener.Accept()
			if err != nil {
				return err
			}
			l.Info("accepted tcp connection", "addr", c.RemoteAddr().String())
			go handle(l, c)
		}
	},
}

func handle(log logr.Logger, c net.Conn) {
	s, err := vsock.Dial(vsockCIDHost, vsockProxyPort)
	if err != nil {
		log.Error(err, "opening vsock")
		return
	}
	defer s.Close()

	localAddr := s.LocalAddr().String()
	remoteAddr := s.RemoteAddr().String()
	log.Info("made connection", "remote_addr", remoteAddr, "local_addr", localAddr)
	go func() {
		if _, err := io.Copy(c, s); err != nil {
			log.Error(err, "copying data")
		}
	}()
	if _, err := io.Copy(s, c); err != nil {
		log.Error(err, "copying data")
	}
	log.Info("closed connection", "remote_addr", remoteAddr, "local_addr", localAddr)
}

func init() {
	guestCmd.AddCommand(proxyCmd)
}

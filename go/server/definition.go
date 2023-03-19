package server

import (
	"context"
	"net"
	"net/http"
)

type (
	// HTTP represents interface for HTTP/s server
	HTTP interface {
		Shutdown(ctx context.Context) error
		ListenAndServer() error
		ListenAndServerTLS(certFile, keyFile string) error
	}

	// GRPC represents interface for grpc server
	GRPC interface {
		GracefulStop()
		Stop()
		Serve(l net.Listener) error
	}

	Config struct {
		Port          int    `yaml:"port"`
		ReadTimeout   int    `yaml:"read_timeout"`
		WriteTimeout  int    `yaml:"write_timeout"`
		MaxHeaderByte int    `yaml:"max_header_byte"`
		Environment   string `yaml:"environment"`
		Handler       http.Handler
		CertFile      string
		KeyFile       string
	}
)

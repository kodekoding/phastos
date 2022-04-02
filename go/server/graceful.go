package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func ServeHTTP(config *Config) error {
	return serveHTTPs(config, false)
}

func ServeHTTPS(config *Config) error {
	return serveHTTPs(config, true)
}

func serveHTTPs(config *Config, secure bool) error {
	listenPort := fmt.Sprintf(":%d", config.Port)
	server := &http.Server{
		Addr:           listenPort,
		ReadTimeout:    time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(config.WriteTimeout) * time.Second,
		Handler:        config.Handler,
		MaxHeaderBytes: config.MaxHeaderByte,
	}

	listener, err := net.Listen("tcp4", listenPort)
	if err != nil {
		return err
	}

	shutdownSignal := WaitTermSig(func(ctx context.Context) error {
		stopped := make(chan struct{})
		ctx, cancel := context.WithTimeout(ctx, time.Duration(10)*time.Second)
		defer cancel()
		go func() {
			_ = server.Shutdown(ctx)
			close(stopped)
		}()

		select {
		case <-ctx.Done():
			return errors.New("server shutdown timed out")
		case <-stopped:

		}

		return nil
	})

	go func() {
		if secure {
			if err := server.ServeTLS(listener, config.CertFile, config.KeyFile); err != nil {
				log.Fatalf("Cannot Listen and Serve: %s", err.Error())
			}
		} else {
			if err := server.Serve(listener); err != nil {
				log.Fatalf("Cannot Listen and Serve: %s", err.Error())
			}
		}
	}()

	protocol := "HTTP"
	if secure {
		protocol += "s"
	}
	log.Printf("%s Server is running on %s", protocol, listenPort)

	<-shutdownSignal

	log.Printf("%s Server stopped", protocol)
	return nil
}

func WaitTermSig(handler func(context.Context) error) <-chan struct{} {
	stoppedCh := make(chan struct{})
	go func() {
		signals := make(chan os.Signal, 1)

		// wait for the sigterm
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-signals

		// We received an os signal, shut down.
		if err := handler(context.Background()); err != nil {
			log.Printf("graceful shutdown  failed: %v", err)
		} else {
			log.Println("gracefull shutdown succeed")
		}

		close(stoppedCh)

	}()
	return stoppedCh
}

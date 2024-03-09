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

	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/kodekoding/phastos/v2/go/helper"
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

	sign := WaitTermSig(func(ctx context.Context) error {

		<-ctx.Done()

		stopped := make(chan struct{})
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
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
			if err = server.ServeTLS(listener, config.CertFile, config.KeyFile); err != http.ErrServerClosed {
				log.Fatalf("Cannot serve HTTPS: %s", err.Error())
			}
		} else {
			if err = server.Serve(listener); err != http.ErrServerClosed {
				log.Fatalf("Cannot serve HTTP: %s", err.Error())
			}
		}
	}()

	protocol := "HTTP"
	if secure {
		protocol += "s"
	}

	appName := os.Getenv("APP_NAME")

	log.Printf("%s %s Server %s is running on %s", appName, protocol, os.Getenv("APPS_ENV"), listenPort)

	go func() {
		if !(env.IsLocal() || env.IsDevelopment()) {
			_ = helper.SendSlackNotification(
				config.Ctx,
				helper.NotifTitle(fmt.Sprintf("%s Service is started", appName)),
			)

		}
	}()
	<-sign

	log.Printf("%s Server stopped", protocol)
	go func() {
		if !(env.IsLocal() || env.IsDevelopment()) {
			_ = helper.SendSlackNotification(
				config.Ctx,
				helper.NotifTitle(fmt.Sprintf("%s Service is Stopped", appName)),
				helper.NotifMsgType(helper.NotifWarnType),
			)
		}
	}()
	return nil
}

func WaitTermSig(handler func(context.Context) error) <-chan struct{} {
	stopedChannel := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		c := make(chan os.Signal, 1)

		// wait for the sigterm
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-c

		cancel()
		// We received an os signal, shut down.
		if err := handler(ctx); err != nil {
			log.Printf("graceful shutdown  failed: %v", err)
		} else {
			log.Println("gracefull shutdown succeed")
		}

		close(stopedChannel)
	}()
	return stopedChannel
}

package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/server"
)

func serveHTTPs(config *server.Config, secure bool) error {
	log := plog.Get()
	listenPort := fmt.Sprintf(":%d", config.Port)
	serverConfig := &http.Server{
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

	sign := WaitTermSig(config.Ctx, func(ctx context.Context) error {

		<-ctx.Done()

		stopped := make(chan struct{})
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
		defer cancel()
		go func() {
			_ = serverConfig.Shutdown(ctx)
			close(stopped)
		}()

		select {
		case <-ctx.Done():
			return errors.New("serverConfig shutdown timed out")
		case <-stopped:

		}

		return nil
	})

	go func() {
		if secure {
			if err = serverConfig.ServeTLS(listener, config.CertFile, config.KeyFile); !errors.Is(http.ErrServerClosed, err) {
				log.Fatal().Err(err).Msg("Failed to Server HTTPS")
			}
		} else {
			if err = serverConfig.Serve(listener); !errors.Is(http.ErrServerClosed, err) {
				log.Fatal().Err(err).Msg("Failed to Server HTTP")
			}
		}
	}()

	protocol := "HTTP"
	if secure {
		protocol += "s"
	}

	appName := os.Getenv("APP_NAME")
	environment := os.Getenv("APPS_ENV")

	log.Info().Str("protocol", protocol).Msg("Server is running")

	go func() {
		isNotifyServiceStatus, err := strconv.ParseBool(os.Getenv("NOTIFY_SERVICE_STATUS"))
		if err != nil {
			isNotifyServiceStatus = false
		}
		if isNotifyServiceStatus {
			_ = helper.SendSlackNotification(
				config.Ctx,
				helper.NotifTitle(fmt.Sprintf("[%s] %s Service is started", environment, appName)),
				helper.NotifData(map[string]string{
					"version":        config.Version,
					"container_name": os.Getenv("CONTAINER_NAME"),
				}),
			)

		}
	}()
	<-sign

	log.Info().Msg("Server stopped")

	return nil
}

func WaitTermSig(ctx context.Context, handler func(context.Context) error) <-chan struct{} {
	log := plog.Ctx(ctx)
	stopedChannel := make(chan struct{})
	newCtx, cancel := context.WithCancel(ctx)

	go func() {
		c := make(chan os.Signal, 1)

		// wait for the sigterm
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
		<-c

		cancel()
		// We received an os signal, shut down.
		if err := handler(newCtx); err != nil {
			log.Err(err).Msg("Graceful shutdown  failed")
		} else {
			log.Info().Msg("Graceful shutdown succeed")
			isNotifyServiceStatus, err := strconv.ParseBool(os.Getenv("NOTIFY_SERVICE_STATUS"))
			if err != nil {
				isNotifyServiceStatus = false
			}
			appName := os.Getenv("APP_NAME")
			environment := os.Getenv("APPS_ENV")
			if isNotifyServiceStatus {
				_ = helper.SendSlackNotification(
					ctx,
					helper.NotifTitle(fmt.Sprintf("[%s] %s Service is Stopped", environment, appName)),
					helper.NotifMsgType(helper.NotifWarnType),
					helper.NotifData(map[string]string{
						"container_name": os.Getenv("CONTAINER_NAME"),
					}),
				)
			}
		}

		close(stopedChannel)
	}()
	return stopedChannel
}

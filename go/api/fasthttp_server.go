package api

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/server"
)

// serveFastHTTPs mirrors serveHTTPs but uses fasthttp under the hood.
func serveFastHTTPs(config *server.Config, secure bool) error {
	log := plog.Get()
	listenPort := fmt.Sprintf(":%d", config.Port)

	// Convert the existing http.Handler (which is built in InitHandler) into a fasthttp handler.
	fastHandler := fasthttpadaptor.NewFastHTTPHandler(config.Handler)

	fastSrv := &fasthttp.Server{
		Handler:            fastHandler,
		ReadTimeout:        time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout:       time.Duration(config.WriteTimeout) * time.Second,
		MaxRequestBodySize: config.MaxHeaderByte,
	}

	ln, err := net.Listen("tcp4", listenPort)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	// graceful‑shutdown helper – re‑use the existing WaitTermSig logic.
	stop := WaitTermSig(config.Ctx, func(ctx context.Context) error {
		<-ctx.Done()
		_ = ln.Close()
		return nil
	})

	go func() {
		if secure {
			if err = fastSrv.ServeTLS(ln, config.CertFile, config.KeyFile); err != nil {
				log.Fatal().Err(err).Msg("Failed to serve HTTPS (fasthttp)")
			}
		} else {
			if err = fastSrv.Serve(ln); err != nil {
				log.Fatal().Err(err).Msg("Failed to serve HTTP (fasthttp)")
			}
		}
	}()

	protocol := "HTTP"
	if secure {
		protocol = "HTTPS"
	}

	appName := os.Getenv("APP_NAME")
	environment := os.Getenv("APPS_ENV")

	log.Info().Str("protocol", protocol).Msg("Fasthttp server is running")

	go func() {
		if isNotify, _ := strconv.ParseBool(os.Getenv("NOTIFY_SERVICE_STATUS")); isNotify {
			_ = helper.SendSlackNotification(
				config.Ctx,
				helper.NotifTitle(fmt.Sprintf("[%s] %s Service is started", environment, appName)),
				helper.NotifData(map[string]string{
					"version":        config.Version,
					containerNameKey: os.Getenv(containerNameEnv),
				}),
			)
		}
	}()

	<-stop // block until termination signal

	log.Info().Msg("Fasthttp server stopped")
	return nil
}

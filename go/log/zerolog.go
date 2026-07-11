package log

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	logWriter "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"github.com/kodekoding/phastos/v2/go/env"
)

var logZero zerolog.Logger
var once sync.Once

type (
	Logger struct {
		newRelicApp     *newrelic.Application
		appPort         int
		appVersion      string
		otelTCPEndpoint string
		otelLogWriter   io.Writer
	}

	LoggerOption func(*Logger)
)

func WithNewRelicApp(app *newrelic.Application) LoggerOption {
	return func(l *Logger) {
		l.newRelicApp = app
	}
}

func WithAppVersion(appVersion string) LoggerOption {
	return func(l *Logger) {
		l.appVersion = appVersion
	}
}

func WithAppPort(appPort int) LoggerOption {
	return func(l *Logger) {
		l.appPort = appPort
	}
}

func WithOTelLogEndpoint() LoggerOption {
	return func(l *Logger) {
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			return
		}
		host := strings.TrimPrefix(endpoint, "http://")
		host = strings.TrimPrefix(host, "https://")
		host = strings.Replace(host, ":4318", ":54526", 1)
		if host == endpoint {
			return
		}
		l.otelTCPEndpoint = host
		l.otelLogWriter = newOTelTCPWriter(host)
	}
}

func Get(loggerOption ...LoggerOption) zerolog.Logger {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		logger := new(Logger)
		for _, opt := range loggerOption {
			opt(logger)
		}

		var logLevel zerolog.Level
		if env.ServiceEnv() == env.ProductionEnv {
			logLevel = zerolog.InfoLevel
		} else {
			logLevel = zerolog.DebugLevel
		}

		var writers []io.Writer
		if logger.newRelicApp != nil {
			writers = append(writers, logWriter.New(os.Stdout, logger.newRelicApp))
		} else if logger.otelLogWriter != nil {
			writers = append(writers, os.Stdout)
		} else {
			writers = append(writers, zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
			})
		}

		if logger.otelLogWriter != nil {
			writers = append(writers, logger.otelLogWriter)
		}

		var writer io.Writer
		if len(writers) == 1 {
			writer = writers[0]
		} else {
			writer = io.MultiWriter(writers...)
		}

		logInit := zerolog.New(writer).
			Level(logLevel).
			With().
			Timestamp().
			Str("app", os.Getenv("APP_NAME")).
			Str("env", env.ServiceEnv()).
			Int("port", logger.appPort)

		if logger.appVersion != "" {
			logInit = logInit.Str("app_version", logger.appVersion)
		}

		containerName := os.Getenv("CONTAINER_NAME")
		if containerName != "" {
			logInit = logInit.Str("container_name", containerName)
		}

		logZero = logInit.Logger()
		zerolog.DefaultContextLogger = &logZero
		logZero.Info().Msgf("Logger succesfully initialized")
	})

	return logZero
}

func Ctx(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}

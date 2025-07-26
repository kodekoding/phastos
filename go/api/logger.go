package api

import (
	"github.com/newrelic/go-agent/v3/newrelic"
	"io"
	"os"
	"time"

	"github.com/kodekoding/phastos/v2/go/env"
	logWriter "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var logZero zerolog.Logger

type (
	Logger struct {
		newRelicApp *newrelic.Application
		appPort     int
	}

	LoggerOption func(*Logger)
)

func LoggerWithNewRelicApp(app *newrelic.Application) LoggerOption {
	return func(l *Logger) {
		l.newRelicApp = app
	}
}

func LoggerWithAppPort(appPort int) LoggerOption {
	return func(l *Logger) {
		l.appPort = appPort
	}
}

func GetLogger(loggerOption ...LoggerOption) zerolog.Logger {
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

		var writer io.Writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}

		if logger.newRelicApp != nil {
			writer = logWriter.New(os.Stdout, logger.newRelicApp)
		}

		logInit := zerolog.New(writer).
			Level(logLevel).
			With().
			Timestamp().
			Str("app", os.Getenv("APP_NAME")).
			Str("env", env.ServiceEnv()).
			Int("port", logger.appPort)

		if appVersion != "" {
			logInit = logInit.Str("app_version", appVersion)
		}

		containerName := os.Getenv("CONTAINER_NAME")
		if containerName != "" {
			logInit = logInit.Str("container_name", containerName)
		}

		logZero = logInit.Logger()
		logZero.Info().Msgf("Logger succesfully initialized")
	})

	return logZero
}

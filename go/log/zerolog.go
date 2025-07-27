package log

import (
	"io"
	"os"
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
		newRelicApp *newrelic.Application
		appPort     int
		appVersion  string
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

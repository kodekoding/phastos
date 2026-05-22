package context

import (
	"context"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/kodekoding/phastos/v2/go/env"
	logWriter "github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var logger *zerolog.Logger
var once sync.Once

func NewLog(newRelicApp *newrelic.Application) {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		var gitRevision string

		buildInfo, ok := debug.ReadBuildInfo()
		if ok {
			for _, v := range buildInfo.Settings {
				if v.Key == "vcs.revision" {
					gitRevision = v.Value
					break
				}
			}
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

		if newRelicApp != nil {
			writer = logWriter.New(os.Stdout, newRelicApp)
		}

		nLogger := zerolog.New(writer).
			Level(logLevel).
			With().
			Timestamp().
			Str("app", os.Getenv("APP_NAME")).
			Str("git_revision", gitRevision).
			Str("go_version", buildInfo.GoVersion).
			Logger()

		logger = &nLogger

	})
}

func Log(ctx ...context.Context) *zerolog.Logger {
	if logger == nil {
		NewLog(nil)
	}
	if len(ctx) > 0 {
		paramCtx := ctx[0]
		logger = zerolog.Ctx(paramCtx)
	}
	return logger
}

func Ctx(ctx ...context.Context) *zerolog.Logger {
	if logger == nil {
		NewLog(nil)
	}
	if len(ctx) > 0 {
		paramCtx := ctx[0]
		logger = zerolog.Ctx(paramCtx)
	}
	return logger
}

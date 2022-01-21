package log

import (
	"context"

	"github.com/kodekoding/phastos/go/log/logger"
	xid "github.com/rs/xid"
)

// RFC3339Milli We agree that we want a millisecond precisstion, sadly in golang it's not there. https://github.com/golang/go/issues/13291
const (
	RFC3339Milli = "2006-01-02T15:04:05.999Z07:00"
)

// SetStdLog will be the entry point for tdk logging library to make sure all Tokopedia services have the same log structures.
// The general specification will be JSON, request id, context id, metadata, error and message.
// For more specifications please go to https://tokopedia.atlassian.net/wiki/spaces/EN/pages/694817819/Logging+Format+Standardization
func SetStdLog(config *Config) error {
	var (
		newLogger         logger.Logger
		newDebugLogger    logger.Logger
		err               error
		loggerConfig      = logger.Config{Level: logger.InfoLevel}
		debugLoggerConfig = loggerConfig
	)

	if config != nil {
		loggerConfig = logger.Config{
			Level:      logger.StringToLevel(config.Level),
			LogFile:    config.LogFile, // Default will goes to os.Stderr
			TimeFormat: RFC3339Milli,
			Caller:     true,
			AppName:    config.AppName,
			UseJSON:    true,
			StdLog:     true,
			UseColor:   isDev,
			CallerSkip: config.CallerSkip,
		}
		debugLoggerConfig = logger.Config{
			Level:      logger.StringToLevel(config.Level),
			LogFile:    config.DebugFile, // Default will goes to os.Stderr
			TimeFormat: RFC3339Milli,
			Caller:     true,
			AppName:    config.AppName,
			UseJSON:    true,
			StdLog:     true,
			UseColor:   isDev,
			CallerSkip: config.CallerSkip,
			Debug:      true,
		}
	}

	newLogger, err = NewLogger(Zerolog, &loggerConfig)
	if err != nil {
		return err
	}

	newDebugLogger, err = NewLogger(Zerolog, &debugLoggerConfig)
	if err != nil {
		return err
	}

	// extra check because it is very difficult to debug if the log itself causes the panic
	if newLogger != nil && debugLogger != nil {
		traceLogger = newDebugLogger
		debugLogger = newDebugLogger
		infoLogger = newDebugLogger
		warnLogger = newLogger
		errorLogger = newLogger
		fatalLogger = newLogger
	}

	return nil

}

// StdTrace print trace level log with standardized parameters.
func StdTrace(ctx context.Context, metadata interface{}, err error, message string) {
	traceLogger.StdTrace(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, message)
}

// StdTracef print trace level log with standardized parameters with formatter.
func StdTracef(ctx context.Context, metadata interface{}, err error, format string, args ...interface{}) {
	traceLogger.StdTracef(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, format, args...)
}

// StdDebug print trace level log with standardized parameters.
func StdDebug(ctx context.Context, metadata interface{}, err error, message string) {
	debugLogger.StdDebug(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, message)
}

// StdDebugf print trace level log with standardized parameters with formatter.
func StdDebugf(ctx context.Context, metadata interface{}, err error, format string, args ...interface{}) {
	debugLogger.StdDebugf(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, format, args...)
}

// StdInfo print trace level log with standardized parameters.
func StdInfo(ctx context.Context, metadata interface{}, err error, message string) {
	infoLogger.StdInfo(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, message)
}

// StdInfof print trace level log with standardized parameters with formatter.
func StdInfof(ctx context.Context, metadata interface{}, err error, format string, args ...interface{}) {
	infoLogger.StdInfof(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, format, args...)
}

// StdWarn print trace level log with standardized parameters.
func StdWarn(ctx context.Context, metadata interface{}, err error, message string) {
	warnLogger.StdWarn(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, message)
}

// StdWarnf print trace level log with standardized parameters with formatter.
func StdWarnf(ctx context.Context, metadata interface{}, err error, format string, args ...interface{}) {
	warnLogger.StdWarnf(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, format, args...)
}

// StdError print trace level log with standardized parameters.
func StdError(ctx context.Context, metadata interface{}, err error, message string) {
	errorLogger.StdError(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, message)
}

// StdErrorf print trace level log with standardized parameters with formatter.
func StdErrorf(ctx context.Context, metadata interface{}, err error, format string, args ...interface{}) {
	errorLogger.StdErrorf(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, format, args...)
}

// StdFatal print trace level log with standardized parameters.
func StdFatal(ctx context.Context, metadata interface{}, err error, message string) {
	fatalLogger.StdFatal(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, message)
}

// StdFatalf print trace level log with standardized parameters with formatter.
func StdFatalf(ctx context.Context, metadata interface{}, err error, format string, args ...interface{}) {
	fatalLogger.StdFatalf(GetCtxRequestID(ctx), GetCtxID(ctx), err, metadata, format, args...)
}

type contextKey string

const (
	contextKeyRequestID = contextKey("tkpd-log-request-id")
	contextKeyID        = contextKey("tkpd-log-context-id")
)

// InitLogContext will initialize context with some informations
// Currently it will inject request id using xid
// Ideally InitLogContext will be wrap around middleware, either grpc or http
// For Request ID will currently set it with xid if empty. Can be explored with trace id
func InitLogContext(ctx context.Context) context.Context {
	// Check if request id is in context
	if GetCtxRequestID(ctx) == "" {
		return SetCtxRequestID(ctx, xid.New().String())
	}
	return ctx
}

// SetCtxRequestID set context with request id
func SetCtxRequestID(ctx context.Context, uuid string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, uuid)
}

// SetCtxID set context with context it
func SetCtxID(ctx context.Context, contextID string) context.Context {
	return context.WithValue(ctx, contextKeyID, contextID)
}

// GetCtxRequestID get request id from context
func GetCtxRequestID(ctx context.Context) string {
	id, ok := ctx.Value(contextKeyRequestID).(string)
	if !ok {
		return ""
	}
	return id
}

// GetCtxID get request id from context
func GetCtxID(ctx context.Context) string {
	id, ok := ctx.Value(contextKeyID).(string)
	if !ok {
		return ""
	}
	return id

}

package log

import (
	"github.com/kodekoding/phastos/go/log/logger"
)

// Engine of logger.
// Deprecated, there is only 1 engine now
type Engine = logger.Engine

// Logrus engine.
// Deprecated, logrus engine is dropped, use zerolog engine instead
const Logrus Engine = logger.Logrus

// Debugw prints debug level log with additional fields.
// Deprecated: use DebugWithFields
func Debugw(msg string, keyValues KV) {
	debugLogger.DebugWithFields(msg, keyValues)
}

// Infow prints info level log with additional fields.
// Deprecated: use InfoWithFields
func Infow(msg string, keyValues KV) {
	infoLogger.InfoWithFields(msg, keyValues)
}

// Warnw prints warn level log with additional fields.
// Deprecated: use WarnWithFields
func Warnw(msg string, keyValues KV) {
	warnLogger.WarnWithFields(msg, keyValues)
}

// Errorw prints error level log with additional fields.
// Deprecated: use ErrorWithFields
func Errorw(msg string, keyValues KV) {
	errorLogger.ErrorWithFields(msg, keyValues)
}

// Fatalw prints fatal level log with additional fields.
// Deprecated: use FatalWithFields
func Fatalw(msg string, keyValues KV) {
	fatalLogger.FatalWithFields(msg, keyValues)
}

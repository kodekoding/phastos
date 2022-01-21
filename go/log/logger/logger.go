package logger

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type (
	// KV is a type for logging with more information
	// this used by ...WithFields function
	KV = map[string]interface{}

	// Logger interface
	Logger interface {
		SetLevel(level Level)
		Debug(args ...interface{})
		Debugln(args ...interface{})
		Debugf(format string, args ...interface{})
		DebugWithFields(msg string, KV KV)
		Info(args ...interface{})
		Infoln(args ...interface{})
		Infof(format string, args ...interface{})
		InfoWithFields(msg string, KV KV)
		Warn(args ...interface{})
		Warnln(args ...interface{})
		Warnf(format string, args ...interface{})
		WarnWithFields(msg string, KV KV)
		Error(args ...interface{})
		Errorln(args ...interface{})
		Errorf(format string, args ...interface{})
		ErrorWithFields(msg string, KV KV)
		Errors(err error)
		Fatal(args ...interface{})
		Fatalln(args ...interface{})
		Fatalf(format string, args ...interface{})
		FatalWithFields(msg string, KV KV)
		IsValid() bool // IsValid check if Logger is created using constructor

		StdTrace(requestID string, contextID string, err error, metadata interface{}, message string)
		StdTracef(requestID string, contextID string, err error, metadata interface{}, format string, args ...interface{})
		StdDebug(requestID string, contextID string, err error, metadata interface{}, message string)
		StdDebugf(requestID string, contextID string, err error, metadata interface{}, format string, args ...interface{})
		StdInfo(requestID string, contextID string, err error, metadata interface{}, message string)
		StdInfof(requestID string, contextID string, err error, metadata interface{}, format string, args ...interface{})
		StdWarn(requestID string, contextID string, err error, metadata interface{}, message string)
		StdWarnf(requestID string, contextID string, err error, metadata interface{}, format string, args ...interface{})
		StdError(requestID string, contextID string, err error, metadata interface{}, message string)
		StdErrorf(requestID string, contextID string, err error, metadata interface{}, format string, args ...interface{})
		StdFatal(requestID string, contextID string, err error, metadata interface{}, message string)
		StdFatalf(requestID string, contextID string, err error, metadata interface{}, format string, args ...interface{})
	}

	// Level of log
	Level int

	// Engine of logger
	Engine string
)

// list of log level
const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// Log level
const (
	TraceLevelString = "trace"
	DebugLevelString = "debug"
	InfoLevelString  = "info"
	WarnLevelString  = "warn"
	ErrorLevelString = "error"
	FatalLevelString = "fatal"
)

// DefaultTimeFormat of logger
const DefaultTimeFormat = time.RFC3339

// Logger engine option
const (
	Logrus  Engine = "logrus"
	Zerolog Engine = "zerolog"
)

// StringToLevel to set string to level
func StringToLevel(level string) Level {
	switch strings.ToLower(level) {
	case TraceLevelString:
		return TraceLevel
	case DebugLevelString:
		return DebugLevel
	case InfoLevelString:
		return InfoLevel
	case WarnLevelString:
		return WarnLevel
	case ErrorLevelString:
		return ErrorLevel
	case FatalLevelString:
		return FatalLevel
	default:
		// TODO: make this more informative when happened
		return InfoLevel
	}
}

// LevelToString convert log level to readable string
func LevelToString(l Level) string {
	switch l {
	case TraceLevel:
		return TraceLevelString
	case DebugLevel:
		return DebugLevelString
	case InfoLevel:
		return InfoLevelString
	case WarnLevel:
		return WarnLevelString
	case ErrorLevel:
		return ErrorLevelString
	case FatalLevel:
		return FatalLevelString
	default:
		return InfoLevelString
	}
}

// Config of logger
type Config struct {
	Level      Level
	AppName    string
	LogFile    string
	TimeFormat string
	CallerSkip int
	Caller     bool
	UseColor   bool
	UseJSON    bool
	StdLog     bool
	Debug      bool
}

// OpenLogFile tries to open the log file (creates it if not exists) in write-only/append mode and return it
// Note: the func return nil for both *os.File and error if the file name is empty string
func (c *Config) OpenLogFile() (*os.File, error) {
	if c.LogFile == "" {
		return nil, nil
	}

	err := os.MkdirAll(filepath.Dir(c.LogFile), 0755)
	if err != nil && err != os.ErrExist {
		return nil, err
	}

	return os.OpenFile(c.LogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
}

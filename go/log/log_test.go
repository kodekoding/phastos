package log

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kodekoding/phastos/v2/go/log/logger"
	"github.com/kodekoding/phastos/v2/go/log/logger/zerolog"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
)

func TestZerolog(t *testing.T) {
	// reset
	err := SetConfig(nil)
	require.NoError(t, err)

	testLineNumber(t, logger.Zerolog)
	testSetLogger(t, logger.Zerolog, &zerolog.Logger{})
	testSetLevel(t, logger.Zerolog)
}

func testLineNumber(t *testing.T, engine logger.Engine) {
	fmt.Println("TestLineNumber")

	defer os.Remove("test.log")
	cfg := Config{
		Engine:     engine,
		AppName:    "tdk-app",
		Level:      "debug",
		LogFile:    "test.log",
		DebugFile:  "test.log",
		TimeFormat: "2006/01/02",
		Caller:     true,
		UseColor:   true,
		UseJSON:    true,
	}
	err := SetConfig(&cfg)
	require.NoError(t, err)

	kv := logger.KV{
		"id":  12345,
		"env": "staging",
	}

	Debug("test")
	Debugln("test")
	Debugf("test")
	Debugw("test", kv)

	Print("test")
	Println("test")
	Printf("test")

	Info("test")
	Infoln("test")
	Infof("test")
	Infow("test", kv)

	Warn("test")
	Warnln("test")
	Warnf("test")
	Warnw("test", kv)

	Error("test")
	Errorln("test")
	Errorf("test")
	Errorw("test", kv)

	// Fatal("test")
	// Fatalf("test")
	// Fatalw("test", nil)

	text, err := ioutil.ReadFile("test.log")
	if err != nil {
		t.Error(err)
	}
	textStr := string(text)

	// check caller, should contain this test file path
	caller := strings.Count(textStr, "tdk/go/log/log_test.go")
	expected := 19 // there are 19 functions

	// TODO: currently logrus caller always point to logrus.go file
	if caller != expected && engine != logger.Logrus {
		t.Errorf("Wrapper is not uniform, expected %d, got %d", expected, caller)
	}

	countKV := strings.Count(textStr, "1234")
	require.Equal(t, 4, countKV, "kv are not printed")

	// check time format
	timeIdxStart := strings.Index(textStr, `"time":"`) + 8
	timeIdxStop := strings.Index(textStr[timeIdxStart:], `"`) + timeIdxStart
	timeStr := textStr[timeIdxStart:timeIdxStop]

	_, err = time.Parse(cfg.TimeFormat, timeStr)
	require.NoError(t, err)
}

func testSetLogger(t *testing.T, engine logger.Engine, invalidLogger logger.Logger) {
	fmt.Println("TestSetLogger")
	// reset
	err := SetConfig(&Config{
		Engine:  engine,
		Level:   "debug",
		AppName: "tdk-app",
	})
	require.NoError(t, err)

	// test invalid
	err = SetLogger(-1, invalidLogger)
	require.Equal(t, errInvalidLevel, err)
	err = SetLogger(0, invalidLogger)
	require.Equal(t, errInvalidLogger, err)
	err = SetLogger(0, nil)
	require.Equal(t, errInvalidLogger, err)

	// test valid
	var logLevel logger.Level
	for logLevel = logger.DebugLevel; logLevel < logger.FatalLevel; logLevel++ {
		outFile := xid.New().String()
		defer os.Remove(outFile)

		// new logger
		newLogger, err := NewLogger(engine, &logger.Config{
			Level:   logger.DebugLevel,
			LogFile: outFile,
			UseJSON: true,
		})
		require.NoError(t, err)

		// set logger
		err = SetLogger(logLevel, newLogger)
		require.NoError(t, err)

		Debug("test")
		Info("test")
		Warn("test")
		Error("test")
		// Fatal("test")

		text, err := ioutil.ReadFile(outFile)
		require.NoError(t, err)
		textStr := string(text)

		var i logger.Level
		for i = logger.DebugLevel; i < logger.FatalLevel; i++ {
			require.Equal(t, i == logLevel, strings.Contains(textStr, logger.LevelToString(i)))
		}
	}
}

func testSetLevel(t *testing.T, engine logger.Engine) {
	fmt.Println("TestSetLevel")
	defer os.Remove("test.log")

	// reset
	err := SetConfig(&Config{
		Engine:    engine,
		AppName:   "tdk-app",
		Level:     "debug",
		LogFile:   "test.log",
		DebugFile: "test.log",
	})
	require.NoError(t, err)

	var logLevel logger.Level
	for logLevel = logger.DebugLevel; logLevel <= logger.FatalLevel; logLevel++ {
		// set level
		SetLevel(logLevel)

		Debug("test")
		Info("test")
		Warn("test")
		Error("test")
		// Fatal("test")

		text, err := ioutil.ReadFile("test.log")
		require.NoError(t, err)
		textStr := string(text)
		require.Equal(t, int(logger.FatalLevel-logLevel), strings.Count(textStr, "test"))

		os.Truncate("test.log", 0)
	}

	for logLevel = logger.FatalLevel; logLevel >= logger.DebugLevel; logLevel-- {
		// set level string
		SetLevelString(logger.LevelToString(logLevel))

		Debug("test")
		Info("test")
		Warn("test")
		Error("test")
		// Fatal("test")

		text, err := ioutil.ReadFile("test.log")
		require.NoError(t, err)
		textStr := string(text)
		require.Equal(t, int(logger.FatalLevel-logLevel), strings.Count(textStr, "test"))

		os.Truncate("test.log", 0)
	}
}

// TestAppName test that we write the appname properly
func TestAppName(t *testing.T) {
	const (
		appName = "tdk-app"
	)
	logFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(logFile.Name())

	SetConfig(&Config{
		Level:     "debug",
		AppName:   appName,
		LogFile:   logFile.Name(),
		DebugFile: logFile.Name(),
	})

	Debug("testappname")
	Info("testappname")
	Warn("testappname")
	Error("testappname")

	logContent, err := ioutil.ReadFile(logFile.Name())
	require.NoError(t, err)

	expected := `app=tdk-app`
	count := strings.Count(string(logContent), expected)
	require.Equal(t, 4, count)
}

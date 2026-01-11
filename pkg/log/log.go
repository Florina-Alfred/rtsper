package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var (
	logger   = log.New(os.Stdout, "", log.LstdFlags)
	logLevel = levelInfo
)

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

// Configure sets log output and level. level should be one of: debug,info,warn,error
func Configure(w io.Writer, level string) error {
	if w == nil {
		w = os.Stdout
	}
	logger = log.New(w, "", log.LstdFlags)
	switch strings.ToLower(level) {
	case "debug":
		logLevel = levelDebug
	case "info":
		logLevel = levelInfo
	case "warn":
		logLevel = levelWarn
	case "error":
		logLevel = levelError
	default:
		return fmt.Errorf("unknown log level: %s", level)
	}
	return nil
}

func output(level int, prefix string, format string, v ...interface{}) {
	if level < logLevel {
		return
	}
	logger.Printf(prefix+format, v...)
}

func Debug(format string, v ...interface{}) { output(levelDebug, "DEBUG: ", format, v...) }
func Info(format string, v ...interface{})  { output(levelInfo, "INFO: ", format, v...) }
func Warn(format string, v ...interface{})  { output(levelWarn, "WARN: ", format, v...) }
func Error(format string, v ...interface{}) { output(levelError, "ERROR: ", format, v...) }

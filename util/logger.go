package util

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/op/go-logging"
)

// CreateLogger creates a logger with the given prefix
func CreateLogger(prefix string) *logging.Logger {
	if prefix != "" {
		prefix = " " + prefix
	}
	return logging.MustGetLogger(prefix)
}

// SetWarningLoggingLevel sets the logging level to the warning
func SetWarningLoggingLevel() {
	logging.SetLevel(logging.WARNING, "")
}

// InitLogging must be called to set the logging string, initialize color and logging level
func InitLogging(levelName string, defaultLevel logging.Level, color bool) (int, error) {
	var format string
	if color {
		format = `[terragrunt%{module}] %{time:2006/01/02 15:04:05.000} %{color}%{level:-8s} %{message}%{color:reset}`
	} else {
		format = `[terragrunt%{module}] %{time:2006/01/02 15:04:05.000} %{level:-8s} %{message}`
	}

	logging.SetBackend(logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logging.MustStringFormatter(format)))

	level, err := getLoggingLevelFromString(levelName, defaultLevel)

	logging.SetLevel(level, "")
	return int(level), err
}

func getLoggingLevelFromString(level string, defaultLevel logging.Level) (logging.Level, error) {
	level = strings.TrimSpace(level)
	if level == "" {
		return defaultLevel, nil
	}

	levelNum, err := strconv.Atoi(level)
	if err == nil {
		return logging.Level(levelNum), nil
	}

	return logging.LogLevel(level)
}

// LogCatcher traps messsage containing logging level [LOGLEVEL] and redirect them to the logging system
type LogCatcher struct {
	Writer io.Writer
	Logger *logging.Logger
}

// This methods intercepts every message written to stderr and determines if a logging
// function should be used.
func (catcher LogCatcher) Write(p []byte) (n int, err error) {
	if logMessage == nil {
		initLogCatcher()
	}
	if matches := logMessage.FindSubmatch(p); matches != nil {
		if level, err := logging.LogLevel(string(matches[1])); err == nil {
			var logFunc func(...interface{})
			// TODO: it would have been preferable to simply call the private
			// method func (l *Logger) log instead of having a switch case here
			switch level {
			case logging.CRITICAL:
				logFunc = catcher.Logger.Critical
			case logging.ERROR:
				logFunc = catcher.Logger.Error
			case logging.WARNING:
				logFunc = catcher.Logger.Warning
			case logging.NOTICE:
				logFunc = catcher.Logger.Notice
			case logging.INFO:
				logFunc = catcher.Logger.Info
			case logging.DEBUG:
				logFunc = catcher.Logger.Debug
			}
			if logFunc != nil {
				logFunc(string(matches[2]))
				return len(p), nil
			}
		}
	}
	return catcher.Writer.Write(p)
}

func initLogCatcher() {
	levelNames := make([]string, 0, 5)
	for level := logging.CRITICAL; level <= logging.DEBUG; level++ {
		levelNames = append(levelNames, level.String())
	}
	// https://regex101.com/r/joriQk/2
	logMessage = regexp.MustCompile(fmt.Sprintf(`(?im)^\s*\[(?P<type>%s)\]\s*(?P<message>.*?)\s*$`, strings.Join(levelNames, "|")))
}

var logMessage *regexp.Regexp

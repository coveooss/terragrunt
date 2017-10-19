package util

import (
	"os"
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
		format = `[terragrunt%{module}] %{time:2006/01/02 15:04:05} %{color}%{level:-8s} %{message}%{color:reset}`
	} else {
		format = `[terragrunt%{module}] %{time:2006/01/02 15:04:05} %{level:-8s} %{message}`
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

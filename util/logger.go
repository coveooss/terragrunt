package util

import (
	"github.com/op/go-logging"
	"os"
	"strconv"
	"strings"
)

// Create a logger with the given prefix
func CreateLogger(prefix string) *logging.Logger {
	if prefix != "" {
		prefix = " " + prefix
	}
	return logging.MustGetLogger(prefix)
}

func InitLogging(levelName string, defaultLevel logging.Level, color bool) error {
	var format string
	if color {
		format = `[terragrunt%{module}] %{time:2006/01/02 15:04:05}: %{color}%{level:-8s} %{message}%{color:reset}`
	} else {
		format = `[terragrunt%{module}] %{time:2006/01/02 15:04:05}: %{level:-8s} %{message}`
	}

	logging.SetBackend(logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logging.MustStringFormatter(format)))

	level, err := getLoggingLevelFromString(levelName, defaultLevel)

	logging.SetLevel(level, "")
	return err
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

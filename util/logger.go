package util

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/coveo/gotemplate/errors"
	"github.com/coveo/gotemplate/template"
	"github.com/coveo/gotemplate/utils"
	"github.com/op/go-logging"
)

// CreateLogger creates a logger with the given prefix
func CreateLogger(prefix string) *logging.Logger {
	return logging.MustGetLogger(prefix)
}

// SetLoggingLevel sets the logging level
func SetLoggingLevel(level int) {
	logging.SetLevel(logging.Level(level), "")
}

// GetLoggingLevel returns the current logging level
func GetLoggingLevel() logging.Level {
	return logging.GetLevel("")
}

// InitLogging must be called to set the logging string, initialize color and logging level
func InitLogging(levelName string, defaultLevel logging.Level, color bool) (int, error) {
	var format string
	if color {
		format = `[terragrunt:%{module}] %{time:2006/01/02 15:04:05.000} %{color}%{level:-8s} %{message}%{color:reset}`
	} else {
		format = `[terragrunt:%{module}] %{time:2006/01/02 15:04:05.000} %{level:-8s} %{message}`
	}

	logging.SetBackend(logging.NewBackendFormatter(logging.NewLogBackend(os.Stderr, "", 0), logging.MustStringFormatter(format)))

	level, err := getLoggingLevelFromString(levelName, defaultLevel)

	logging.SetLevel(level, "")
	template.InitLogging()
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
	Writer    io.Writer
	Logger    *logging.Logger
	remaining string
}

// Close implements io.Closer
func (catcher *LogCatcher) Close() error {
	if catcher.remaining != "" {
		catcher.Write([]byte("\n"))
	}
	return nil
}

func (catcher *LogCatcher) write(s string) (int, error) {
	_, err := catcher.Writer.Write([]byte(s))
	return len(s), err
}

// This methods intercepts every message written to stream and determines if a logging
// function should be used.
func (catcher *LogCatcher) Write(p []byte) (n int, err error) {
	var buffer string
	if catcher.remaining != "" {
		n -= len(catcher.remaining)
		buffer = catcher.remaining + string(p)
		catcher.remaining = ""
	} else {
		buffer = string(p)
	}

	var errArray errors.Array
	if logMessage == nil {
		initLogCatcher()
	}
	if lastCR := strings.LastIndex(buffer, "\n"); lastCR >= 0 {
		catcher.remaining = buffer[lastCR+1:]
		buffer = buffer[:lastCR+1]
		n += len(catcher.remaining)
	}

	for {
		matches, _ := utils.MultiMatch(buffer, logMessage)
		if len(matches) == 0 {
			break
		}
		count, err := catcher.write(matches["before"])
		if err != nil {
			errArray = append(errArray, err)
		}
		n += count
		buffer = buffer[count:]

		if level, err := logging.LogLevel(matches["level"]); err == nil {
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
				logFunc(string(matches["message"]))
				toRemove := len(matches["toRemove"])
				buffer = buffer[toRemove:]
				n += toRemove
			}
		}
	}

	count, err := catcher.write(buffer)
	if err != nil {
		errArray = append(errArray, err)
	}
	n += count
	if len(errArray) > 0 {
		return n, errArray
	}
	return n, nil
}

func initLogCatcher() {
	levelNames := make([]string, 0, 5)
	for level := logging.CRITICAL; level <= logging.DEBUG; level++ {
		levelNames = append(levelNames, level.String())
	}
	// https://regex101.com/r/joriQk/3
	logMessage = regexp.MustCompile(fmt.Sprintf(`(?is)(?P<before>.*?\n)(?P<toRemove>[[:blank:]]*\[(?P<level>%s)\]\s*(?P<message>.*?)[[:blank:]]*\n)`, strings.Join(levelNames, "|")))
}

var logMessage *regexp.Regexp

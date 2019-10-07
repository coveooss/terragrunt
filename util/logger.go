package util

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/coveooss/gotemplate/v3/errors"
	"github.com/coveooss/gotemplate/v3/template"
	"github.com/coveooss/gotemplate/v3/utils"
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

	level, err := template.TryGetLoggingLevelFromString(levelName, defaultLevel)

	logging.SetLevel(level, "")
	template.InitLogging()
	return int(level), err
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
		catcher.Write(nil)
	}
	return nil
}

func (catcher *LogCatcher) write(s string) (out int, err error) {
	if out, err = catcher.Writer.Write([]byte(s)); err == nil && out != len(s) {
		err = fmt.Errorf("Write error, %d byte(s) out of %d written", out, len(s))
	}
	return
}

// This methods intercepts every message written to stream and determines if a logging
// function should be used.
func (catcher *LogCatcher) Write(writeBuffer []byte) (resultCount int, err error) {
	var buffer string
	if catcher.remaining != "" {
		resultCount -= len(catcher.remaining)
		buffer = catcher.remaining + string(writeBuffer)
		catcher.remaining = ""
	} else {
		buffer = string(writeBuffer)
	}

	var errArray errors.Array
	if logMessages == nil {
		initLogCatcher()
	}
	if writeBuffer != nil {
		lastCR := strings.LastIndex(buffer, "\n")
		catcher.remaining = buffer[lastCR+1:]
		buffer = buffer[:lastCR+1]
		resultCount += len(catcher.remaining)
	}

	for {
		searchBuffer, extraChar := buffer, 0
		if writeBuffer == nil {
			searchBuffer += "\n"
			extraChar = 1
		}
		matches, _ := utils.MultiMatch(searchBuffer, logMessages...)
		if len(matches) == 0 {
			break
		}
		count, err := catcher.write(matches["before"])
		if err != nil {
			errArray = append(errArray, err)
		}
		resultCount += count
		buffer = buffer[count:]

		var level logging.Level
		logFunc := catcher.Logger.Fatal
		if level, err = logging.LogLevel(matches["level"]); err == nil {
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
		}
		message := matches["message"]
		if prefix := matches["prefix"]; prefix != "" {
			message = fmt.Sprintf("%s %s %s", prefix, matches["level"], message)
		}
		logFunc(message)
		toRemove := len(matches["toRemove"]) - extraChar
		buffer = buffer[toRemove:]
		resultCount += toRemove
	}

	count, err := catcher.write(buffer)
	if err != nil {
		errArray = append(errArray, err)
	}
	resultCount += count

	switch len(errArray) {
	case 0:
		return resultCount, nil
	case 1:
		return resultCount, errArray[0]
	default:
		return resultCount, errArray
	}
}

func initLogCatcher() {
	levelNames := make([]string, 0, 5)
	for level := logging.CRITICAL; level <= logging.DEBUG; level++ {
		levelNames = append(levelNames, level.String())
	}
	choices := fmt.Sprintf(`\[(?P<level>%s|\[PANIC\])\]`, strings.Join(levelNames, "|"))
	expressions := []string{
		// https://regex101.com/r/jhhPLS/2
		`${choices}[[:blank:]]*{\s*${message}\s*}`,
		`[[:blank:]]*(?P<prefix>[^\n]*?)[[:blank:]]*${choices}[[:blank:]]*${message}[[:blank:]]*\n`,
	}

	for _, expr := range expressions {
		expr = fmt.Sprintf(`(?is)(?P<before>.*?)(?P<toRemove>%s)`, expr)
		expr = strings.Replace(expr, "${choices}", choices, -1)
		expr = strings.Replace(expr, "${message}", `(?P<message>.*?)`, -1)
		logMessages = append(logMessages, regexp.MustCompile(expr))
	}
}

var logMessages []*regexp.Regexp

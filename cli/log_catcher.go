package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/op/go-logging"
)

type logCatcher struct {
	writer io.Writer
	logger *logging.Logger
}

// This methods intercepts every message written to stderr and determines if a logging
// function should be used.
func (catcher logCatcher) Write(p []byte) (n int, err error) {
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
				logFunc = catcher.logger.Critical
			case logging.ERROR:
				logFunc = catcher.logger.Error
			case logging.WARNING:
				logFunc = catcher.logger.Warning
			case logging.NOTICE:
				logFunc = catcher.logger.Notice
			case logging.INFO:
				logFunc = catcher.logger.Info
			case logging.DEBUG:
				logFunc = catcher.logger.Debug
			}
			if logFunc != nil {
				logFunc(string(matches[2]))
				return len(p), nil
			}
		}
	}
	return catcher.writer.Write(p)
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

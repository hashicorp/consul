package logger

import (
	"io/ioutil"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/logutils"
)

var (
	allowedLogLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERR", "ERROR"}
)

// LevelFilter returns a LevelFilter that is configured with the log
// levels that we use.
func LevelFilter() *logutils.LevelFilter {
	var convertedLevels []logutils.LogLevel
	for _, l := range allowedLogLevels {
		convertedLevels = append(convertedLevels, logutils.LogLevel(l))
	}
	return &logutils.LevelFilter{
		Levels:   convertedLevels,
		MinLevel: "INFO",
		Writer:   ioutil.Discard,
	}
}

func AllowedLogLevels() []string {
	var c []string
	copy(c, allowedLogLevels)
	return c
}

// ValidateLogLevel verifies that a new log level is valid
func ValidateLogLevel(minLevel string) bool {
	newLevel := strings.ToUpper(minLevel)
	for _, level := range allowedLogLevels {
		if level == newLevel {
			return true
		}
	}
	return false
}

// Backwards compatibility with former ERR log level
func LevelFromString(level string) hclog.Level {
	if strings.ToUpper(level) == "ERR" {
		level = "ERROR"
	}
	return hclog.LevelFromString(level)
}

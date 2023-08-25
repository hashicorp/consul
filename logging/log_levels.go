package logging

import (
	"strings"

	"github.com/hashicorp/go-hclog"
)

var (
	allowedLogLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERR", "ERROR"}
)

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

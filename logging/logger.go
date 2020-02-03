package logging

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/mitchellh/cli"
)

// Config is used to set up logging.
type Config struct {
	// LogLevel is the minimum level to be logged.
	LogLevel string

	// LogJSON controls outputing logs in a JSON format.
	LogJSON bool

	// Name is the name the returned logger will use to prefix log lines.
	Name string

	// EnableSyslog controls forwarding to syslog.
	EnableSyslog bool

	// SyslogFacility is the destination for syslog forwarding.
	SyslogFacility string

	//LogFilePath is the path to write the logs to the user specified file.
	LogFilePath string

	//LogRotateDuration is the user specified time to rotate logs
	LogRotateDuration time.Duration

	//LogRotateBytes is the user specified byte limit to rotate logs
	LogRotateBytes int

	//LogRotateMaxFiles is the maximum number of past archived log files to keep
	LogRotateMaxFiles int
}

const (
	// defaultRotateDuration is the default time taken by the agent to rotate logs
	defaultRotateDuration = 24 * time.Hour
)

var (
	logRotateDuration time.Duration
	logRotateBytes    int
)

// Setup is used to perform setup of several logging objects:
//
// * A hclog.Logger is used to perform filtering by log level and write to io.Writer.
// * A GatedWriter is used to buffer logs until startup UI operations are
//   complete. After this is flushed then logs flow directly to output
//   destinations.
// * An io.Writer is provided as the sink for all logs to flow to.
//
// The provided ui object will get any log messages related to setting up
// logging itself, and will also be hooked up to the gated logger. The final bool
// parameter indicates if logging was set up successfully.
func Setup(config *Config, ui cli.Ui) (hclog.InterceptLogger, *GatedWriter, io.Writer, bool) {
	// The gated writer buffers logs at startup and holds until it's flushed.
	logGate := &GatedWriter{
		Writer: &cli.UiWriter{Ui: ui},
	}

	if !ValidateLogLevel(config.LogLevel) {
		ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			config.LogLevel, allowedLogLevels))
		return nil, nil, nil, false
	}

	// Set up syslog if it's enabled.
	var syslog io.Writer
	if config.EnableSyslog {
		retries := 12
		delay := 5 * time.Second
		for i := 0; i <= retries; i++ {
			l, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, config.SyslogFacility, "consul")
			if err == nil {
				syslog = &SyslogWrapper{l}
				break
			}

			ui.Error(fmt.Sprintf("Syslog setup error: %v", err))
			if i == retries {
				timeout := time.Duration(retries) * delay
				ui.Error(fmt.Sprintf("Syslog setup did not succeed within timeout (%s).", timeout.String()))
				return nil, nil, nil, false
			}

			ui.Error(fmt.Sprintf("Retrying syslog setup in %s...", delay.String()))
			time.Sleep(delay)
		}
	}
	writers := []io.Writer{logGate}

	var logOutput io.Writer
	if syslog != nil {
		writers = append(writers, syslog)
	}

	// Create a file logger if the user has specified the path to the log file
	if config.LogFilePath != "" {
		dir, fileName := filepath.Split(config.LogFilePath)
		// If a path is provided but has no fileName a default is provided.
		if fileName == "" {
			fileName = "consul.log"
		}
		// Try to enter the user specified log rotation duration first
		if config.LogRotateDuration != 0 {
			logRotateDuration = config.LogRotateDuration
		} else {
			// Default to 24 hrs if no rotation period is specified
			logRotateDuration = defaultRotateDuration
		}
		// User specified byte limit for log rotation if one is provided
		if config.LogRotateBytes != 0 {
			logRotateBytes = config.LogRotateBytes
		}
		logFile := &LogFile{
			fileName: fileName,
			logPath:  dir,
			duration: logRotateDuration,
			MaxBytes: logRotateBytes,
			MaxFiles: config.LogRotateMaxFiles,
		}
		writers = append(writers, logFile)
	}

	logOutput = io.MultiWriter(writers...)

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Level:      LevelFromString(config.LogLevel),
		Name:       config.Name,
		Output:     logOutput,
		JSONFormat: config.LogJSON,
	})

	return logger, logGate, logOutput, true
}

package logging

import (
	"encoding/json"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestLogger_SetupBasic(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "INFO",
	}
	ui := cli.NewMockUi()

	logger, gatedWriter, writer, ok := Setup(cfg, ui)
	require.True(ok)
	require.NotNil(gatedWriter)
	require.NotNil(writer)
	require.NotNil(logger)
}

func TestLogger_SetupInvalidLogLevel(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{}
	ui := cli.NewMockUi()

	_, _, _, ok := Setup(cfg, ui)
	require.False(ok)
	require.Contains(ui.ErrorWriter.String(), "Invalid log level")
}

func TestLogger_SetupLoggerErrorLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc   string
		before func(*Config)
	}{
		{
			desc: "ERR log level",
			before: func(cfg *Config) {
				cfg.LogLevel = "ERR"
			},
		},
		{
			desc: "ERROR log level",
			before: func(cfg *Config) {
				cfg.LogLevel = "ERROR"
			},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var cfg Config

			c.before(&cfg)
			require := require.New(t)
			ui := cli.NewMockUi()

			logger, gatedWriter, _, ok := Setup(&cfg, ui)
			require.True(ok)
			require.NotNil(logger)
			require.NotNil(gatedWriter)

			gatedWriter.Flush()

			logger.Error("test error msg")
			logger.Info("test info msg")

			require.Contains(ui.OutputWriter.String(), "[ERROR] test error msg")
			require.NotContains(ui.OutputWriter.String(), "[INFO]  test info msg")
		})
	}
}

func TestLogger_SetupLoggerDebugLevel(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "DEBUG",
	}
	ui := cli.NewMockUi()

	logger, gatedWriter, _, ok := Setup(cfg, ui)
	require.True(ok)
	require.NotNil(logger)
	require.NotNil(gatedWriter)

	gatedWriter.Flush()

	logger.Info("test info msg")
	logger.Debug("test debug msg")

	require.Contains(ui.OutputWriter.String(), "[INFO]  test info msg")
	require.Contains(ui.OutputWriter.String(), "[DEBUG] test debug msg")
}

func TestLogger_SetupLoggerWithName(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "DEBUG",
		Name:     "test-system",
	}
	ui := cli.NewMockUi()

	logger, gatedWriter, _, ok := Setup(cfg, ui)
	require.True(ok)
	require.NotNil(logger)
	require.NotNil(gatedWriter)

	gatedWriter.Flush()

	logger.Warn("test warn msg")

	require.Contains(ui.OutputWriter.String(), "[WARN]  test-system: test warn msg")
}

func TestLogger_SetupLoggerWithJSON(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "DEBUG",
		LogJSON:  true,
		Name:     "test-system",
	}
	ui := cli.NewMockUi()

	logger, gatedWriter, _, ok := Setup(cfg, ui)
	require.True(ok)
	require.NotNil(logger)
	require.NotNil(gatedWriter)

	gatedWriter.Flush()

	logger.Warn("test warn msg")

	var jsonOutput map[string]string
	err := json.Unmarshal(ui.OutputWriter.Bytes(), &jsonOutput)
	require.NoError(err)
	require.Contains(jsonOutput, "@level")
	require.Equal(jsonOutput["@level"], "warn")
	require.Contains(jsonOutput, "@message")
	require.Equal(jsonOutput["@message"], "test warn msg")
}

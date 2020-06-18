package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestLogger_SetupBasic(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "INFO",
	}

	logger, writer, err := Setup(cfg, nil)
	require.NoError(err)
	require.NotNil(writer)
	require.NotNil(logger)
}

func TestLogger_SetupInvalidLogLevel(t *testing.T) {
	t.Parallel()
	cfg := &Config{}

	_, _, err := Setup(cfg, nil)
	testutil.RequireErrorContains(t, err, "Invalid log level")
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
			var buf bytes.Buffer

			logger, _, err := Setup(&cfg, []io.Writer{&buf})
			require.NoError(err)
			require.NotNil(logger)

			logger.Error("test error msg")
			logger.Info("test info msg")

			output := buf.String()

			require.Contains(output, "[ERROR] test error msg")
			require.NotContains(output, "[INFO]  test info msg")
		})
	}
}

func TestLogger_SetupLoggerDebugLevel(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "DEBUG",
	}
	var buf bytes.Buffer

	logger, _, err := Setup(cfg, []io.Writer{&buf})
	require.NoError(err)
	require.NotNil(logger)

	logger.Info("test info msg")
	logger.Debug("test debug msg")

	output := buf.String()

	require.Contains(output, "[INFO]  test info msg")
	require.Contains(output, "[DEBUG] test debug msg")
}

func TestLogger_SetupLoggerWithName(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "DEBUG",
		Name:     "test-system",
	}
	var buf bytes.Buffer

	logger, _, err := Setup(cfg, []io.Writer{&buf})
	require.NoError(err)
	require.NotNil(logger)

	logger.Warn("test warn msg")

	require.Contains(buf.String(), "[WARN]  test-system: test warn msg")
}

func TestLogger_SetupLoggerWithJSON(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	cfg := &Config{
		LogLevel: "DEBUG",
		LogJSON:  true,
		Name:     "test-system",
	}
	var buf bytes.Buffer

	logger, _, err := Setup(cfg, []io.Writer{&buf})
	require.NoError(err)
	require.NotNil(logger)

	logger.Warn("test warn msg")

	var jsonOutput map[string]string
	err = json.Unmarshal(buf.Bytes(), &jsonOutput)
	require.NoError(err)
	require.Contains(jsonOutput, "@level")
	require.Equal(jsonOutput["@level"], "warn")
	require.Contains(jsonOutput, "@message")
	require.Equal(jsonOutput["@message"], "test warn msg")
}

func TestLogger_SetupLoggerWithValidLogPath(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tmpDir := testutil.TempDir(t, t.Name())
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		LogLevel:    "INFO",
		LogFilePath: tmpDir + "/",
	}
	var buf bytes.Buffer

	logger, _, err := Setup(cfg, []io.Writer{&buf})
	require.NoError(err)
	require.NotNil(logger)
}

func TestLogger_SetupLoggerWithInValidLogPath(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	cfg := &Config{
		LogLevel:    "INFO",
		LogFilePath: "nonexistentdir/",
	}
	var buf bytes.Buffer

	logger, _, err := Setup(cfg, []io.Writer{&buf})
	require.Error(err)
	require.True(errors.Is(err, os.ErrNotExist))
	require.Nil(logger)
}

func TestLogger_SetupLoggerWithInValidLogPathPermission(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	tmpDir := "/tmp/" + t.Name()

	os.Mkdir(tmpDir, 0000)
	defer os.RemoveAll(tmpDir)

	cfg := &Config{
		LogLevel:    "INFO",
		LogFilePath: tmpDir + "/",
	}
	var buf bytes.Buffer

	logger, _, err := Setup(cfg, []io.Writer{&buf})
	require.Error(err)
	require.True(errors.Is(err, os.ErrPermission))
	require.Nil(logger)
}

package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestLogger_SetupBasic(t *testing.T) {
	cfg := Config{LogLevel: "INFO"}

	logger, err := Setup(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestLogger_SetupInvalidLogLevel(t *testing.T) {
	cfg := Config{}

	_, err := Setup(cfg, nil)
	testutil.RequireErrorContains(t, err, "Invalid log level")
}

func TestLogger_SetupLoggerErrorLevel(t *testing.T) {

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
			var buf bytes.Buffer

			logger, err := Setup(cfg, &buf)
			require.NoError(t, err)
			require.NotNil(t, logger)

			logger.Error("test error msg")
			logger.Info("test info msg")

			output := buf.String()

			require.Contains(t, output, "[ERROR] test error msg")
			require.NotContains(t, output, "[INFO]  test info msg")
		})
	}
}

func TestLogger_SetupLoggerDebugLevel(t *testing.T) {
	cfg := Config{LogLevel: "DEBUG"}
	var buf bytes.Buffer

	logger, err := Setup(cfg, &buf)
	require.NoError(t, err)
	require.NotNil(t, logger)

	logger.Info("test info msg")
	logger.Debug("test debug msg")

	output := buf.String()

	require.Contains(t, output, "[INFO]  test info msg")
	require.Contains(t, output, "[DEBUG] test debug msg")
}

func TestLogger_SetupLoggerWithName(t *testing.T) {
	cfg := Config{
		LogLevel: "DEBUG",
		Name:     "test-system",
	}
	var buf bytes.Buffer

	logger, err := Setup(cfg, &buf)
	require.NoError(t, err)
	require.NotNil(t, logger)

	logger.Warn("test warn msg")

	require.Contains(t, buf.String(), "[WARN]  test-system: test warn msg")
}

func TestLogger_SetupLoggerWithJSON(t *testing.T) {
	cfg := Config{
		LogLevel: "DEBUG",
		LogJSON:  true,
		Name:     "test-system",
	}
	var buf bytes.Buffer

	logger, err := Setup(cfg, &buf)
	require.NoError(t, err)
	require.NotNil(t, logger)

	logger.Warn("test warn msg")

	var jsonOutput map[string]string
	err = json.Unmarshal(buf.Bytes(), &jsonOutput)
	require.NoError(t, err)
	require.Contains(t, jsonOutput, "@level")
	require.Equal(t, jsonOutput["@level"], "warn")
	require.Contains(t, jsonOutput, "@message")
	require.Equal(t, jsonOutput["@message"], "test warn msg")
}

func TestLogger_SetupLoggerWithValidLogPath(t *testing.T) {

	tmpDir := testutil.TempDir(t, t.Name())

	cfg := Config{
		LogLevel:    "INFO",
		LogFilePath: tmpDir + "/",
	}
	var buf bytes.Buffer

	logger, err := Setup(cfg, &buf)
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestLogger_SetupLoggerWithInValidLogPath(t *testing.T) {

	cfg := Config{
		LogLevel:    "INFO",
		LogFilePath: "nonexistentdir/",
	}
	var buf bytes.Buffer

	logger, err := Setup(cfg, &buf)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrNotExist))
	require.Nil(t, logger)
}

func TestLogger_SetupLoggerWithInValidLogPathPermission(t *testing.T) {

	tmpDir := "/tmp/" + t.Name()

	os.Mkdir(tmpDir, 0000)
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		LogLevel:    "INFO",
		LogFilePath: tmpDir + "/",
	}
	var buf bytes.Buffer

	logger, err := Setup(cfg, &buf)
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrPermission))
	require.Nil(t, logger)
}

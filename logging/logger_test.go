// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
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

func TestLogger_SetupLoggerWithSublevels(t *testing.T) {
	type testcase struct {
		cfg    Config
		assert []string
	}
	logDump := func(logger hclog.Logger) {
		logger.Trace("trace")
		logger.Debug("debug")
		logger.Info("info")
		logger.Warn("warn")
		logger.Error("error")
	}
	run := func(t *testing.T, tc testcase) {
		var buf bytes.Buffer
		rootLogger, err := Setup(tc.cfg, &buf)
		require.NoError(t, err)
		require.NotNil(t, rootLogger)

		sub1Logger := rootLogger.Named("sub1")
		sub2Logger := rootLogger.Named("sub2")
		sub1subALogger := sub1Logger.Named("a")

		logDump(rootLogger)
		logDump(sub1Logger)
		logDump(sub2Logger)
		logDump(sub1subALogger)

		s := strings.TrimSpace(buf.String())
		ss := strings.Split(s, "\n")

		require.Len(t, ss, len(tc.assert), "expected %d log lines, got %d", len(tc.assert), len(ss))
		for i, got := range ss {
			assert.Contains(t, got, tc.assert[i])
		}
	}
	tcs := map[string]testcase{
		"root level info": {
			cfg: Config{
				Name:     "root",
				LogLevel: "info",
			},
			assert: []string{
				"root: info",
				"root: warn",
				"root: error",
				"root.sub1: info",
				"root.sub1: warn",
				"root.sub1: error",
				"root.sub2: info",
				"root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: info",
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"root level info overwrite by sublevel warn": {
			cfg: Config{
				Name:     "root",
				LogLevel: "info",
				LogSublevels: map[string]string{
					"root": "warn",
				},
			},
			assert: []string{
				"root: warn",
				"root: error",
				"root.sub1: warn",
				"root.sub1: error",
				"root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"root level info overwrite by sublevel debug": {
			cfg: Config{
				Name:     "root",
				LogLevel: "info",
				LogSublevels: map[string]string{
					"root": "debug",
				},
			},
			assert: []string{
				"root: debug", //
				"root: info",
				"root: warn",
				"root: error",
				"root.sub1: debug", //
				"root.sub1: info",
				"root.sub1: warn",
				"root.sub1: error",
				"root.sub2: debug", //
				"root.sub2: info",
				"root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: debug", //
				"root.sub1.a: info",
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"root level warn sub2 trace": {
			cfg: Config{
				Name:     "root",
				LogLevel: "warn",
				LogSublevels: map[string]string{
					"root.sub2": "trace",
				},
			},
			assert: []string{
				"root: warn",
				"root: error",
				"root.sub1: warn",
				"root.sub1: error",
				"root.sub2: trace", //
				"root.sub2: debug", //
				"root.sub2: info",  //
				"root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"root level warn sub2 error": {
			cfg: Config{
				Name:     "root",
				LogLevel: "warn",
				LogSublevels: map[string]string{
					"root.sub2": "error",
				},
			},
			assert: []string{
				"root: warn",
				"root: error",
				"root.sub1: warn",
				"root.sub1: error",
				// "root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"root level warn sub1a debug": {
			cfg: Config{
				Name:     "root",
				LogLevel: "warn",
				LogSublevels: map[string]string{
					"root.sub1.a": "debug",
				},
			},
			assert: []string{
				"root: warn",
				"root: error",
				"root.sub1: warn",
				"root.sub1: error",
				"root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: debug", //
				"root.sub1.a: info",  //
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"root level warn sub1 info sub1a debug": {
			cfg: Config{
				Name:     "root",
				LogLevel: "warn",
				LogSublevels: map[string]string{
					"root.sub1":   "info",
					"root.sub1.a": "debug",
				},
			},
			assert: []string{
				"root: warn",
				"root: error",
				"root.sub1: info", //
				"root.sub1: warn",
				"root.sub1: error",
				"root.sub2: warn",
				"root.sub2: error",
				"root.sub1.a: debug", //
				"root.sub1.a: info",  //
				"root.sub1.a: warn",
				"root.sub1.a: error",
			},
		},
		"ensure prefix matching happens on whole names": {
			cfg: Config{
				Name:     "root",
				LogLevel: "error",
				LogSublevels: map[string]string{
					"root.su": "info",
				},
			},
			assert: []string{
				"root: error",
				"root.sub1: error",
				"root.sub2: error",
				"root.sub1.a: error",
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

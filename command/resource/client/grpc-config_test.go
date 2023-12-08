// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadGRPCConfig(t *testing.T) {
	t.Run("Default Config", func(t *testing.T) {
		// Test when defaultConfig is nil
		config, err := LoadGRPCConfig(nil)
		assert.NoError(t, err)
		assert.Equal(t, GetDefaultGRPCConfig(), config)
	})

	t.Run("Env Overwritten", func(t *testing.T) {
		// Test when environment variables are set

		// Mock environment variables
		os.Setenv(GRPCAddrEnvName, "localhost:8500")
		os.Setenv(GRPCTLSEnvName, "true")
		os.Setenv(GRPCTLSVerifyEnvName, "false")
		os.Setenv(GRPCClientCertEnvName, "/path/to/client.crt")
		os.Setenv(GRPCClientKeyEnvName, "/path/to/client.key")
		os.Setenv(GRPCCAFileEnvName, "/path/to/ca.crt")
		os.Setenv(GRPCCAPathEnvName, "/path/to/cacerts")

		defer func() {
			// Clean up environment variables after the test
			os.Clearenv()
		}()

		// Load and validate the configuration
		config, err := LoadGRPCConfig(nil)
		assert.NoError(t, err)
		expectedConfig := &GRPCConfig{
			Address:       "localhost:8500",
			GRPCTLS:       true,
			GRPCTLSVerify: false,
			CertFile:      "/path/to/client.crt",
			KeyFile:       "/path/to/client.key",
			CAFile:        "/path/to/ca.crt",
			CAPath:        "/path/to/cacerts",
		}
		assert.Equal(t, expectedConfig, config)
	})

	t.Run("Error Parsing Bool", func(t *testing.T) {
		// Test when there's an error parsing a boolean value from an environment variable

		// Mock environment variable with an invalid boolean value
		os.Setenv(GRPCTLSEnvName, "invalid_boolean_value")

		defer func() {
			// Clean up environment variables after the test
			os.Clearenv()
		}()

		// Load and expect an error
		config, err := LoadGRPCConfig(nil)
		assert.Error(t, err, "failed to parse CONSUL_GRPC_TLS: strconv.ParseBool: parsing \"invalid_boolean_value\": invalid syntax")
		assert.Nil(t, config)
	})
}

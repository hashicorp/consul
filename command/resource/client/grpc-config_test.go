// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
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

	// Test when environment variables are set
	t.Run("Env Overwritten", func(t *testing.T) {
		// Mock environment variables
		t.Setenv(GRPCAddrEnvName, "localhost:8500")
		t.Setenv(GRPCTLSEnvName, "true")
		t.Setenv(GRPCTLSVerifyEnvName, "false")
		t.Setenv(GRPCClientCertEnvName, "/path/to/client.crt")
		t.Setenv(GRPCClientKeyEnvName, "/path/to/client.key")
		t.Setenv(GRPCCAFileEnvName, "/path/to/ca.crt")
		t.Setenv(GRPCCAPathEnvName, "/path/to/cacerts")

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

	// Test when there's an error parsing a boolean value from an environment variable
	t.Run("Error Parsing Bool", func(t *testing.T) {
		// Mock environment variable with an invalid boolean value
		t.Setenv(GRPCTLSEnvName, "invalid_boolean_value")

		// Load and expect an error
		config, err := LoadGRPCConfig(nil)
		assert.Error(t, err, "failed to parse CONSUL_GRPC_TLS: strconv.ParseBool: parsing \"invalid_boolean_value\": invalid syntax")
		assert.Nil(t, config)
	})
}

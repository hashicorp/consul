// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeFlagsIntoGRPCConfig(t *testing.T) {
	t.Run("MergeFlagsIntoGRPCConfig", func(t *testing.T) {
		// Setup GRPCFlags with some flag values
		flags := &GRPCFlags{
			address:  TValue[string]{v: stringPointer("https://example.com:8502")},
			grpcTLS:  TValue[bool]{v: boolPointer(true)},
			certFile: TValue[string]{v: stringPointer("/path/to/client.crt")},
			keyFile:  TValue[string]{v: stringPointer("/path/to/client.key")},
			caFile:   TValue[string]{v: stringPointer("/path/to/ca.crt")},
			caPath:   TValue[string]{v: stringPointer("/path/to/cacerts")},
		}

		// Setup GRPCConfig with some initial values
		config := &GRPCConfig{
			Address:       "localhost:8500",
			GRPCTLS:       false,
			GRPCTLSVerify: true,
			CertFile:      "/path/to/default/client.crt",
			KeyFile:       "/path/to/default/client.key",
			CAFile:        "/path/to/default/ca.crt",
			CAPath:        "/path/to/default/cacerts",
		}

		// Call MergeFlagsIntoGRPCConfig to merge flag values into the config
		flags.MergeFlagsIntoGRPCConfig(config)

		// Validate the merged config
		expectedConfig := &GRPCConfig{
			Address:       "example.com:8502",
			GRPCTLS:       true,
			GRPCTLSVerify: true,
			CertFile:      "/path/to/client.crt",
			KeyFile:       "/path/to/client.key",
			CAFile:        "/path/to/ca.crt",
			CAPath:        "/path/to/cacerts",
		}

		assert.Equal(t, expectedConfig, config)
	})
	t.Run("MergeFlagsIntoGRPCConfig: allow empty values", func(t *testing.T) {
		// Setup GRPCFlags with some flag values
		flags := &GRPCFlags{
			address:  TValue[string]{v: stringPointer("http://example.com:8502")},
			grpcTLS:  TValue[bool]{},
			certFile: TValue[string]{v: stringPointer("/path/to/client.crt")},
			keyFile:  TValue[string]{v: stringPointer("/path/to/client.key")},
			caFile:   TValue[string]{v: stringPointer("/path/to/ca.crt")},
			caPath:   TValue[string]{v: stringPointer("/path/to/cacerts")},
		}

		// Setup GRPCConfig with some initial values
		config := &GRPCConfig{
			Address:       "localhost:8500",
			GRPCTLSVerify: true,
			CertFile:      "/path/to/default/client.crt",
			KeyFile:       "/path/to/default/client.key",
			CAFile:        "/path/to/default/ca.crt",
			CAPath:        "/path/to/default/cacerts",
		}

		// Call MergeFlagsIntoGRPCConfig to merge flag values into the config
		flags.MergeFlagsIntoGRPCConfig(config)

		// Validate the merged config
		expectedConfig := &GRPCConfig{
			Address:       "example.com:8502",
			GRPCTLS:       false,
			GRPCTLSVerify: true,
			CertFile:      "/path/to/client.crt",
			KeyFile:       "/path/to/client.key",
			CAFile:        "/path/to/ca.crt",
			CAPath:        "/path/to/cacerts",
		}

		assert.Equal(t, expectedConfig, config)
	})
}

// Utility function to convert string to string pointer
func stringPointer(s string) *string {
	return &s
}

// Utility function to convert bool to bool pointer
func boolPointer(b bool) *bool {
	return &b
}

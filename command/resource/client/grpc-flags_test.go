// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeFlagsIntoGRPCConfig(t *testing.T) {
	t.Run("MergeFlagsIntoGRPCConfig", func(t *testing.T) {
		// Setup GRPCFlags with some flag values
		flags := &GRPCFlags{
			address:   TValue[string]{v: stringPointer("https://example.com:8502")},
			grpcTLS:   TValue[bool]{v: boolPointer(true)},
			certFile:  TValue[string]{v: stringPointer("/path/to/client.crt")},
			keyFile:   TValue[string]{v: stringPointer("/path/to/client.key")},
			caFile:    TValue[string]{v: stringPointer("/path/to/ca.crt")},
			caPath:    TValue[string]{v: stringPointer("/path/to/cacerts")},
			token:     TValue[string]{v: stringPointer("token")},
			tokenFile: TValue[string]{v: stringPointer("/path/to/token/file")},
		}

		// Setup GRPCConfig with some initial values
		config := &GRPCConfig{
			Address:   "localhost:8500",
			GRPCTLS:   false,
			CertFile:  "/path/to/default/client.crt",
			KeyFile:   "/path/to/default/client.key",
			CAFile:    "/path/to/default/ca.crt",
			CAPath:    "/path/to/default/cacerts",
			Token:     "default-token",
			TokenFile: "/path/to/default/token/file",
		}

		// Call MergeFlagsIntoGRPCConfig to merge flag values into the config
		flags.MergeFlagsIntoGRPCConfig(config)

		// Validate the merged config
		expectedConfig := &GRPCConfig{
			Address:   "example.com:8502",
			GRPCTLS:   true,
			CertFile:  "/path/to/client.crt",
			KeyFile:   "/path/to/client.key",
			CAFile:    "/path/to/ca.crt",
			CAPath:    "/path/to/cacerts",
			Token:     "token",
			TokenFile: "/path/to/token/file",
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
			Address:  "localhost:8500",
			CertFile: "/path/to/default/client.crt",
			KeyFile:  "/path/to/default/client.key",
			CAFile:   "/path/to/default/ca.crt",
			CAPath:   "/path/to/default/cacerts",
		}

		// Call MergeFlagsIntoGRPCConfig to merge flag values into the config
		flags.MergeFlagsIntoGRPCConfig(config)

		// Validate the merged config
		expectedConfig := &GRPCConfig{
			Address:  "example.com:8502",
			GRPCTLS:  false,
			CertFile: "/path/to/client.crt",
			KeyFile:  "/path/to/client.key",
			CAFile:   "/path/to/ca.crt",
			CAPath:   "/path/to/cacerts",
		}

		assert.Equal(t, expectedConfig, config)
	})

	t.Run("MergeFlagsIntoGRPCConfig: TLS verification", func(t *testing.T) {
		cases := []struct {
			name           string
			flag           *bool
			configured     bool
			expectInsecure bool
		}{
			{name: "verifies by default", flag: nil, configured: false, expectInsecure: false},
			{name: "flag disables verification", flag: boolPointer(false), configured: false, expectInsecure: true},
			{name: "flag overrides env disabling verification", flag: boolPointer(true), configured: true, expectInsecure: false},
			{name: "unset flag keeps env disabling verification", flag: nil, configured: true, expectInsecure: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				flags := &GRPCFlags{
					grpcTLS:       TValue[bool]{v: boolPointer(true)},
					grpcTLSVerify: TValue[bool]{v: tc.flag},
				}
				config := &GRPCConfig{InsecureSkipVerify: tc.configured}

				flags.MergeFlagsIntoGRPCConfig(config)

				assert.True(t, config.GRPCTLS)
				assert.Equal(t, tc.expectInsecure, config.InsecureSkipVerify)
			})
		}
	})

	t.Run("MergeFlagsIntoGRPCConfig: TLS server name", func(t *testing.T) {
		flags := &GRPCFlags{tlsServerName: TValue[string]{v: stringPointer("server.dc1.consul")}}
		config := &GRPCConfig{TLSServerName: "from-env.example.com"}

		flags.MergeFlagsIntoGRPCConfig(config)

		assert.Equal(t, "server.dc1.consul", config.TLSServerName)
	})
}

func TestGRPCFlags_ClientFlags(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		expectTLS      bool
		expectInsecure bool
		expectAddress  string
	}{
		{
			// Flag parsing is unchanged: boolean values can be given with a
			// space or with "=", and any flags that follow are still parsed.
			name:          "space separated value",
			args:          []string{"-grpc-tls", "true", "-grpc-addr", "consul.internal:8503"},
			expectTLS:     true,
			expectAddress: "consul.internal:8503",
		},
		{
			name:           "explicit values",
			args:           []string{"-grpc-tls=true", "-grpc-tls-verify=false", "-tls-server-name=server.dc1.consul"},
			expectTLS:      true,
			expectInsecure: true,
			expectAddress:  "localhost:8502",
		},
		{
			name:          "explicit false",
			args:          []string{"-grpc-tls=false", "-grpc-tls-verify=true"},
			expectAddress: "localhost:8502",
		},
		{
			name:           "space separated verify value",
			args:           []string{"-grpc-tls", "true", "-grpc-tls-verify", "false", "-grpc-addr", "consul.internal:8503"},
			expectTLS:      true,
			expectInsecure: true,
			expectAddress:  "consul.internal:8503",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &GRPCFlags{}
			fs := flags.ClientFlags()
			require.NoError(t, fs.Parse(tc.args))
			require.Empty(t, fs.Args())

			config := GetDefaultGRPCConfig()
			flags.MergeFlagsIntoGRPCConfig(config)

			assert.Equal(t, tc.expectTLS, config.GRPCTLS)
			assert.Equal(t, tc.expectInsecure, config.InsecureSkipVerify)
			assert.Equal(t, tc.expectAddress, config.Address)
		})
	}

	t.Run("tls server name", func(t *testing.T) {
		flags := &GRPCFlags{}
		fs := flags.ClientFlags()
		require.NoError(t, fs.Parse([]string{"-grpc-tls=true", "-tls-server-name=server.dc1.consul"}))

		config := GetDefaultGRPCConfig()
		flags.MergeFlagsIntoGRPCConfig(config)

		assert.Equal(t, "server.dc1.consul", config.TLSServerName)
	})

	// -grpc-tls and -grpc-tls-verify take a value, as -grpc-tls always has, so
	// a bare flag is rejected rather than consuming the next flag as its value.
	t.Run("boolean flags require a value", func(t *testing.T) {
		for _, name := range []string{"-grpc-tls", "-grpc-tls-verify"} {
			t.Run(name, func(t *testing.T) {
				fs := (&GRPCFlags{}).ClientFlags()
				fs.SetOutput(io.Discard)
				err := fs.Parse([]string{name, "-grpc-addr=consul.internal:8503"})
				require.ErrorContains(t, err, "invalid value")
			})
		}
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

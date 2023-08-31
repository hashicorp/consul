// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/types"
)

func TestLoad(t *testing.T) {
	// Basically just testing that injection of the extra
	// source works.
	devMode := true
	builderOpts := LoadOpts{
		// putting this in dev mode so that the config validates
		// without having to specify a data directory
		DevMode: &devMode,
		DefaultConfig: FileSource{
			Name:   "test",
			Format: "hcl",
			Data:   `node_name = "hobbiton"`,
		},
		Overrides: []Source{
			FileSource{
				Name:   "overrides",
				Format: "json",
				Data:   `{"check_reap_interval": "1ms"}`,
			},
		},
	}

	result, err := Load(builderOpts)
	require.NoError(t, err)
	require.Empty(t, result.Warnings)
	cfg := result.RuntimeConfig
	require.NotNil(t, cfg)
	require.Equal(t, "hobbiton", cfg.NodeName)
	require.Equal(t, 1*time.Millisecond, cfg.CheckReapInterval)
}

func TestShouldParseFile(t *testing.T) {
	var testcases = []struct {
		filename     string
		configFormat string
		expected     bool
	}{
		{filename: "config.json", expected: true},
		{filename: "config.hcl", expected: true},
		{filename: "config", configFormat: "hcl", expected: true},
		{filename: "config.js", configFormat: "json", expected: true},
		{filename: "config.yaml", expected: false},
	}

	for _, tc := range testcases {
		name := fmt.Sprintf("filename=%s, format=%s", tc.filename, tc.configFormat)
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, shouldParseFile(tc.filename, tc.configFormat))
		})
	}
}

func TestNewBuilder_PopulatesSourcesFromConfigFiles(t *testing.T) {
	path, err := os.MkdirTemp("", t.Name())
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(path) })

	subpath := filepath.Join(path, "sub")
	err = os.Mkdir(subpath, 0755)
	require.NoError(t, err)

	for _, dir := range []string{path, subpath} {
		err = os.WriteFile(filepath.Join(dir, "a.hcl"), []byte("content a"), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dir, "b.json"), []byte("content b"), 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dir, "c.yaml"), []byte("content c"), 0644)
		require.NoError(t, err)
	}
	paths := []string{
		filepath.Join(path, "a.hcl"),
		filepath.Join(path, "b.json"),
		filepath.Join(path, "c.yaml"),
	}

	t.Run("fail on unknown files", func(t *testing.T) {
		_, err := newBuilder(LoadOpts{ConfigFiles: append(paths, subpath)})
		require.Error(t, err)
	})

	t.Run("skip on unknown files in dir", func(t *testing.T) {
		b, err := newBuilder(LoadOpts{ConfigFiles: []string{subpath}})
		require.NoError(t, err)

		expected := []Source{
			FileSource{Name: filepath.Join(subpath, "a.hcl"), Format: "hcl", Data: "content a"},
			FileSource{Name: filepath.Join(subpath, "b.json"), Format: "json", Data: "content b"},
		}
		require.Equal(t, expected, b.Sources)
		require.Len(t, b.Warnings, 1)
	})

	t.Run("force config format", func(t *testing.T) {
		b, err := newBuilder(LoadOpts{ConfigFiles: append(paths, subpath), ConfigFormat: "hcl"})
		require.NoError(t, err)

		expected := []Source{
			FileSource{Name: paths[0], Format: "hcl", Data: "content a"},
			FileSource{Name: paths[1], Format: "hcl", Data: "content b"},
			FileSource{Name: paths[2], Format: "hcl", Data: "content c"},
			FileSource{Name: filepath.Join(subpath, "a.hcl"), Format: "hcl", Data: "content a"},
			FileSource{Name: filepath.Join(subpath, "b.json"), Format: "hcl", Data: "content b"},
			FileSource{Name: filepath.Join(subpath, "c.yaml"), Format: "hcl", Data: "content c"},
		}
		require.Equal(t, expected, b.Sources)
	})
}

func TestLoad_NodeName(t *testing.T) {
	type testCase struct {
		name         string
		nodeName     string
		expectedWarn string
	}

	fn := func(t *testing.T, tc testCase) {
		opts := LoadOpts{
			FlagValues: FlagValuesTarget{
				Config: Config{
					NodeName: pString(tc.nodeName),
					DataDir:  pString("dir"),
				},
			},
		}
		patchLoadOptsShims(&opts)
		result, err := Load(opts)
		require.NoError(t, err)
		require.Len(t, result.Warnings, 1)
		require.Contains(t, result.Warnings[0], tc.expectedWarn)
	}

	var testCases = []testCase{
		{
			name:         "invalid character - unicode",
			nodeName:     "🐼",
			expectedWarn: `Node name "🐼" will not be discoverable via DNS due to invalid characters`,
		},
		{
			name:         "invalid character - slash",
			nodeName:     "thing/other/ok",
			expectedWarn: `Node name "thing/other/ok" will not be discoverable via DNS due to invalid characters`,
		},
		{
			name:         "too long",
			nodeName:     strings.Repeat("a", 66),
			expectedWarn: "due to it being too long.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestBuilder_unixPermissionsVal(t *testing.T) {

	b, _ := newBuilder(LoadOpts{
		FlagValues: FlagValuesTarget{
			Config: Config{
				NodeName: pString("foo"),
				DataDir:  pString("dir"),
			},
		},
	})

	goodmode := "666"
	badmode := "9666"

	patchLoadOptsShims(&b.opts)
	require.NoError(t, b.err)
	_ = b.unixPermissionsVal("local_bind_socket_mode", &goodmode)
	require.NoError(t, b.err)
	require.Len(t, b.Warnings, 0)

	_ = b.unixPermissionsVal("local_bind_socket_mode", &badmode)
	require.NotNil(t, b.err)
	require.Contains(t, b.err.Error(), "local_bind_socket_mode: invalid mode")
	require.Len(t, b.Warnings, 0)
}

func patchLoadOptsShims(opts *LoadOpts) {
	if opts.hostname == nil {
		opts.hostname = func() (string, error) {
			return "thehostname", nil
		}
	}
	if opts.getPrivateIPv4 == nil {
		opts.getPrivateIPv4 = func() ([]*net.IPAddr, error) {
			return []*net.IPAddr{ipAddr("10.0.0.1")}, nil
		}
	}
	if opts.getPublicIPv6 == nil {
		opts.getPublicIPv6 = func() ([]*net.IPAddr, error) {
			return []*net.IPAddr{ipAddr("dead:beef::1")}, nil
		}
	}
}

func TestLoad_HTTPMaxConnsPerClientExceedsRLimit(t *testing.T) {
	hcl := `
		limits{
			# We put a very high value to be sure to fail
			# This value is more than max on Windows as well
			http_max_conns_per_client = 16777217
		}`

	opts := LoadOpts{
		DefaultConfig: FileSource{
			Name:   "test",
			Format: "hcl",
			Data: `
		    ae_interval = "1m"
		    data_dir="/tmp/00000000001979"
			bind_addr = "127.0.0.1"
			advertise_addr = "127.0.0.1"
			datacenter = "dc1"
			bootstrap = true
			server = true
			node_id = "00000000001979"
			node_name = "Node-00000000001979"
		`,
		},
		HCL: []string{hcl},
	}

	_, err := Load(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "but limits.http_max_conns_per_client: 16777217 needs at least 16777237")
}

func TestLoad_EmptyClientAddr(t *testing.T) {

	type testCase struct {
		name                   string
		clientAddr             *string
		expectedWarningMessage *string
	}

	fn := func(t *testing.T, tc testCase) {
		opts := LoadOpts{
			FlagValues: FlagValuesTarget{
				Config: Config{
					ClientAddr: tc.clientAddr,
					DataDir:    pString("dir"),
				},
			},
		}
		patchLoadOptsShims(&opts)
		result, err := Load(opts)
		require.NoError(t, err)
		if tc.expectedWarningMessage != nil {
			require.Len(t, result.Warnings, 1)
			require.Contains(t, result.Warnings[0], *tc.expectedWarningMessage)
		}
	}

	var testCases = []testCase{
		{
			name:                   "empty string",
			clientAddr:             pString(""),
			expectedWarningMessage: pString("client_addr is empty, client services (DNS, HTTP, HTTPS, GRPC) will not be listening for connections"),
		},
		{
			name:                   "nil pointer",
			clientAddr:             nil, // defaults to 127.0.0.1
			expectedWarningMessage: nil, // expecting no warnings
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestBuilder_DurationVal_InvalidDuration(t *testing.T) {
	b := builder{}
	badDuration1 := "not-a-duration"
	badDuration2 := "also-not"
	b.durationVal("field1", &badDuration1)
	b.durationVal("field1", &badDuration2)

	require.Error(t, b.err)
	require.Contains(t, b.err.Error(), "2 errors")
	require.Contains(t, b.err.Error(), badDuration1)
	require.Contains(t, b.err.Error(), badDuration2)
}

func TestBuilder_DurationValWithDefaultMin(t *testing.T) {
	b := builder{}

	// Attempt to validate that a duration of 10 hours will not error when the min val is 1 hour.
	dur := "10h0m0s"
	b.durationValWithDefaultMin("field2", &dur, 24*7*time.Hour, time.Hour)
	require.NoError(t, b.err)

	// Attempt to validate that a duration of 1 min will error when the min val is 1 hour.
	dur = "0h1m0s"
	b.durationValWithDefaultMin("field1", &dur, 24*7*time.Hour, time.Hour)
	require.Error(t, b.err)
	require.Contains(t, b.err.Error(), "1 error")
}

func TestBuilder_ServiceVal_MultiError(t *testing.T) {
	b := builder{}
	b.serviceVal(&ServiceDefinition{
		Meta:       map[string]string{"": "empty-key"},
		Port:       intPtr(12345),
		SocketPath: strPtr("/var/run/socket.sock"),
		Checks: []CheckDefinition{
			{Interval: strPtr("bad-interval")},
		},
		Weights: &ServiceWeights{Passing: intPtr(-1)},
	})
	require.Error(t, b.err)
	require.Contains(t, b.err.Error(), "4 errors")
	require.Contains(t, b.err.Error(), "bad-interval")
	require.Contains(t, b.err.Error(), "Key cannot be blank")
	require.Contains(t, b.err.Error(), "Invalid weight")
	require.Contains(t, b.err.Error(), "cannot have both socket path")
}

func TestBuilder_ServiceVal_with_Check(t *testing.T) {
	b := builder{}
	svc := b.serviceVal(&ServiceDefinition{
		Name: strPtr("unbound"),
		ID:   strPtr("unbound"),
		Port: intPtr(12345),
		Checks: []CheckDefinition{
			{
				Interval: strPtr("5s"),
				UDP:      strPtr("localhost:53"),
			},
		},
	})
	require.NoError(t, b.err)
	require.Equal(t, 1, len(svc.Checks))
	require.Equal(t, "localhost:53", svc.Checks[0].UDP)
}

func intPtr(v int) *int {
	return &v
}

func TestBuilder_tlsVersion(t *testing.T) {
	b := builder{}

	validTLSVersion := "TLSv1_3"
	b.tlsVersion("tls.defaults.tls_min_version", &validTLSVersion)

	deprecatedTLSVersion := "tls11"
	b.tlsVersion("tls.defaults.tls_min_version", &deprecatedTLSVersion)

	invalidTLSVersion := "tls9"
	b.tlsVersion("tls.defaults.tls_min_version", &invalidTLSVersion)

	require.Error(t, b.err)
	require.Contains(t, b.err.Error(), "2 errors")
	require.Contains(t, b.err.Error(), deprecatedTLSVersion)
	require.Contains(t, b.err.Error(), invalidTLSVersion)
}

func TestBuilder_WarnGRPCTLS(t *testing.T) {
	tests := []struct {
		name      string
		hcl       string
		expectErr bool
	}{
		{
			name:      "success",
			hcl:       ``,
			expectErr: false,
		},
		{
			name: "grpc_tls is disabled but explicitly defined",
			hcl: `
			ports { grpc_tls = -1 }
			tls { grpc { cert_file = "defined" }}
			`,
			// This behavior is a little strange, but it allows users
			// to setup TLS and disable the port if they wish.
			expectErr: false,
		},
		{
			name: "grpc is disabled",
			hcl: `
			ports { grpc = -1 }
			tls { grpc { cert_file = "defined" }}
			`,
			expectErr: false,
		},
		{
			name: "grpc_tls is undefined with default manual cert",
			hcl: `
			tls { defaults { cert_file = "defined" }}
			`,
			expectErr: true,
		},
		{
			name: "grpc_tls is undefined with manual cert",
			hcl: `
			tls { grpc { cert_file = "defined" }}
			`,
			expectErr: true,
		},
		{
			name: "grpc_tls is undefined with auto encrypt",
			hcl: `
			auto_encrypt { tls = true }
			tls { grpc { use_auto_cert = true }}
			`,
			expectErr: true,
		},
		{
			name: "grpc_tls is undefined with auto config",
			hcl: `
			auto_config { enabled = true }
			tls { grpc { use_auto_cert = true }}
			`,
			expectErr: true,
		},
	}
	for _, tc := range tests {
		// using dev mode skips the need for a data dir
		// and enables both grpc ports by default.
		devMode := true
		builderOpts := LoadOpts{
			DevMode: &devMode,
			Overrides: []Source{
				FileSource{
					Name:   "overrides",
					Format: "hcl",
					Data:   tc.hcl,
				},
			},
		}
		_, err := Load(builderOpts)
		if tc.expectErr {
			require.Error(t, err)
			require.Contains(t, err.Error(), "listener no longer supports TLS")
		} else {
			require.NoError(t, err)
		}
	}
}

func TestBuilder_tlsCipherSuites(t *testing.T) {
	b := builder{}

	validCipherSuites := strings.Join([]string{
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	}, ",")
	b.tlsCipherSuites("tls.defaults.tls_cipher_suites", &validCipherSuites, types.TLSv1_2)
	require.NoError(t, b.err)

	unsupportedCipherSuites := strings.Join([]string{
		"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
	}, ",")
	b.tlsCipherSuites("tls.defaults.tls_cipher_suites", &unsupportedCipherSuites, types.TLSv1_2)

	invalidCipherSuites := strings.Join([]string{
		"cipherX",
	}, ",")
	b.tlsCipherSuites("tls.defaults.tls_cipher_suites", &invalidCipherSuites, types.TLSv1_2)

	b.tlsCipherSuites("tls.defaults.tls_cipher_suites", &validCipherSuites, types.TLSv1_3)

	require.Error(t, b.err)
	require.Contains(t, b.err.Error(), "3 errors")
	require.Contains(t, b.err.Error(), unsupportedCipherSuites)
	require.Contains(t, b.err.Error(), invalidCipherSuites)
	require.Contains(t, b.err.Error(), "cipher suites are not configurable")
}

func TestBuilder_parsePrefixFilter(t *testing.T) {
	t.Run("Check that 1.12 rpc metrics are parsed correctly.", func(t *testing.T) {
		type testCase struct {
			name                  string
			metricsPrefix         string
			prefixFilter          []string
			expectedAllowedPrefix []string
			expectedBlockedPrefix []string
		}

		var testCases = []testCase{
			{
				name:                  "no prefix filter",
				metricsPrefix:         "somePrefix",
				prefixFilter:          []string{},
				expectedAllowedPrefix: nil,
				expectedBlockedPrefix: []string{"somePrefix.rpc.server.call"},
			},
			{
				name:                  "operator enables 1.12 rpc metrics",
				metricsPrefix:         "somePrefix",
				prefixFilter:          []string{"+somePrefix.rpc.server.call"},
				expectedAllowedPrefix: []string{"somePrefix.rpc.server.call"},
				expectedBlockedPrefix: nil,
			},
			{
				name:                  "operator enables 1.12 rpc metrics",
				metricsPrefix:         "somePrefix",
				prefixFilter:          []string{"-somePrefix.rpc.server.call"},
				expectedAllowedPrefix: nil,
				expectedBlockedPrefix: []string{"somePrefix.rpc.server.call"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				b := builder{}
				telemetry := &Telemetry{
					MetricsPrefix: &tc.metricsPrefix,
					PrefixFilter:  tc.prefixFilter,
				}

				allowedPrefix, blockedPrefix := b.parsePrefixFilter(telemetry)

				require.Equal(t, tc.expectedAllowedPrefix, allowedPrefix)
				require.Equal(t, tc.expectedBlockedPrefix, blockedPrefix)
			})
		}
	})
}

func TestBuidler_hostMetricsWithCloud(t *testing.T) {
	devMode := true
	builderOpts := LoadOpts{
		DevMode: &devMode,
		DefaultConfig: FileSource{
			Name:   "test",
			Format: "hcl",
			Data:   `cloud{ resource_id = "abc" client_id = "abc" client_secret = "abc"}`,
		},
	}

	result, err := Load(builderOpts)
	require.NoError(t, err)
	require.Empty(t, result.Warnings)
	cfg := result.RuntimeConfig
	require.NotNil(t, cfg)
	require.True(t, cfg.Telemetry.EnableHostMetrics)
}

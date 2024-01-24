// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package loader

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/agent/hcp/bootstrap"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/lib"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestBootstrapConfigLoader(t *testing.T) {
	baseLoader := func(source config.Source) (config.LoadResult, error) {
		return config.Load(config.LoadOpts{
			DefaultConfig: source,
			HCL: []string{
				`server = true`,
				`bind_addr = "127.0.0.1"`,
				`data_dir = "/tmp/consul-data"`,
			},
		})
	}

	bootstrapLoader := func(source config.Source) (config.LoadResult, error) {
		return bootstrapConfigLoader(baseLoader, &bootstrap.RawBootstrapConfig{
			ConfigJSON:      `{"bootstrap_expect": 8}`,
			ManagementToken: "test-token",
		})(source)
	}

	result, err := bootstrapLoader(nil)
	require.NoError(t, err)

	// bootstrap_expect and management token are injected from bootstrap config received from HCP.
	require.Equal(t, 8, result.RuntimeConfig.BootstrapExpect)
	require.Equal(t, "test-token", result.RuntimeConfig.Cloud.ManagementToken)
}

func Test_finalizeRuntimeConfig(t *testing.T) {
	type testCase struct {
		rc       *config.RuntimeConfig
		cfg      *bootstrap.RawBootstrapConfig
		verifyFn func(t *testing.T, rc *config.RuntimeConfig)
	}
	run := func(t *testing.T, tc testCase) {
		finalizeRuntimeConfig(tc.rc, tc.cfg)
		tc.verifyFn(t, tc.rc)
	}

	tt := map[string]testCase{
		"set management token": {
			rc: &config.RuntimeConfig{},
			cfg: &bootstrap.RawBootstrapConfig{
				ManagementToken: "test-token",
			},
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				require.Equal(t, "test-token", rc.Cloud.ManagementToken)
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func Test_AddAclPolicyAccessControlHeader(t *testing.T) {
	type testCase struct {
		baseLoader ConfigLoader
		verifyFn   func(t *testing.T, rc *config.RuntimeConfig)
	}
	run := func(t *testing.T, tc testCase) {
		loader := AddAclPolicyAccessControlHeader(tc.baseLoader)
		result, err := loader(nil)
		require.NoError(t, err)
		tc.verifyFn(t, result.RuntimeConfig)
	}

	tt := map[string]testCase{
		"append to header if present": {
			baseLoader: func(source config.Source) (config.LoadResult, error) {
				return config.Load(config.LoadOpts{
					DefaultConfig: config.DefaultSource(),
					HCL: []string{
						`server = true`,
						`bind_addr = "127.0.0.1"`,
						`data_dir = "/tmp/consul-data"`,
						fmt.Sprintf(`http_config = { response_headers = { %s = "test" } }`, accessControlHeaderName),
					},
				})
			},
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				require.Equal(t, "test,x-consul-default-acl-policy", rc.HTTPResponseHeaders[accessControlHeaderName])
			},
		},
		"set header if not present": {
			baseLoader: func(source config.Source) (config.LoadResult, error) {
				return config.Load(config.LoadOpts{
					DefaultConfig: config.DefaultSource(),
					HCL: []string{
						`server = true`,
						`bind_addr = "127.0.0.1"`,
						`data_dir = "/tmp/consul-data"`,
					},
				})
			},
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				require.Equal(t, "x-consul-default-acl-policy", rc.HTTPResponseHeaders[accessControlHeaderName])
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func TestLoadConfig_Persistence(t *testing.T) {
	type testCase struct {
		// resourceID is the HCP resource ID. If set, a server is considered to be cloud-enabled.
		resourceID string

		// devMode indicates whether the loader should not have a data directory.
		devMode bool

		// verifyFn issues case-specific assertions.
		verifyFn func(t *testing.T, rc *config.RuntimeConfig)
	}

	run := func(t *testing.T, tc testCase) {
		dir, err := os.MkdirTemp(os.TempDir(), "bootstrap-test-")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dir) })

		s := hcp.NewMockHCPServer()
		s.AddEndpoint(bootstrap.TestEndpoint())

		// Use an HTTPS server since that's what the HCP SDK expects for auth.
		srv := httptest.NewTLSServer(s)
		defer srv.Close()

		caCert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
		require.NoError(t, err)

		pool := x509.NewCertPool()
		pool.AddCert(caCert)
		clientTLS := &tls.Config{RootCAs: pool}

		baseOpts := config.LoadOpts{
			HCL: []string{
				`server = true`,
				`bind_addr = "127.0.0.1"`,
				fmt.Sprintf(`http_config = { response_headers = { %s = "Content-Encoding" } }`, accessControlHeaderName),
				fmt.Sprintf(`cloud { client_id="test" client_secret="test" hostname=%q auth_url=%q resource_id=%q }`,
					srv.Listener.Addr().String(), srv.URL, tc.resourceID),
			},
		}
		if tc.devMode {
			baseOpts.DevMode = boolPtr(true)
		} else {
			baseOpts.HCL = append(baseOpts.HCL, fmt.Sprintf(`data_dir = %q`, dir))
		}

		baseLoader := func(source config.Source) (config.LoadResult, error) {
			baseOpts.DefaultConfig = source
			return config.Load(baseOpts)
		}

		ui := cli.NewMockUi()

		// Load initial config to check whether bootstrapping from HCP is enabled.
		initial, err := baseLoader(nil)
		require.NoError(t, err)

		// Override the client TLS config so that the test server can be trusted.
		initial.RuntimeConfig.Cloud.WithTLSConfig(clientTLS)
		client, err := hcpclient.NewClient(initial.RuntimeConfig.Cloud)
		require.NoError(t, err)

		loader, err := LoadConfig(context.Background(), client, initial.RuntimeConfig.DataDir, baseLoader, ui)
		require.NoError(t, err)

		// Load the agent config with the potentially wrapped loader.
		fromRemote, err := loader(nil)
		require.NoError(t, err)

		// HCP-enabled cases should fetch from HCP on the first run of LoadConfig.
		require.Contains(t, ui.OutputWriter.String(), "Fetching configuration from HCP")

		// Run case-specific verification.
		tc.verifyFn(t, fromRemote.RuntimeConfig)

		require.Empty(t, fromRemote.RuntimeConfig.ACLInitialManagementToken,
			"initial_management token should have been sanitized")

		if tc.devMode {
			// Re-running the bootstrap func below isn't relevant to dev mode
			// since they don't have a data directory to load data from.
			return
		}

		// Run LoadConfig again to exercise the logic of loading config from disk.
		loader, err = LoadConfig(context.Background(), client, initial.RuntimeConfig.DataDir, baseLoader, ui)
		require.NoError(t, err)

		fromDisk, err := loader(nil)
		require.NoError(t, err)

		// HCP-enabled cases should fetch from disk on the second run.
		require.Contains(t, ui.OutputWriter.String(), "Loaded HCP configuration from local disk")

		// Config loaded from disk should be the same as the one that was initially fetched from the HCP servers.
		require.Equal(t, fromRemote.RuntimeConfig, fromDisk.RuntimeConfig)
	}

	tt := map[string]testCase{
		"dev mode": {
			devMode: true,

			resourceID: "organization/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"project/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"consul.cluster/new-cluster-id",

			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				require.Empty(t, rc.DataDir)

				// Dev mode should have persisted certs since they can't be inlined.
				require.NotEmpty(t, rc.TLS.HTTPS.CertFile)
				require.NotEmpty(t, rc.TLS.HTTPS.KeyFile)
				require.NotEmpty(t, rc.TLS.HTTPS.CAFile)

				// Find the temporary directory they got stored in.
				dir := filepath.Dir(rc.TLS.HTTPS.CertFile)

				// Ensure we only stored the TLS materials.
				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, 3)

				haveFiles := make([]string, 3)
				for i, entry := range entries {
					haveFiles[i] = entry.Name()
				}

				wantFiles := []string{bootstrap.CAFileName, bootstrap.CertFileName, bootstrap.KeyFileName}
				require.ElementsMatch(t, wantFiles, haveFiles)
			},
		},
		"new cluster": {
			resourceID: "organization/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"project/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"consul.cluster/new-cluster-id",

			// New clusters should have received and persisted the whole suite of config.
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				dir := filepath.Join(rc.DataDir, bootstrap.SubDir)

				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, 6)

				files := []string{
					filepath.Join(dir, bootstrap.ConfigFileName),
					filepath.Join(dir, bootstrap.CAFileName),
					filepath.Join(dir, bootstrap.CertFileName),
					filepath.Join(dir, bootstrap.KeyFileName),
					filepath.Join(dir, bootstrap.TokenFileName),
					filepath.Join(dir, bootstrap.SuccessFileName),
				}
				for _, name := range files {
					_, err := os.Stat(name)
					require.NoError(t, err)
				}

				require.Equal(t, filepath.Join(dir, bootstrap.CertFileName), rc.TLS.HTTPS.CertFile)
				require.Equal(t, filepath.Join(dir, bootstrap.KeyFileName), rc.TLS.HTTPS.KeyFile)
				require.Equal(t, filepath.Join(dir, bootstrap.CAFileName), rc.TLS.HTTPS.CAFile)

				cert, key, caCerts, err := bootstrap.LoadCerts(dir)
				require.NoError(t, err)

				require.NoError(t, bootstrap.ValidateTLSCerts(cert, key, caCerts))
			},
		},
		"existing cluster": {
			resourceID: "organization/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"project/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"consul.cluster/" + bootstrap.TestExistingClusterID,

			// Existing clusters should have only received and persisted the management token.
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				dir := filepath.Join(rc.DataDir, bootstrap.SubDir)

				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, 3)

				files := []string{
					filepath.Join(dir, bootstrap.TokenFileName),
					filepath.Join(dir, bootstrap.SuccessFileName),
					filepath.Join(dir, bootstrap.ConfigFileName),
				}
				for _, name := range files {
					_, err := os.Stat(name)
					require.NoError(t, err)
				}
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidatePersistedConfig(t *testing.T) {
	type testCase struct {
		configContents string
		expectErr      string
	}

	run := func(t *testing.T, tc testCase) {
		dataDir, err := os.MkdirTemp(os.TempDir(), "load-bootstrap-test-")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dataDir) })

		dir := filepath.Join(dataDir, bootstrap.SubDir)
		require.NoError(t, lib.EnsurePath(dir, true))

		if tc.configContents != "" {
			name := filepath.Join(dir, bootstrap.ConfigFileName)
			require.NoError(t, os.WriteFile(name, []byte(tc.configContents), 0600))
		}

		err = validatePersistedConfig(dataDir)
		if tc.expectErr != "" {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectErr)
		} else {
			require.NoError(t, err)
		}
	}

	tt := map[string]testCase{
		"valid": {
			configContents: `{"bootstrap_expect": 1, "cloud": {"resource_id": "id"}}`,
		},
		"invalid config key": {
			configContents: `{"not_a_consul_agent_config_field": "zap"}`,
			expectErr:      "invalid config key not_a_consul_agent_config_field",
		},
		"invalid format": {
			configContents: `{"not_json" = "invalid"}`,
			expectErr:      "invalid character '=' after object key",
		},
		"missing configuration file": {
			expectErr: "no such file or directory",
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

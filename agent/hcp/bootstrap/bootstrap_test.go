// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bootstrap

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
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-uuid"
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
		return bootstrapConfigLoader(baseLoader, &RawBootstrapConfig{
			ConfigJSON:      `{"bootstrap_expect": 8}`,
			ManagementToken: "test-token",
		})(source)
	}

	result, err := bootstrapLoader(nil)
	require.NoError(t, err)

	// bootstrap_expect and management token are injected from bootstrap config received from HCP.
	require.Equal(t, 8, result.RuntimeConfig.BootstrapExpect)
	require.Equal(t, "test-token", result.RuntimeConfig.Cloud.ManagementToken)

	// Response header is always injected from a constant.
	require.Equal(t, "x-consul-default-acl-policy", result.RuntimeConfig.HTTPResponseHeaders[accessControlHeaderName])
}

func Test_finalizeRuntimeConfig(t *testing.T) {
	type testCase struct {
		rc       *config.RuntimeConfig
		cfg      *RawBootstrapConfig
		verifyFn func(t *testing.T, rc *config.RuntimeConfig)
	}
	run := func(t *testing.T, tc testCase) {
		finalizeRuntimeConfig(tc.rc, tc.cfg)
		tc.verifyFn(t, tc.rc)
	}

	tt := map[string]testCase{
		"set header if not present": {
			rc: &config.RuntimeConfig{},
			cfg: &RawBootstrapConfig{
				ManagementToken: "test-token",
			},
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				require.Equal(t, "test-token", rc.Cloud.ManagementToken)
				require.Equal(t, "x-consul-default-acl-policy", rc.HTTPResponseHeaders[accessControlHeaderName])
			},
		},
		"append to header if present": {
			rc: &config.RuntimeConfig{
				HTTPResponseHeaders: map[string]string{
					accessControlHeaderName: "Content-Encoding",
				},
			},
			cfg: &RawBootstrapConfig{
				ManagementToken: "test-token",
			},
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				require.Equal(t, "test-token", rc.Cloud.ManagementToken)
				require.Equal(t, "Content-Encoding,x-consul-default-acl-policy", rc.HTTPResponseHeaders[accessControlHeaderName])
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
		s.AddEndpoint(TestEndpoint())

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

				wantFiles := []string{caFileName, certFileName, keyFileName}
				require.ElementsMatch(t, wantFiles, haveFiles)
			},
		},
		"new cluster": {
			resourceID: "organization/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"project/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"consul.cluster/new-cluster-id",

			// New clusters should have received and persisted the whole suite of config.
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				dir := filepath.Join(rc.DataDir, subDir)

				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, 6)

				files := []string{
					filepath.Join(dir, configFileName),
					filepath.Join(dir, caFileName),
					filepath.Join(dir, certFileName),
					filepath.Join(dir, keyFileName),
					filepath.Join(dir, tokenFileName),
					filepath.Join(dir, successFileName),
				}
				for _, name := range files {
					_, err := os.Stat(name)
					require.NoError(t, err)
				}

				require.Equal(t, filepath.Join(dir, certFileName), rc.TLS.HTTPS.CertFile)
				require.Equal(t, filepath.Join(dir, keyFileName), rc.TLS.HTTPS.KeyFile)
				require.Equal(t, filepath.Join(dir, caFileName), rc.TLS.HTTPS.CAFile)

				cert, key, caCerts, err := loadCerts(dir)
				require.NoError(t, err)

				require.NoError(t, validateTLSCerts(cert, key, caCerts))
			},
		},
		"existing cluster": {
			resourceID: "organization/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"project/0b9de9a3-8403-4ca6-aba8-fca752f42100/" +
				"consul.cluster/" + TestExistingClusterID,

			// Existing clusters should have only received and persisted the management token.
			verifyFn: func(t *testing.T, rc *config.RuntimeConfig) {
				dir := filepath.Join(rc.DataDir, subDir)

				entries, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Len(t, entries, 3)

				files := []string{
					filepath.Join(dir, tokenFileName),
					filepath.Join(dir, successFileName),
					filepath.Join(dir, configFileName),
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

func Test_loadPersistedBootstrapConfig(t *testing.T) {
	type expect struct {
		loaded  bool
		warning string
	}
	type testCase struct {
		existingCluster        bool
		disableManagementToken bool
		mutateFn               func(t *testing.T, dir string)
		expect                 expect
	}

	run := func(t *testing.T, tc testCase) {
		dataDir, err := os.MkdirTemp(os.TempDir(), "load-bootstrap-test-")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dataDir) })

		dir := filepath.Join(dataDir, subDir)

		// Do some common setup as if we received config from HCP and persisted it to disk.
		require.NoError(t, lib.EnsurePath(dir, true))
		require.NoError(t, persistSuccessMarker(dir))

		if !tc.existingCluster {
			caCert, caKey, err := tlsutil.GenerateCA(tlsutil.CAOpts{})
			require.NoError(t, err)

			serverCert, serverKey, err := testLeaf(caCert, caKey)
			require.NoError(t, err)
			require.NoError(t, persistTLSCerts(dir, serverCert, serverKey, []string{caCert}))

			cfgJSON := `{"bootstrap_expect": 8}`
			require.NoError(t, persistBootstrapConfig(dir, cfgJSON))
		}

		var token string
		if !tc.disableManagementToken {
			token, err = uuid.GenerateUUID()
			require.NoError(t, err)
			require.NoError(t, persistManagementToken(dir, token))
		}

		// Optionally mutate the persisted data to trigger errors while loading.
		if tc.mutateFn != nil {
			tc.mutateFn(t, dir)
		}

		ui := cli.NewMockUi()
		cfg, loaded := loadPersistedBootstrapConfig(dataDir, ui)
		require.Equal(t, tc.expect.loaded, loaded, ui.ErrorWriter.String())
		if loaded {
			require.Equal(t, token, cfg.ManagementToken)
			require.Empty(t, ui.ErrorWriter.String())
		} else {
			require.Nil(t, cfg)
			require.Contains(t, ui.ErrorWriter.String(), tc.expect.warning)
		}
	}

	tt := map[string]testCase{
		"existing cluster with valid files": {
			existingCluster: true,
			// Don't mutate, files from setup are valid.
			mutateFn: nil,
			expect: expect{
				loaded:  true,
				warning: "",
			},
		},
		"existing cluster no token": {
			existingCluster:        true,
			disableManagementToken: true,
			expect: expect{
				loaded: false,
			},
		},
		"existing cluster no files": {
			existingCluster: true,
			mutateFn: func(t *testing.T, dir string) {
				// Remove all files
				require.NoError(t, os.RemoveAll(dir))
			},
			expect: expect{
				loaded: false,
				// No warnings since we assume we need to fetch config from HCP for the first time.
				warning: "",
			},
		},
		"new cluster with valid files": {
			// Don't mutate, files from setup are valid.
			mutateFn: nil,
			expect: expect{
				loaded:  true,
				warning: "",
			},
		},
		"new cluster with no token": {
			disableManagementToken: true,
			expect: expect{
				loaded: false,
			},
		},
		"new cluster some files": {
			mutateFn: func(t *testing.T, dir string) {
				// Remove one of the required files
				require.NoError(t, os.Remove(filepath.Join(dir, certFileName)))
			},
			expect: expect{
				loaded:  false,
				warning: "configuration files on disk are incomplete",
			},
		},
		"new cluster no files": {
			mutateFn: func(t *testing.T, dir string) {
				// Remove all files
				require.NoError(t, os.RemoveAll(dir))
			},
			expect: expect{
				loaded: false,
				// No warnings since we assume we need to fetch config from HCP for the first time.
				warning: "",
			},
		},
		"new cluster invalid cert": {
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, certFileName)
				require.NoError(t, os.WriteFile(name, []byte("not-a-cert"), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "invalid server certificate",
			},
		},
		"new cluster invalid CA": {
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, caFileName)
				require.NoError(t, os.WriteFile(name, []byte("not-a-ca-cert"), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "invalid CA certificate",
			},
		},
		"new cluster invalid config flag": {
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, configFileName)
				require.NoError(t, os.WriteFile(name, []byte(`{"not_a_consul_agent_config_field" = "zap"}`), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "failed to parse local bootstrap config",
			},
		},
		"existing cluster invalid token": {
			existingCluster: true,
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, tokenFileName)
				require.NoError(t, os.WriteFile(name, []byte("not-a-uuid"), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "is not a valid UUID",
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

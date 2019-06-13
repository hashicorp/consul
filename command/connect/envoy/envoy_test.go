package envoy

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

func TestCatalogCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

// testSetAndResetEnv sets the env vars passed as KEY=value strings in the
// current ENV and returns a func() that will undo it's work at the end of the
// test for use with defer.
func testSetAndResetEnv(t *testing.T, env []string) func() {
	old := make(map[string]*string)
	for _, e := range env {
		pair := strings.SplitN(e, "=", 2)
		current := os.Getenv(pair[0])
		if current != "" {
			old[pair[0]] = &current
		} else {
			// save it as a nil so we know to remove again
			old[pair[0]] = nil
		}
		os.Setenv(pair[0], pair[1])
	}
	// Return a func that will reset to old values
	return func() {
		for k, v := range old {
			if v == nil {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, *v)
			}
		}
	}
}

// This tests the args we use to generate the template directly because they
// encapsulate all the argument and default handling code which is where most of
// the logic is. We also allow generating golden files but only for cases that
// pass the test of having their template args generated as expected.
func TestGenerateConfig(t *testing.T) {
	cases := []struct {
		Name        string
		Flags       []string
		Env         []string
		Files       map[string]string
		ProxyConfig map[string]interface{}
		WantArgs    BootstrapTplArgs
		WantErr     string
	}{
		{
			Name:    "no-args",
			Flags:   []string{},
			Env:     []string{},
			WantErr: "No proxy ID specified",
		},
		{
			Name:  "defaults",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502", // Note this is the gRPC port
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name: "token-arg",
			Flags: []string{"-proxy-id", "test-proxy",
				"-token", "c9a52720-bf6c-4aa6-b8bc-66881a5ade95"},
			Env: []string{},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502", // Note this is the gRPC port
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
		},
		{
			Name:  "token-env",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env: []string{
				"CONSUL_HTTP_TOKEN=c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502", // Note this is the gRPC port
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
		},
		{
			Name: "token-file-arg",
			Flags: []string{"-proxy-id", "test-proxy",
				"-token-file", "@@TEMPDIR@@token.txt",
			},
			Env: []string{},
			Files: map[string]string{
				"token.txt": "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502", // Note this is the gRPC port
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
		},
		{
			Name:  "token-file-env",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env: []string{
				"CONSUL_HTTP_TOKEN_FILE=@@TEMPDIR@@token.txt",
			},
			Files: map[string]string{
				"token.txt": "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502", // Note this is the gRPC port
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
		},
		{
			Name: "grpc-addr-flag",
			Flags: []string{"-proxy-id", "test-proxy",
				"-grpc-addr", "localhost:9999"},
			Env: []string{},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				AgentAddress:          "127.0.0.1",
				AgentPort:             "9999",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "grpc-addr-env",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env: []string{
				"CONSUL_GRPC_ADDR=localhost:9999",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				AgentAddress:          "127.0.0.1",
				AgentPort:             "9999",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "access-log-path",
			Flags: []string{"-proxy-id", "test-proxy", "-admin-access-log-path", "/some/path/access.log"},
			Env:   []string{},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502",
				AdminAccessLogPath:    "/some/path/access.log",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "custom-bootstrap",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			ProxyConfig: map[string]interface{}{
				// Add a completely custom bootstrap template. Never mind if this is
				// invalid envoy config just as long as it works and gets the variables
				// interplated.
				"envoy_bootstrap_json_tpl": `
				{
					"admin": {
						"access_log_path": "/dev/null",
						"address": {
							"socket_address": {
								"address": "{{ .AdminBindAddress }}",
								"port_value": {{ .AdminBindPort }}
							}
						}
					},
					"node": {
						"cluster": "{{ .ProxyCluster }}",
						"id": "{{ .ProxyID }}"
					},
					custom_field = "foo"
				}`,
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "extra_-single",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			ProxyConfig: map[string]interface{}{
				// Add a custom sections with interpolated variables. These are all
				// invalid config syntax too but we are just testing they have the right
				// effect.
				"envoy_extra_static_clusters_json": `
				{
					"name": "fake_cluster_1"
				}`,
				"envoy_extra_static_listeners_json": `
				{
					"name": "fake_listener_1"
				}`,
				"envoy_extra_stats_sinks_json": `
				{
					"name": "fake_sink_1"
				}`,
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "extra_-multiple",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			ProxyConfig: map[string]interface{}{
				// Add a custom sections with interpolated variables. These are all
				// invalid config syntax too but we are just testing they have the right
				// effect.
				"envoy_extra_static_clusters_json": `
				{
					"name": "fake_cluster_1"
				},
				{
					"name": "fake_cluster_2"
				}`,
				"envoy_extra_static_listeners_json": `
				{
					"name": "fake_listener_1"
				},{
					"name": "fake_listener_2"
				}`,
				"envoy_extra_stats_sinks_json": `
				{
					"name": "fake_sink_1"
				} , { "name": "fake_sink_2" }`,
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "stats-config-override",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			ProxyConfig: map[string]interface{}{
				// Add a custom sections with interpolated variables. These are all
				// invalid config syntax too but we are just testing they have the right
				// effect.
				"envoy_stats_config_json": `
				{
					"name": "fake_config"
				}`,
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
		{
			Name:  "zipkin-tracing-config",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env:   []string{},
			ProxyConfig: map[string]interface{}{
				// Add a custom sections with interpolated variables. These are all
				// invalid config syntax too but we are just testing they have the right
				// effect.
				"envoy_tracing_json": `{
					"http": {
						"name": "envoy.zipkin",
						"config": {
							"collector_cluster": "zipkin",
							"collector_endpoint": "/api/v1/spans"
						}
					}
				}`,
				// Need to setup the cluster to send that too as well
				"envoy_extra_static_clusters_json": `{
					"name": "zipkin",
					"type": "STRICT_DNS",
					"connect_timeout": "5s",
					"load_assignment": {
						"cluster_name": "zipkin",
						"endpoints": [
							{
								"lb_endpoints": [
									{
										"endpoint": {
											"address": {
												"socket_address": {
													"address": "zipkin.service.consul",
													"port_value": 9411
												}
											}
										}
									}
								]
							}
						]
					}
				}`,
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:          "test-proxy",
				ProxyID:               "test-proxy",
				AgentAddress:          "127.0.0.1",
				AgentPort:             "8502",
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
			},
		},
	}

	copyAndReplaceAll := func(s []string, old, new string) []string {
		out := make([]string, len(s))
		for i, v := range s {
			out[i] = strings.ReplaceAll(v, old, new)
		}
		return out
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			require := require.New(t)

			testDir := testutil.TempDir(t, "envoytest")
			defer os.RemoveAll(testDir)

			if len(tc.Files) > 0 {
				for fn, fv := range tc.Files {
					fullname := filepath.Join(testDir, fn)
					require.NoError(ioutil.WriteFile(fullname, []byte(fv), 0600))
				}
			}

			ui := cli.NewMockUi()
			c := New(ui)

			// Run a mock agent API that just always returns the proxy config in the
			// test.
			srv := httptest.NewServer(testMockAgentProxyConfig(tc.ProxyConfig))
			defer srv.Close()

			// Set the agent HTTP address in ENV to be our mock
			tc.Env = append(tc.Env, "CONSUL_HTTP_ADDR="+srv.URL)

			testDirPrefix := testDir + string(filepath.Separator)

			myFlags := copyAndReplaceAll(tc.Flags, "@@TEMPDIR@@", testDirPrefix)
			myEnv := copyAndReplaceAll(tc.Env, "@@TEMPDIR@@", testDirPrefix)

			defer testSetAndResetEnv(t, myEnv)()

			// Run the command
			args := append([]string{"-bootstrap"}, myFlags...)
			code := c.Run(args)
			if tc.WantErr == "" {
				require.Equal(0, code, ui.ErrorWriter.String())
			} else {
				require.Equal(1, code, ui.ErrorWriter.String())
				require.Contains(ui.ErrorWriter.String(), tc.WantErr)
				return
			}

			// Verify we handled the env and flags right first to get correct template
			// args.
			got, err := c.templateArgs()
			require.NoError(err) // Error cases should have returned above
			require.Equal(&tc.WantArgs, got)

			// Actual template output goes to stdout direct to avoid prefix in UI, so
			// generate it again here to assert on.
			actual, err := c.generateConfig()
			require.NoError(err)

			// If we got the arg handling write, verify output
			golden := filepath.Join("testdata", tc.Name+".golden")
			if *update {
				ioutil.WriteFile(golden, actual, 0644)
			}

			expected, err := ioutil.ReadFile(golden)
			require.NoError(err)
			require.Equal(string(expected), string(actual))
		})
	}
}

func testMockAgentProxyConfig(cfg map[string]interface{}) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the proxy-id from the end of the URL (blindly assuming it's correct
		// format)
		proxyID := strings.TrimPrefix(r.URL.Path, "/v1/agent/service/")
		serviceID := strings.TrimSuffix(proxyID, "-sidecar-proxy")

		svc := api.AgentService{
			Kind:    api.ServiceKindConnectProxy,
			ID:      proxyID,
			Service: proxyID,
			Proxy: &api.AgentServiceConnectProxyConfig{
				DestinationServiceName: serviceID,
				DestinationServiceID:   serviceID,
				Config:                 cfg,
			},
		}

		cfgJSON, err := json.Marshal(svc)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(cfgJSON)
	})
}

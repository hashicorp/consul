package envoy

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
)

var update = flag.Bool("update", false, "update golden files")

func TestEnvoyCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestEnvoyGateway_Validation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		args   []string
		output string
	}{
		{
			"-register for non-gateway",
			[]string{"-register", "-proxy-id", "not-a-gateway"},
			"Auto-Registration can only be used for gateways",
		},
		{
			"-mesh-gateway and -gateway cannot be combined",
			[]string{"-register", "-mesh-gateway", "-gateway", "mesh"},
			"The mesh-gateway flag is deprecated and cannot be used alongside the gateway flag",
		},
		{
			"no proxy registration specified nor discovered",
			[]string{""},
			"No proxy ID specified",
		},
		{
			"-register with nodename",
			[]string{"-register", "-proxy-id", "gw-svc-id", "-node-name", "gw-node"},
			"'-register' cannot be used with '-node-name'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)
			c.init()

			code := c.Run(tc.args)
			if code == 0 {
				t.Errorf("%s: expected non-zero exit", tc.name)
			}

			output := ui.ErrorWriter.String()
			if !strings.Contains(output, tc.output) {
				t.Errorf("expected %q to contain %q", output, tc.output)
			}
		})
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
		require.NoError(t, os.Setenv(pair[0], pair[1]))
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

type generateConfigTestCase struct {
	Name              string
	TLSServer         bool
	Flags             []string
	Env               []string
	Files             map[string]string
	ProxyConfig       map[string]interface{}
	NamespacesEnabled bool
	XDSPorts          agent.GRPCPorts // only used for testing custom-configured grpc port
	AgentSelf110      bool            // fake the agent API from versions v1.10 and earlier
	WantArgs          BootstrapTplArgs
	WantErr           string
}

// This tests the args we use to generate the template directly because they
// encapsulate all the argument and default handling code which is where most of
// the logic is. We also allow generating golden files but only for cases that
// pass the test of having their template args generated as expected.
func TestGenerateConfig(t *testing.T) {
	cases := []generateConfigTestCase{
		{
			Name:    "no-args",
			Flags:   []string{},
			Env:     []string{},
			WantErr: "No proxy ID specified",
		},
		{
			Name:    "node-name without proxy-id",
			Flags:   []string{"-node-name", "test-node"},
			WantErr: "'-node-name' requires '-proxy-id'",
		},
		{
			Name:  "defaults",
			Flags: []string{"-proxy-id", "test-proxy"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusBackendPort: "",
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "defaults-nodemeta",
			Flags: []string{"-proxy-id", "test-proxy", "-node-name", "test-node"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				NodeName:     "test-node",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusBackendPort: "",
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name: "prometheus-metrics",
			Flags: []string{"-proxy-id", "test-proxy",
				"-prometheus-backend-port", "20100", "-prometheus-scrape-path", "/scrape-path"},
			ProxyConfig: map[string]interface{}{
				// When envoy_prometheus_bind_addr is set, if
				// PrometheusBackendPort is set, there will be a
				// "prometheus_backend" cluster in the Envoy configuration.
				"envoy_prometheus_bind_addr": "0.0.0.0:9000",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusBackendPort: "20100",
				PrometheusScrapePath:  "/scrape-path",
			},
		},
		{
			Name: "prometheus-metrics-tls-ca-file",
			Flags: []string{"-proxy-id", "test-proxy",
				"-prometheus-backend-port", "20100", "-prometheus-scrape-path", "/scrape-path",
				"-prometheus-ca-file", "../../../test/key/ourdomain.cer", "-prometheus-cert-file", "../../../test/key/ourdomain_server.cer",
				"-prometheus-key-file", "../../../test/key/ourdomain_server.key"},
			ProxyConfig: map[string]interface{}{
				// When envoy_prometheus_bind_addr is set, if
				// PrometheusBackendPort is set, there will be a
				// "prometheus_backend" cluster in the Envoy configuration.
				"envoy_prometheus_bind_addr": "0.0.0.0:9000",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusBackendPort: "20100",
				PrometheusScrapePath:  "/scrape-path",
				PrometheusCAFile:      "../../../test/key/ourdomain.cer",
				PrometheusCertFile:    "../../../test/key/ourdomain_server.cer",
				PrometheusKeyFile:     "../../../test/key/ourdomain_server.key",
			},
		},
		{
			Name: "prometheus-metrics-tls-ca-path",
			Flags: []string{"-proxy-id", "test-proxy",
				"-prometheus-backend-port", "20100", "-prometheus-scrape-path", "/scrape-path",
				"-prometheus-ca-path", "../../../test/ca_path", "-prometheus-cert-file", "../../../test/key/ourdomain_server.cer",
				"-prometheus-key-file", "../../../test/key/ourdomain_server.key"},
			ProxyConfig: map[string]interface{}{
				// When envoy_prometheus_bind_addr is set, if
				// PrometheusBackendPort is set, there will be a
				// "prometheus_backend" cluster in the Envoy configuration.
				"envoy_prometheus_bind_addr": "0.0.0.0:9000",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusBackendPort: "20100",
				PrometheusScrapePath:  "/scrape-path",
				PrometheusCAPath:      "../../../test/ca_path",
				PrometheusCertFile:    "../../../test/key/ourdomain_server.cer",
				PrometheusKeyFile:     "../../../test/key/ourdomain_server.key",
			},
		},
		{
			Name: "token-arg",
			Flags: []string{"-proxy-id", "test-proxy",
				"-token", "c9a52720-bf6c-4aa6-b8bc-66881a5ade95"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "token-env",
			Flags: []string{"-proxy-id", "test-proxy"},
			Env: []string{
				"CONSUL_HTTP_TOKEN=c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name: "token-file-arg",
			Flags: []string{"-proxy-id", "test-proxy",
				"-token-file", "@@TEMPDIR@@token.txt",
			},
			Files: map[string]string{
				"token.txt": "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
				PrometheusScrapePath:  "/metrics",
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
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				Token:                 "c9a52720-bf6c-4aa6-b8bc-66881a5ade95",
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name: "grpc-addr-flag",
			Flags: []string{"-proxy-id", "test-proxy",
				"-grpc-addr", "localhost:9999"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "9999",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
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
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "9999",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name: "grpc-addr-unix",
			Flags: []string{"-proxy-id", "test-proxy",
				"-grpc-addr", "unix:///var/run/consul.sock"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentSocket: "/var/run/consul.sock",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name: "grpc-addr-unix-with-tls",
			Flags: []string{"-proxy-id", "test-proxy",
				"-grpc-ca-file", "../../../test/ca/root.cer",
				"-grpc-addr", "unix:///var/run/consul.sock"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				GRPC: GRPC{
					AgentSocket: "/var/run/consul.sock",
					AgentTLS:    true,
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				AgentCAPEM:            `-----BEGIN CERTIFICATE-----\nMIIEtzCCA5+gAwIBAgIJAIewRMI8OnvTMA0GCSqGSIb3DQEBBQUAMIGYMQswCQYD\nVQQGEwJVUzELMAkGA1UECBMCQ0ExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xHDAa\nBgNVBAoTE0hhc2hpQ29ycCBUZXN0IENlcnQxDDAKBgNVBAsTA0RldjEWMBQGA1UE\nAxMNdGVzdC5pbnRlcm5hbDEgMB4GCSqGSIb3DQEJARYRdGVzdEBpbnRlcm5hbC5j\nb20wHhcNMTQwNDA3MTkwMTA4WhcNMjQwNDA0MTkwMTA4WjCBmDELMAkGA1UEBhMC\nVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRwwGgYDVQQK\nExNIYXNoaUNvcnAgVGVzdCBDZXJ0MQwwCgYDVQQLEwNEZXYxFjAUBgNVBAMTDXRl\nc3QuaW50ZXJuYWwxIDAeBgkqhkiG9w0BCQEWEXRlc3RAaW50ZXJuYWwuY29tMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxrs6JK4NpiOItxrpNR/1ppUU\nmH7p2BgLCBZ6eHdclle9J56i68adt8J85zaqphCfz6VDP58DsFx+N50PZyjQaDsU\nd0HejRqfHRMtg2O+UQkv4Z66+Vo+gc6uGuANi2xMtSYDVTAqqzF48OOPQDgYkzcG\nxcFZzTRFFZt2vPnyHj8cHcaFo/NMNVh7C3yTXevRGNm9u2mrbxCEeiHzFC2WUnvg\nU2jQuC7Fhnl33Zd3B6d3mQH6O23ncmwxTcPUJe6xZaIRrDuzwUcyhLj5Z3faag/f\npFIIcHSiHRfoqHLGsGg+3swId/zVJSSDHr7pJUu7Cre+vZa63FqDaooqvnisrQID\nAQABo4IBADCB/TAdBgNVHQ4EFgQUo/nrOfqvbee2VklVKIFlyQEbuJUwgc0GA1Ud\nIwSBxTCBwoAUo/nrOfqvbee2VklVKIFlyQEbuJWhgZ6kgZswgZgxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEcMBoGA1UE\nChMTSGFzaGlDb3JwIFRlc3QgQ2VydDEMMAoGA1UECxMDRGV2MRYwFAYDVQQDEw10\nZXN0LmludGVybmFsMSAwHgYJKoZIhvcNAQkBFhF0ZXN0QGludGVybmFsLmNvbYIJ\nAIewRMI8OnvTMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBADa9fV9h\ngjapBlkNmu64WX0Ufub5dsJrdHS8672P30S7ILB7Mk0W8sL65IezRsZnG898yHf9\n2uzmz5OvNTM9K380g7xFlyobSVq+6yqmmSAlA/ptAcIIZT727P5jig/DB7fzJM3g\njctDlEGOmEe50GQXc25VKpcpjAsNQi5ER5gowQ0v3IXNZs+yU+LvxLHc0rUJ/XSp\nlFCAMOqd5uRoMOejnT51G6krvLNzPaQ3N9jQfNVY4Q0zfs0M+6dRWvqfqB9Vyq8/\nPOLMld+HyAZEBk9zK3ZVIXx6XS4dkDnSNR91njLq7eouf6M7+7s/oMQZZRtAfQ6r\nwlW975rYa1ZqEdA=\n-----END CERTIFICATE-----\n`,
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:     "xds-addr-config",
			Flags:    []string{"-proxy-id", "test-proxy"},
			XDSPorts: agent.GRPCPorts{Plaintext: 9999, TLS: 0},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "9999",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:         "grpc-tls-addr-config",
			Flags:        []string{"-proxy-id", "test-proxy"},
			XDSPorts:     agent.GRPCPorts{Plaintext: 9997, TLS: 9998},
			AgentSelf110: false,
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "9998",
					AgentTLS:     true,
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:         "deprecated-grpc-addr-config",
			Flags:        []string{"-proxy-id", "test-proxy"},
			XDSPorts:     agent.GRPCPorts{Plaintext: 9999, TLS: 0},
			AgentSelf110: true,
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "9999",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "access-log-path",
			Flags: []string{"-proxy-id", "test-proxy", "-admin-access-log-path", "/some/path/access.log"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/some/path/access.log",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "missing-ca-file",
			Flags: []string{"-proxy-id", "test-proxy", "-ca-file", "some/path"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
			},
			WantErr: "Error loading CA File: open some/path: no such file or directory",
		},
		{
			Name:      "existing-ca-file",
			TLSServer: true,
			Flags:     []string{"-proxy-id", "test-proxy", "-grpc-ca-file", "../../../test/ca/root.cer"},
			Env:       []string{"CONSUL_GRPC_ADDR=https://127.0.0.1:8502"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
					AgentTLS:     true,
				},
				AgentCAPEM:            `-----BEGIN CERTIFICATE-----\nMIIEtzCCA5+gAwIBAgIJAIewRMI8OnvTMA0GCSqGSIb3DQEBBQUAMIGYMQswCQYD\nVQQGEwJVUzELMAkGA1UECBMCQ0ExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xHDAa\nBgNVBAoTE0hhc2hpQ29ycCBUZXN0IENlcnQxDDAKBgNVBAsTA0RldjEWMBQGA1UE\nAxMNdGVzdC5pbnRlcm5hbDEgMB4GCSqGSIb3DQEJARYRdGVzdEBpbnRlcm5hbC5j\nb20wHhcNMTQwNDA3MTkwMTA4WhcNMjQwNDA0MTkwMTA4WjCBmDELMAkGA1UEBhMC\nVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRwwGgYDVQQK\nExNIYXNoaUNvcnAgVGVzdCBDZXJ0MQwwCgYDVQQLEwNEZXYxFjAUBgNVBAMTDXRl\nc3QuaW50ZXJuYWwxIDAeBgkqhkiG9w0BCQEWEXRlc3RAaW50ZXJuYWwuY29tMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxrs6JK4NpiOItxrpNR/1ppUU\nmH7p2BgLCBZ6eHdclle9J56i68adt8J85zaqphCfz6VDP58DsFx+N50PZyjQaDsU\nd0HejRqfHRMtg2O+UQkv4Z66+Vo+gc6uGuANi2xMtSYDVTAqqzF48OOPQDgYkzcG\nxcFZzTRFFZt2vPnyHj8cHcaFo/NMNVh7C3yTXevRGNm9u2mrbxCEeiHzFC2WUnvg\nU2jQuC7Fhnl33Zd3B6d3mQH6O23ncmwxTcPUJe6xZaIRrDuzwUcyhLj5Z3faag/f\npFIIcHSiHRfoqHLGsGg+3swId/zVJSSDHr7pJUu7Cre+vZa63FqDaooqvnisrQID\nAQABo4IBADCB/TAdBgNVHQ4EFgQUo/nrOfqvbee2VklVKIFlyQEbuJUwgc0GA1Ud\nIwSBxTCBwoAUo/nrOfqvbee2VklVKIFlyQEbuJWhgZ6kgZswgZgxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEcMBoGA1UE\nChMTSGFzaGlDb3JwIFRlc3QgQ2VydDEMMAoGA1UECxMDRGV2MRYwFAYDVQQDEw10\nZXN0LmludGVybmFsMSAwHgYJKoZIhvcNAQkBFhF0ZXN0QGludGVybmFsLmNvbYIJ\nAIewRMI8OnvTMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBADa9fV9h\ngjapBlkNmu64WX0Ufub5dsJrdHS8672P30S7ILB7Mk0W8sL65IezRsZnG898yHf9\n2uzmz5OvNTM9K380g7xFlyobSVq+6yqmmSAlA/ptAcIIZT727P5jig/DB7fzJM3g\njctDlEGOmEe50GQXc25VKpcpjAsNQi5ER5gowQ0v3IXNZs+yU+LvxLHc0rUJ/XSp\nlFCAMOqd5uRoMOejnT51G6krvLNzPaQ3N9jQfNVY4Q0zfs0M+6dRWvqfqB9Vyq8/\nPOLMld+HyAZEBk9zK3ZVIXx6XS4dkDnSNR91njLq7eouf6M7+7s/oMQZZRtAfQ6r\nwlW975rYa1ZqEdA=\n-----END CERTIFICATE-----\n`,
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "missing-ca-path",
			Flags: []string{"-proxy-id", "test-proxy", "-ca-path", "some/path"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
			},
			WantErr: "lstat some/path: no such file or directory",
		},
		{
			Name:      "existing-ca-path",
			TLSServer: true,
			Flags:     []string{"-proxy-id", "test-proxy", "-grpc-ca-path", "../../../test/ca_path/"},
			Env:       []string{"CONSUL_GRPC_ADDR=https://127.0.0.1:8502"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
					AgentTLS:     true,
				},
				AgentCAPEM:            `-----BEGIN CERTIFICATE-----\nMIIFADCCAuqgAwIBAgIBATALBgkqhkiG9w0BAQswEzERMA8GA1UEAxMIQ2VydEF1\ndGgwHhcNMTUwNTExMjI0NjQzWhcNMjUwNTExMjI0NjU0WjATMREwDwYDVQQDEwhD\nZXJ0QXV0aDCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBALcMByyynHsA\n+K4PJwo5+XHygaEZAhPGvHiKQK2Cbc9NDm0ZTzx0rA/dRTZlvouhDyzcJHm+6R1F\nj6zQv7iaSC3qQtJiPnPsfZ+/0XhFZ3fQWMnfDiGbZpF1kJF01ofB6vnsuocFC0zG\naGC+SZiLAzs+QMP3Bebw1elCBIeoN+8NWnRYmLsYIaYGJGBSbNo/lCpLTuinofUn\nL3ehWEGv1INwpHnSVeN0Ml2GFe23d7PUlj/wNIHgUdpUR+KEJxIP3klwtsI3QpSH\nc4VjWdf4aIcka6K3IFuw+K0PUh3xAAPnMpAQOtCZk0AhF5rlvUbevC6jADxpKxLp\nOONmvCTer4LtyNURAoBH52vbK0r/DNcTpPEFV0IP66nXUFgkk0mRKsu8HTb4IOkC\nX3K4mp18EiWUUtrHZAnNct0iIniDBqKK0yhSNhztG6VakVt/1WdQY9Ey3mNtxN1O\nthqWFKdpKUzPKYC3P6PfVpiE7+VbWTLLXba+8BPe8BxWPsVkjJqGSGnCte4COusz\nM8/7bbTgifwJfsepwFtZG53tvwjWlO46Exl30VoDNTaIGvs1fO0GqJlh2A7FN5F2\nS1rS5VYHtPK8QdmUSvyq+7JDBc1HNT5I2zsIQbNcLwDTZ5EsbU6QR7NHDJKxjv/w\nbs3eTXJSSNcFD74wRU10pXjgE5wOFu9TAgMBAAGjYzBhMA4GA1UdDwEB/wQEAwIA\nBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQHazgZ3Puiuc6K2LzgcX5b6fAC\nPzAfBgNVHSMEGDAWgBQHazgZ3Puiuc6K2LzgcX5b6fACPzALBgkqhkiG9w0BAQsD\nggIBAEmeNrSUhpHg1I8dtfqu9hCU/6IZThjtcFA+QcPkkMa+Z1k0SOtsgW8MdlcA\ngCf5g5yQZ0DdpWM9nDB6xDIhQdccm91idHgf8wmpEHUj0an4uyn2ESCt8eqrAWf7\nAClYORCASTYfguJCxcfvwtI1uqaOeCxSOdmFay79UVitVsWeonbCRGsVgBDifJxw\nG2oCQqoYAmXPM4J6syk5GHhB1O9MMq+g1+hOx9s+XHyTui9FL4V+IUO1ygVqEQB5\nPSiRBvcIsajSGVao+vK0gf2XfcXzqr3y3NhBky9rFMp1g+ykb2yWekV4WiROJlCj\nTsWwWZDRyjiGahDbho/XW8JciouHZhJdjhmO31rqW3HdFviCTdXMiGk3GQIzz/Jg\nP+enOaHXoY9lcxzDvY9z1BysWBgNvNrMnVge/fLP9o+a0a0PRIIVl8T0Ef3zeg1O\nCLCSy/1Vae5Tx63ZTFvGFdOSusYkG9rlAUHXZE364JRCKzM9Bz0bM+t+LaO0MaEb\nYoxcXEPU+gB2IvmARpInN3oHexR6ekuYHVTRGdWrdmuHFzc7eFwygRqTFdoCCU+G\nQZEkd+lOEyv0zvQqYg+Jp0AEGz2B2zB53uBVECtn0EqrSdPtRzUBSByXVs6QhSXn\neVmy+z3U3MecP63X6oSPXekqSyZFuegXpNNuHkjNoL4ep2ix\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIEtzCCA5+gAwIBAgIJAIewRMI8OnvTMA0GCSqGSIb3DQEBBQUAMIGYMQswCQYD\nVQQGEwJVUzELMAkGA1UECBMCQ0ExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xHDAa\nBgNVBAoTE0hhc2hpQ29ycCBUZXN0IENlcnQxDDAKBgNVBAsTA0RldjEWMBQGA1UE\nAxMNdGVzdC5pbnRlcm5hbDEgMB4GCSqGSIb3DQEJARYRdGVzdEBpbnRlcm5hbC5j\nb20wHhcNMTQwNDA3MTkwMTA4WhcNMjQwNDA0MTkwMTA4WjCBmDELMAkGA1UEBhMC\nVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRwwGgYDVQQK\nExNIYXNoaUNvcnAgVGVzdCBDZXJ0MQwwCgYDVQQLEwNEZXYxFjAUBgNVBAMTDXRl\nc3QuaW50ZXJuYWwxIDAeBgkqhkiG9w0BCQEWEXRlc3RAaW50ZXJuYWwuY29tMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxrs6JK4NpiOItxrpNR/1ppUU\nmH7p2BgLCBZ6eHdclle9J56i68adt8J85zaqphCfz6VDP58DsFx+N50PZyjQaDsU\nd0HejRqfHRMtg2O+UQkv4Z66+Vo+gc6uGuANi2xMtSYDVTAqqzF48OOPQDgYkzcG\nxcFZzTRFFZt2vPnyHj8cHcaFo/NMNVh7C3yTXevRGNm9u2mrbxCEeiHzFC2WUnvg\nU2jQuC7Fhnl33Zd3B6d3mQH6O23ncmwxTcPUJe6xZaIRrDuzwUcyhLj5Z3faag/f\npFIIcHSiHRfoqHLGsGg+3swId/zVJSSDHr7pJUu7Cre+vZa63FqDaooqvnisrQID\nAQABo4IBADCB/TAdBgNVHQ4EFgQUo/nrOfqvbee2VklVKIFlyQEbuJUwgc0GA1Ud\nIwSBxTCBwoAUo/nrOfqvbee2VklVKIFlyQEbuJWhgZ6kgZswgZgxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEcMBoGA1UE\nChMTSGFzaGlDb3JwIFRlc3QgQ2VydDEMMAoGA1UECxMDRGV2MRYwFAYDVQQDEw10\nZXN0LmludGVybmFsMSAwHgYJKoZIhvcNAQkBFhF0ZXN0QGludGVybmFsLmNvbYIJ\nAIewRMI8OnvTMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBADa9fV9h\ngjapBlkNmu64WX0Ufub5dsJrdHS8672P30S7ILB7Mk0W8sL65IezRsZnG898yHf9\n2uzmz5OvNTM9K380g7xFlyobSVq+6yqmmSAlA/ptAcIIZT727P5jig/DB7fzJM3g\njctDlEGOmEe50GQXc25VKpcpjAsNQi5ER5gowQ0v3IXNZs+yU+LvxLHc0rUJ/XSp\nlFCAMOqd5uRoMOejnT51G6krvLNzPaQ3N9jQfNVY4Q0zfs0M+6dRWvqfqB9Vyq8/\nPOLMld+HyAZEBk9zK3ZVIXx6XS4dkDnSNR91njLq7eouf6M7+7s/oMQZZRtAfQ6r\nwlW975rYa1ZqEdA=\n-----END CERTIFICATE-----\n`,
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "custom-bootstrap",
			Flags: []string{"-proxy-id", "test-proxy"},
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
					"custom_field": "foo"
				}`,
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "extra_-single",
			Flags: []string{"-proxy-id", "test-proxy"},
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
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "extra_-multiple",
			Flags: []string{"-proxy-id", "test-proxy"},
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
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "stats-config-override",
			Flags: []string{"-proxy-id", "test-proxy"},
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
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "zipkin-tracing-config",
			Flags: []string{"-proxy-id", "test-proxy"},
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
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "CONSUL_HTTP_ADDR-with-https-scheme-does-not-affect-grpc-tls",
			Flags: []string{"-proxy-id", "test-proxy", "-ca-file", "../../../test/ca/root.cer"},
			Env:   []string{"CONSUL_HTTP_ADDR=https://127.0.0.1:8500"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
					AgentTLS:     false,
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "CONSUL_GRPC_ADDR-with-https-scheme-enables-tls",
			Flags: []string{"-proxy-id", "test-proxy", "-ca-file", "../../../test/ca/root.cer"},
			Env:   []string{"CONSUL_GRPC_ADDR=https://127.0.0.1:8502"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
					AgentTLS:     true,
				},
				AgentCAPEM:            `-----BEGIN CERTIFICATE-----\nMIIEtzCCA5+gAwIBAgIJAIewRMI8OnvTMA0GCSqGSIb3DQEBBQUAMIGYMQswCQYD\nVQQGEwJVUzELMAkGA1UECBMCQ0ExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xHDAa\nBgNVBAoTE0hhc2hpQ29ycCBUZXN0IENlcnQxDDAKBgNVBAsTA0RldjEWMBQGA1UE\nAxMNdGVzdC5pbnRlcm5hbDEgMB4GCSqGSIb3DQEJARYRdGVzdEBpbnRlcm5hbC5j\nb20wHhcNMTQwNDA3MTkwMTA4WhcNMjQwNDA0MTkwMTA4WjCBmDELMAkGA1UEBhMC\nVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRwwGgYDVQQK\nExNIYXNoaUNvcnAgVGVzdCBDZXJ0MQwwCgYDVQQLEwNEZXYxFjAUBgNVBAMTDXRl\nc3QuaW50ZXJuYWwxIDAeBgkqhkiG9w0BCQEWEXRlc3RAaW50ZXJuYWwuY29tMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxrs6JK4NpiOItxrpNR/1ppUU\nmH7p2BgLCBZ6eHdclle9J56i68adt8J85zaqphCfz6VDP58DsFx+N50PZyjQaDsU\nd0HejRqfHRMtg2O+UQkv4Z66+Vo+gc6uGuANi2xMtSYDVTAqqzF48OOPQDgYkzcG\nxcFZzTRFFZt2vPnyHj8cHcaFo/NMNVh7C3yTXevRGNm9u2mrbxCEeiHzFC2WUnvg\nU2jQuC7Fhnl33Zd3B6d3mQH6O23ncmwxTcPUJe6xZaIRrDuzwUcyhLj5Z3faag/f\npFIIcHSiHRfoqHLGsGg+3swId/zVJSSDHr7pJUu7Cre+vZa63FqDaooqvnisrQID\nAQABo4IBADCB/TAdBgNVHQ4EFgQUo/nrOfqvbee2VklVKIFlyQEbuJUwgc0GA1Ud\nIwSBxTCBwoAUo/nrOfqvbee2VklVKIFlyQEbuJWhgZ6kgZswgZgxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEcMBoGA1UE\nChMTSGFzaGlDb3JwIFRlc3QgQ2VydDEMMAoGA1UECxMDRGV2MRYwFAYDVQQDEw10\nZXN0LmludGVybmFsMSAwHgYJKoZIhvcNAQkBFhF0ZXN0QGludGVybmFsLmNvbYIJ\nAIewRMI8OnvTMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBADa9fV9h\ngjapBlkNmu64WX0Ufub5dsJrdHS8672P30S7ILB7Mk0W8sL65IezRsZnG898yHf9\n2uzmz5OvNTM9K380g7xFlyobSVq+6yqmmSAlA/ptAcIIZT727P5jig/DB7fzJM3g\njctDlEGOmEe50GQXc25VKpcpjAsNQi5ER5gowQ0v3IXNZs+yU+LvxLHc0rUJ/XSp\nlFCAMOqd5uRoMOejnT51G6krvLNzPaQ3N9jQfNVY4Q0zfs0M+6dRWvqfqB9Vyq8/\nPOLMld+HyAZEBk9zK3ZVIXx6XS4dkDnSNR91njLq7eouf6M7+7s/oMQZZRtAfQ6r\nwlW975rYa1ZqEdA=\n-----END CERTIFICATE-----\n`,
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "both-CONSUL_HTTP_ADDR-TLS-and-CONSUL_GRPC_ADDR-PLAIN-is-plain",
			Flags: []string{"-proxy-id", "test-proxy", "-ca-file", "../../../test/ca/root.cer"},
			Env: []string{
				"CONSUL_HTTP_ADDR=https://127.0.0.1:8500",
				"CONSUL_GRPC_ADDR=http://127.0.0.1:8502",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
					AgentTLS:     false,
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "both-CONSUL_HTTP_ADDR-PLAIN-and-CONSUL_GRPC_ADDR-TLS-is-tls",
			Flags: []string{"-proxy-id", "test-proxy", "-ca-file", "../../../test/ca/root.cer"},
			Env: []string{
				"CONSUL_HTTP_ADDR=http://127.0.0.1:8500",
				"CONSUL_GRPC_ADDR=https://127.0.0.1:8502",
			},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				// Should resolve IP, note this might not resolve the same way
				// everywhere which might make this test brittle but not sure what else
				// to do.
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
					AgentTLS:     true,
				},
				AgentCAPEM:            `-----BEGIN CERTIFICATE-----\nMIIEtzCCA5+gAwIBAgIJAIewRMI8OnvTMA0GCSqGSIb3DQEBBQUAMIGYMQswCQYD\nVQQGEwJVUzELMAkGA1UECBMCQ0ExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xHDAa\nBgNVBAoTE0hhc2hpQ29ycCBUZXN0IENlcnQxDDAKBgNVBAsTA0RldjEWMBQGA1UE\nAxMNdGVzdC5pbnRlcm5hbDEgMB4GCSqGSIb3DQEJARYRdGVzdEBpbnRlcm5hbC5j\nb20wHhcNMTQwNDA3MTkwMTA4WhcNMjQwNDA0MTkwMTA4WjCBmDELMAkGA1UEBhMC\nVVMxCzAJBgNVBAgTAkNBMRYwFAYDVQQHEw1TYW4gRnJhbmNpc2NvMRwwGgYDVQQK\nExNIYXNoaUNvcnAgVGVzdCBDZXJ0MQwwCgYDVQQLEwNEZXYxFjAUBgNVBAMTDXRl\nc3QuaW50ZXJuYWwxIDAeBgkqhkiG9w0BCQEWEXRlc3RAaW50ZXJuYWwuY29tMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxrs6JK4NpiOItxrpNR/1ppUU\nmH7p2BgLCBZ6eHdclle9J56i68adt8J85zaqphCfz6VDP58DsFx+N50PZyjQaDsU\nd0HejRqfHRMtg2O+UQkv4Z66+Vo+gc6uGuANi2xMtSYDVTAqqzF48OOPQDgYkzcG\nxcFZzTRFFZt2vPnyHj8cHcaFo/NMNVh7C3yTXevRGNm9u2mrbxCEeiHzFC2WUnvg\nU2jQuC7Fhnl33Zd3B6d3mQH6O23ncmwxTcPUJe6xZaIRrDuzwUcyhLj5Z3faag/f\npFIIcHSiHRfoqHLGsGg+3swId/zVJSSDHr7pJUu7Cre+vZa63FqDaooqvnisrQID\nAQABo4IBADCB/TAdBgNVHQ4EFgQUo/nrOfqvbee2VklVKIFlyQEbuJUwgc0GA1Ud\nIwSBxTCBwoAUo/nrOfqvbee2VklVKIFlyQEbuJWhgZ6kgZswgZgxCzAJBgNVBAYT\nAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEcMBoGA1UE\nChMTSGFzaGlDb3JwIFRlc3QgQ2VydDEMMAoGA1UECxMDRGV2MRYwFAYDVQQDEw10\nZXN0LmludGVybmFsMSAwHgYJKoZIhvcNAQkBFhF0ZXN0QGludGVybmFsLmNvbYIJ\nAIewRMI8OnvTMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEFBQADggEBADa9fV9h\ngjapBlkNmu64WX0Ufub5dsJrdHS8672P30S7ILB7Mk0W8sL65IezRsZnG898yHf9\n2uzmz5OvNTM9K380g7xFlyobSVq+6yqmmSAlA/ptAcIIZT727P5jig/DB7fzJM3g\njctDlEGOmEe50GQXc25VKpcpjAsNQi5ER5gowQ0v3IXNZs+yU+LvxLHc0rUJ/XSp\nlFCAMOqd5uRoMOejnT51G6krvLNzPaQ3N9jQfNVY4Q0zfs0M+6dRWvqfqB9Vyq8/\nPOLMld+HyAZEBk9zK3ZVIXx6XS4dkDnSNR91njLq7eouf6M7+7s/oMQZZRtAfQ6r\nwlW975rYa1ZqEdA=\n-----END CERTIFICATE-----\n`,
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "ingress-gateway",
			Flags: []string{"-proxy-id", "ingress-gateway-1", "-gateway", "ingress"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:       "ingress-gateway",
				ProxyID:            "ingress-gateway-1",
				ProxySourceService: "ingress-gateway",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "ingress-gateway-nodemeta",
			Flags: []string{"-proxy-id", "ingress-gateway-1", "-node-name", "test-node"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "ingress-gateway-1",
				ProxyID:      "ingress-gateway-1",
				NodeName:     "test-node",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name: "envoy-readiness-probe",
			Flags: []string{"-proxy-id", "test-proxy",
				"-envoy-ready-bind-address", "127.0.0.1", "-envoy-ready-bind-port", "21000"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster: "test-proxy",
				ProxyID:      "test-proxy",
				// We don't know this til after the lookup so it will be empty in the
				// initial args call we are testing here.
				ProxySourceService: "",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502", // Note this is the gRPC port
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusBackendPort: "",
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "ingress-gateway-address-specified",
			Flags: []string{"-proxy-id", "ingress-gateway", "-gateway", "ingress", "-address", "1.2.3.4:7777"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:       "ingress-gateway",
				ProxyID:            "ingress-gateway",
				ProxySourceService: "ingress-gateway",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "ingress-gateway-register-with-service-without-proxy-id",
			Flags: []string{"-gateway", "ingress", "-register", "-service", "my-gateway", "-address", "127.0.0.1:7777"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:       "my-gateway",
				ProxyID:            "my-gateway",
				ProxySourceService: "my-gateway",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "ingress-gateway-register-with-service-and-proxy-id",
			Flags: []string{"-gateway", "ingress", "-register", "-service", "my-gateway", "-proxy-id", "my-gateway-123", "-address", "127.0.0.1:7777"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:       "my-gateway",
				ProxyID:            "my-gateway-123",
				ProxySourceService: "my-gateway",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
		{
			Name:  "ingress-gateway-no-auto-register",
			Flags: []string{"-gateway", "ingress", "-address", "127.0.0.1:7777"},
			WantArgs: BootstrapTplArgs{
				ProxyCluster:       "ingress-gateway",
				ProxyID:            "ingress-gateway",
				ProxySourceService: "ingress-gateway",
				GRPC: GRPC{
					AgentAddress: "127.0.0.1",
					AgentPort:    "8502",
				},
				AdminAccessLogPath:    "/dev/null",
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				LocalAgentClusterName: xds.LocalAgentClusterName,
				PrometheusScrapePath:  "/metrics",
			},
		},
	}

	cases = append(cases, enterpriseGenerateConfigTestCases()...)

	copyAndReplaceAll := func(s []string, old, new string) []string {
		out := make([]string, len(s))
		for i, v := range s {
			out[i] = strings.ReplaceAll(v, old, new)
		}
		return out
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			testDir := testutil.TempDir(t, "envoytest")

			if len(tc.Files) > 0 {
				for fn, fv := range tc.Files {
					fullname := filepath.Join(testDir, fn)
					require.NoError(t, ioutil.WriteFile(fullname, []byte(fv), 0600))
				}
			}

			// Run a mock agent API that just always returns the proxy config in the
			// test.
			var srv *httptest.Server
			if tc.TLSServer {
				srv = httptest.NewTLSServer(testMockAgent(tc))
			} else {
				srv = httptest.NewServer(testMockAgent(tc))
			}
			defer srv.Close()

			testDirPrefix := testDir + string(filepath.Separator)
			myEnv := copyAndReplaceAll(tc.Env, "@@TEMPDIR@@", testDirPrefix)
			defer testSetAndResetEnv(t, myEnv)()

			client, err := api.NewClient(&api.Config{Address: srv.URL, TLSConfig: api.TLSConfig{InsecureSkipVerify: true}})
			require.NoError(t, err)

			ui := cli.NewMockUi()
			c := New(ui)
			// explicitly set the client to one which can connect to the httptest.Server
			c.client = client

			// Run the command
			myFlags := copyAndReplaceAll(tc.Flags, "@@TEMPDIR@@", testDirPrefix)
			args := append([]string{"-bootstrap"}, myFlags...)

			require.NoError(t, c.flags.Parse(args))
			code := c.run(c.flags.Args())
			if tc.WantErr == "" {
				require.Equal(t, 0, code, ui.ErrorWriter.String())
			} else {
				require.Equal(t, 1, code, ui.ErrorWriter.String())
				require.Contains(t, ui.ErrorWriter.String(), tc.WantErr)
				return
			}

			// Verify we handled the env and flags right first to get correct template
			// args.
			got, err := c.templateArgs()
			require.NoError(t, err) // Error cases should have returned above
			require.Equal(t, &tc.WantArgs, got)

			actual := ui.OutputWriter.Bytes()

			// If we got the arg handling write, verify output
			golden := filepath.Join("testdata", tc.Name+".golden")
			if *update {
				ioutil.WriteFile(golden, actual, 0644)
			}

			expected, err := ioutil.ReadFile(golden)
			require.NoError(t, err)
			require.Equal(t, string(expected), string(actual))
		})
	}
}

func TestEnvoy_GatewayRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	tt := []struct {
		name    string
		args    []string
		kind    api.ServiceKind
		id      string
		service string
	}{
		{
			name: "register gateway with proxy-id and name",
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-register",
				"-bootstrap",
				"-gateway", "ingress",
				"-service", "us-ingress",
				"-proxy-id", "us-ingress-1",
			},
			kind:    api.ServiceKindIngressGateway,
			id:      "us-ingress-1",
			service: "us-ingress",
		},
		{
			name: "register gateway without proxy-id with name",
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-register",
				"-bootstrap",
				"-gateway", "ingress",
				"-service", "us-ingress",
			},
			kind:    api.ServiceKindIngressGateway,
			id:      "us-ingress",
			service: "us-ingress",
		},
		{
			name: "register gateway without proxy-id and without name",
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-register",
				"-bootstrap",
				"-gateway", "ingress",
			},
			kind:    api.ServiceKindIngressGateway,
			id:      "ingress-gateway",
			service: "ingress-gateway",
		},
		{
			name: "register gateway with proxy-id without name",
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-register",
				"-bootstrap",
				"-gateway", "ingress",
				"-proxy-id", "us-ingress-1",
			},
			kind:    api.ServiceKindIngressGateway,
			id:      "us-ingress-1",
			service: "ingress-gateway",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			code := c.Run(tc.args)
			if code != 0 {
				t.Fatalf("bad exit code: %d. %#v", code, ui.ErrorWriter.String())
			}

			data, _, err := client.Agent().Service(tc.id, nil)
			assert.NoError(t, err)

			assert.NotNil(t, data)
			assert.Equal(t, tc.kind, data.Kind)
			assert.Equal(t, tc.id, data.ID)
			assert.Equal(t, tc.service, data.Service)
			assert.Equal(t, defaultGatewayPort, data.Port)
		})
	}
}

// testMockAgent combines testMockAgentProxyConfig and testMockAgentSelf,
// routing /agent/service/... requests to testMockAgentProxyConfig,
// routing /catalog/node-services/... requests to testMockCatalogNodeServiceList
// routing /agent/self requests to testMockAgentSelf.
func testMockAgent(tc generateConfigTestCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/agent/services"):
			testMockAgentGatewayConfig(tc.NamespacesEnabled)(w, r)
		case strings.Contains(r.URL.Path, "/agent/service"):
			testMockAgentProxyConfig(tc.ProxyConfig, tc.NamespacesEnabled)(w, r)
		case strings.Contains(r.URL.Path, "/agent/self"):
			testMockAgentSelf(tc.XDSPorts, tc.AgentSelf110)(w, r)
		case strings.Contains(r.URL.Path, "/catalog/node-services"):
			testMockCatalogNodeServiceList()(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func testMockAgentGatewayConfig(namespacesEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the proxy-id from the end of the URL (blindly assuming it's correct
		// format)
		params := r.URL.Query()
		filter := params["filter"][0]

		var kind api.ServiceKind
		switch {
		case strings.Contains(filter, string(api.ServiceKindTerminatingGateway)):
			kind = api.ServiceKindTerminatingGateway
		case strings.Contains(filter, string(api.ServiceKindIngressGateway)):
			kind = api.ServiceKindIngressGateway
		}

		svc := map[string]*api.AgentService{
			string(kind): {
				Kind:       kind,
				ID:         string(kind),
				Service:    string(kind),
				Datacenter: "dc1",
			},
		}

		if namespacesEnabled {
			svc[string(kind)].Namespace = namespaceFromQuery(r)
			svc[string(kind)].Partition = partitionFromQuery(r)
		}

		cfgJSON, err := json.Marshal(svc)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(cfgJSON)
	}
}

func namespaceFromQuery(r *http.Request) string {
	// Use the namespace in the request if there is one, otherwise
	// use-default.
	if queryNs := r.URL.Query().Get("ns"); queryNs != "" {
		return queryNs
	}
	return "default"
}

func partitionFromQuery(r *http.Request) string {
	// Use the partition in the request if there is one, otherwise
	// use-default.
	if queryAP := r.URL.Query().Get("partition"); queryAP != "" {
		return queryAP
	}
	return "default"
}

func testMockAgentProxyConfig(cfg map[string]interface{}, namespacesEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the proxy-id from the end of the URL (blindly assuming it's correct
		// format)
		proxyID := strings.TrimPrefix(r.URL.Path, "/v1/agent/service/")
		serviceID := strings.TrimSuffix(proxyID, "-proxy")

		svc := api.AgentService{
			Kind:    api.ServiceKindConnectProxy,
			ID:      proxyID,
			Service: proxyID,
			Proxy: &api.AgentServiceConnectProxyConfig{
				DestinationServiceName: serviceID,
				DestinationServiceID:   serviceID,
				Config:                 cfg,
			},
			Datacenter: "dc1",
		}

		if namespacesEnabled {
			svc.Namespace = namespaceFromQuery(r)
			svc.Partition = partitionFromQuery(r)
		}

		cfgJSON, err := json.Marshal(svc)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(cfgJSON)
	}
}

func testMockCatalogNodeServiceList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		quotedProxyID := strings.TrimPrefix(r.URL.Query().Get("filter"), "ID == ")
		proxyID := quotedProxyID[1 : len(quotedProxyID)-1]
		serviceID := strings.TrimSuffix(proxyID, "-proxy")

		var svcKind api.ServiceKind
		if strings.Contains(proxyID, "ingress-gateway") {
			svcKind = api.ServiceKindIngressGateway
		} else {
			svcKind = api.ServiceKindConnectProxy
		}

		var svcProxy api.AgentServiceConnectProxyConfig
		if svcKind == api.ServiceKindConnectProxy {
			svcProxy = api.AgentServiceConnectProxyConfig{
				DestinationServiceName: serviceID,
				DestinationServiceID:   serviceID,
			}
		}
		svc := api.AgentService{
			Kind:    svcKind,
			ID:      proxyID,
			Service: proxyID,
			Proxy:   &svcProxy,
		}

		nodeSvc := api.CatalogNodeServiceList{
			Node:     &api.Node{Datacenter: "dc1"},
			Services: []*api.AgentService{&svc},
		}

		cfgJSON, err := json.Marshal(nodeSvc)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(cfgJSON)
	}
}

func TestEnvoyCommand_canBindInternal(t *testing.T) {
	t.Parallel()
	type testCheck struct {
		expected bool
		addr     string
	}

	type testCase struct {
		ifAddrs []net.Addr
		checks  map[string]testCheck
	}

	parseIPNets := func(t *testing.T, in ...string) []net.Addr {
		var out []net.Addr
		for _, addr := range in {
			ip := net.ParseIP(addr)
			require.NotNil(t, ip)
			out = append(out, &net.IPNet{IP: ip})
		}
		return out
	}

	parseIPs := func(t *testing.T, in ...string) []net.Addr {
		var out []net.Addr
		for _, addr := range in {
			ip := net.ParseIP(addr)
			require.NotNil(t, ip)
			out = append(out, &net.IPAddr{IP: ip})
		}
		return out
	}

	cases := map[string]testCase{
		"IPNet": {
			parseIPNets(t, "10.3.0.2", "198.18.0.1", "2001:db8:a0b:12f0::1"),
			map[string]testCheck{
				"ipv4": {
					true,
					"10.3.0.2",
				},
				"secondary ipv4": {
					true,
					"198.18.0.1",
				},
				"ipv6": {
					true,
					"2001:db8:a0b:12f0::1",
				},
				"ipv4 not found": {
					false,
					"1.2.3.4",
				},
				"ipv6 not found": {
					false,
					"::ffff:192.168.0.1",
				},
			},
		},
		"IPAddr": {
			parseIPs(t, "10.3.0.2", "198.18.0.1", "2001:db8:a0b:12f0::1"),
			map[string]testCheck{
				"ipv4": {
					true,
					"10.3.0.2",
				},
				"secondary ipv4": {
					true,
					"198.18.0.1",
				},
				"ipv6": {
					true,
					"2001:db8:a0b:12f0::1",
				},
				"ipv4 not found": {
					false,
					"1.2.3.4",
				},
				"ipv6 not found": {
					false,
					"::ffff:192.168.0.1",
				},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			for checkName, check := range tcase.checks {
				t.Run(checkName, func(t *testing.T) {
					require.Equal(t, check.expected, canBindInternal(check.addr, tcase.ifAddrs))
				})
			}
		})
	}
}

// testMockAgentSelf returns an empty /v1/agent/self response except GRPC
// port is filled in to match the given wantXDSPort argument.
func testMockAgentSelf(wantXDSPorts agent.GRPCPorts, agentSelf110 bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := agent.Self{
			Config: map[string]interface{}{
				"Datacenter": "dc1",
			},
		}

		if agentSelf110 {
			resp.DebugConfig = map[string]interface{}{
				"GRPCPort": wantXDSPorts.Plaintext,
			}
		} else {
			resp.XDS = &agent.XDSSelf{
				// The deprecated Port field should default to TLS if it's available.
				Port:  wantXDSPorts.TLS,
				Ports: wantXDSPorts,
			}
			if wantXDSPorts.TLS <= 0 {
				resp.XDS.Port = wantXDSPorts.Plaintext
			}
		}

		selfJSON, err := json.Marshal(resp)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write(selfJSON)
	}
}

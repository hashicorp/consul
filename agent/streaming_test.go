// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func testGRPCStreamingWorking(t *testing.T, config string) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, config)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?index=3", nil)
	resp := httptest.NewRecorder()
	_, err := a.srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)
	require.NotEmpty(t, resp.Header().Get("X-Consul-Index"))
	require.Equal(t, "streaming", resp.Header().Get("X-Consul-Query-Backend"))
}

func TestGRPCWithTLSConfigs(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	t.Parallel()
	testCases := []struct {
		name   string
		config string
	}{
		{
			name:   "no-tls",
			config: "",
		},
		{
			name: "tls-all-enabled",
			config: `
				# tls
				ca_file   = "../test/hostname/CertAuth.crt"
				cert_file = "../test/hostname/Bob.crt"
				key_file  = "../test/hostname/Bob.key"
				verify_incoming               = true
				verify_outgoing               = true
				verify_server_hostname        = true
				`,
		},
		{
			name: "tls ready no verify incoming",
			config: `
				# tls
				ca_file   = "../test/hostname/CertAuth.crt"
				cert_file = "../test/hostname/Bob.crt"
				key_file  = "../test/hostname/Bob.key"
				verify_incoming               = false
				verify_outgoing               = true
				verify_server_hostname        = false
				`,
		},
		{
			name: "tls ready no verify outgoing and incoming",
			config: `
				# tls
				ca_file   = "../test/hostname/CertAuth.crt"
				cert_file = "../test/hostname/Bob.crt"
				key_file  = "../test/hostname/Bob.key"
				verify_incoming               = false
				verify_outgoing               = false
				verify_server_hostname        = false
				`,
		},
		{
			name: "tls ready, all defaults",
			config: `
				# tls
				ca_file   = "../test/hostname/CertAuth.crt"
				cert_file = "../test/hostname/Bob.crt"
				key_file  = "../test/hostname/Bob.key"
				`,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := testutil.TempDir(t, "agent") // we manage the data dir
			cfg := `data_dir = "` + dataDir + `"
					domain = "consul"
					node_name = "my-fancy-server"
					datacenter = "dc1"
					primary_datacenter = "dc1"
					rpc {
						enable_streaming = true
					}
					use_streaming_backend = true
			       ` + tt.config
			testGRPCStreamingWorking(t, cfg)
		})
	}
}

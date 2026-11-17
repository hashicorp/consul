// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

// TestHTTPWriteTimeout_BlockingQueryNotCutByGlobalTimeout verifies that a
// blocking query is bounded by its own wait duration rather than the global
// http_config.write_timeout. With a write_timeout shorter than the blocking
// wait, the per-request write deadline set in wrap() must keep the response
// stream open until the query returns valid JSON instead of resetting it.
//
// Before the per-request deadline, the global per-connection WriteTimeout would
// reset the in-flight blocking-query stream (EOF / INTERNAL_ERROR on HTTP/2)
// once write_timeout elapsed from connection start. See hashicorp/consul#23243.
func TestHTTPWriteTimeout_BlockingQueryNotCutByGlobalTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		http_config {
			write_timeout = "2s"
		}
		max_query_time = "30s"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	base := fmt.Sprintf("http://%s/v1/catalog/services", a.HTTPAddr())

	// Read the current index so the follow-up query actually blocks rather than
	// returning immediately.
	r0, err := http.Get(base)
	require.NoError(t, err)
	idx := r0.Header.Get("X-Consul-Index")
	r0.Body.Close()
	require.NotEmpty(t, idx)

	// Block on the current index for 4s, longer than the 2s write_timeout.
	start := time.Now()
	resp, err := http.Get(fmt.Sprintf("%s?index=%s&wait=4s", base, idx))
	require.NoError(t, err, "blocking query must not be reset by the 2s write_timeout")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var out map[string][]string
	require.NoError(t, json.Unmarshal(body, &out), "expected valid JSON, not a reset/EOF")
	require.GreaterOrEqual(t, time.Since(start), 2*time.Second,
		"query should have outlived the global write_timeout")
}

// TestHTTPWriteTimeout_HTTP2BlockingQueryNotCutByWriteTimeout covers the
// HTTP/2 path used by real clients over TLS. The HTTP/2 server applies
// http_config.write_timeout to every stream and resets it with
// RST_STREAM(INTERNAL_ERROR) when it fires, so without the per-request write
// deadline a blocking query whose wait exceeds write_timeout fails with
// "stream error: ... INTERNAL_ERROR; received from peer".
func TestHTTPWriteTimeout_HTTP2BlockingQueryNotCutByWriteTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := StartTestAgent(t, TestAgent{
		UseHTTPS: true,
		HCL: `
			http_config {
				write_timeout = "2s"
			}
			max_query_time = "30s"
			tls {
				defaults {
					ca_file   = "../test/client_certs/rootca.crt"
					cert_file = "../test/client_certs/server.crt"
					key_file  = "../test/client_certs/server.key"
				}
			}
		`,
	})
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	addr, err := firstAddr(a.apiServers, "https")
	require.NoError(t, err)
	base := fmt.Sprintf("https://%s/v1/catalog/services", addr)

	tlsccfg, err := api.SetupTLSConfig(&api.TLSConfig{
		Address:  "consul.test",
		KeyFile:  "../test/client_certs/client.key",
		CertFile: "../test/client_certs/client.crt",
		CAFile:   "../test/client_certs/rootca.crt",
	})
	require.NoError(t, err)
	transport := api.DefaultConfig().Transport
	transport.TLSHandshakeTimeout = 30 * time.Second
	transport.TLSClientConfig = tlsccfg
	var dials int32
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		atomic.AddInt32(&dials, 1)
		return dialer.DialContext(ctx, network, addr)
	}
	require.NoError(t, http2.ConfigureTransport(transport))
	client := &http.Client{Transport: transport}

	// Read the current index so the follow-up query actually blocks.
	r0, err := client.Get(base)
	require.NoError(t, err)
	require.Equal(t, 2, r0.ProtoMajor, "expected an HTTP/2 connection")
	idx := r0.Header.Get("X-Consul-Index")
	r0.Body.Close()
	require.NotEmpty(t, idx)

	// Block on the current index for 4s, longer than the 2s write_timeout.
	start := time.Now()
	resp, err := client.Get(fmt.Sprintf("%s?index=%s&wait=4s", base, idx))
	require.NoError(t, err, "HTTP/2 blocking query must not be reset by the 2s write_timeout")
	defer resp.Body.Close()
	require.Equal(t, 2, resp.ProtoMajor)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var out map[string][]string
	require.NoError(t, json.Unmarshal(body, &out), "expected valid JSON, not a stream reset")
	require.GreaterOrEqual(t, time.Since(start), 2*time.Second,
		"query should have outlived the global write_timeout")
	require.EqualValues(t, 1, atomic.LoadInt32(&dials), "both requests should share one HTTP/2 connection")
}

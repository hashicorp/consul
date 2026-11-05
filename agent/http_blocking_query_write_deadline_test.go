// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

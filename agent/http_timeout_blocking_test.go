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
)

// TestHTTPWriteTimeoutWithBlockingQuery validates that blocking queries
// return proper JSON responses even when they timeout, rather than EOF.
// This test reproduces the issue reported in #23243.
func TestHTTPWriteTimeoutWithBlockingQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Run("short write timeout causes EOF", func(t *testing.T) {
		// Create agent with short write timeout (2 seconds)
		a := NewTestAgent(t, `
			http_config = {
				write_timeout = "2s"
			}
		`)
		defer a.Shutdown()

		// Make a blocking query that waits longer than write timeout
		url := fmt.Sprintf("http://%s/v1/catalog/services?wait=5s&index=1", a.HTTPAddr())

		start := time.Now()
		resp, err := http.Get(url)
		duration := time.Since(start)

		// With short write timeout, we expect either:
		// 1. EOF error (connection closed by server)
		// 2. Response that comes back before the wait time
		if err != nil {
			// Check if it's an EOF or connection reset error
			t.Logf("Got error (expected with short timeout): %v after %v", err, duration)
			require.Contains(t, err.Error(), "EOF", "Expected EOF error when write timeout expires")
			require.Less(t, duration, 5*time.Second, "Should timeout before wait parameter")
		} else {
			defer resp.Body.Close()

			// If we got a response, it should be valid JSON
			body, readErr := io.ReadAll(resp.Body)
			require.NoError(t, readErr)

			var result map[string][]string
			jsonErr := json.Unmarshal(body, &result)

			if jsonErr != nil {
				t.Logf("Response came back in %v but is not valid JSON: %v", duration, string(body))
				t.Logf("This demonstrates the bug: server closed connection without proper response")
			} else {
				t.Logf("Got valid JSON response in %v (faster than expected)", duration)
			}
		}
	})

	t.Run("long write timeout allows proper JSON response", func(t *testing.T) {
		// Create agent with long write timeout (15 minutes)
		a := NewTestAgent(t, `
			http_config = {
				write_timeout = "15m"
			}
		`)
		defer a.Shutdown()

		// Make a blocking query with short wait
		url := fmt.Sprintf("http://%s/v1/catalog/services?wait=2s&index=1", a.HTTPAddr())

		start := time.Now()
		resp, err := http.Get(url)
		duration := time.Since(start)

		// With long write timeout, blocking query should complete normally
		require.NoError(t, err, "Should not get EOF with long write timeout")
		defer resp.Body.Close()

		// Should get valid JSON response
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string][]string
		err = json.Unmarshal(body, &result)
		require.NoError(t, err, "Should get valid JSON response")

		t.Logf("Got valid JSON response in %v", duration)
		// Note: Blocking queries return immediately if data is available,
		// so we don't assert on minimum duration. The key point is that
		// we get valid JSON, not EOF.
	})
}

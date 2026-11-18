// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ratelimit

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestHTTPConnectionLimit verifies limits.http_max_conns_per_client: an agent
// serves a client up to the limit, turns further connections away with 429
// while the ones it accepted keep working, and takes new connections again as
// soon as slots are freed.
func TestHTTPConnectionLimit(t *testing.T) {
	const limit = 12

	addr, _ := newServer(t, `limits { http_max_conns_per_client = 12 }`)

	// The harness's own client may hold a few idle connections, which count
	// towards the same limit, so the exact number accepted is only bounded.
	const harnessSlack = 6

	accepted, refused := utils.FillHTTPConns(t, addr, 3*limit)
	defer utils.CloseHTTPConns(accepted)

	require.Equal(t, http.StatusTooManyRequests, refused, "connections beyond the limit are refused")
	require.LessOrEqual(t, len(accepted), limit)
	require.GreaterOrEqual(t, len(accepted), limit-harnessSlack)

	t.Run("connections that were accepted keep working", func(t *testing.T) {
		for i, c := range accepted {
			require.Equal(t, http.StatusOK, c.Get(t, "/v1/status/leader"), "connection %d", i)
		}
	})

	t.Run("a freed slot can be used again", func(t *testing.T) {
		refusedConn, stillRefused := utils.OpenHTTPConn(t, addr)
		refusedConn.Close()
		require.Equal(t, http.StatusTooManyRequests, stillRefused)

		accepted[0].Close()
		retry.RunWith(&retry.Timer{Timeout: 15 * time.Second, Wait: 200 * time.Millisecond}, t, func(r *retry.R) {
			c, status := utils.OpenHTTPConn(t, addr)
			defer c.Close()
			require.Equal(r, http.StatusOK, status)
		})
	})
}

// TestConnectionLimitDefaults verifies the limits an agent runs with when none
// are configured.
func TestConnectionLimitDefaults(t *testing.T) {
	_, client := newServer(t, "")

	self, err := client.Agent().Self()
	require.NoError(t, err)

	debug := self["DebugConfig"]
	require.EqualValues(t, 200, debug["HTTPMaxConnsPerClient"])
	require.EqualValues(t, 100, debug["RPCMaxConnsPerClient"])
	require.EqualValues(t, 100, debug["GRPCMaxConnsPerClient"])
}

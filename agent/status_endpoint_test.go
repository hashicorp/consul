// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestStatusLeader(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/status/leader", nil)
	obj, err := a.srv.StatusLeader(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(string)
	if val == "" {
		t.Fatalf("bad addr: %v", obj)
	}
}

func TestStatusLeaderSecondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "datacenter = \"primary\"")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "datacenter = \"secondary\"")
	defer a2.Shutdown()

	testrpc.WaitForTestAgent(t, a1.RPC, "primary")
	testrpc.WaitForTestAgent(t, a2.RPC, "secondary")

	a1SerfAddr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	a1Addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.ServerPort)
	a2Addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.ServerPort)
	_, err := a2.JoinWAN([]string{a1SerfAddr})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
		require.Len(r, a2.WANMembers(), 2)
	})

	testrpc.WaitForLeader(t, a1.RPC, "secondary")
	testrpc.WaitForLeader(t, a2.RPC, "primary")

	req, _ := http.NewRequest("GET", "/v1/status/leader?dc=secondary", nil)
	obj, err := a1.srv.StatusLeader(nil, req)
	require.NoError(t, err)
	leader, ok := obj.(string)
	require.True(t, ok)
	require.Equal(t, a2Addr, leader)

	req, _ = http.NewRequest("GET", "/v1/status/leader?dc=primary", nil)
	obj, err = a2.srv.StatusLeader(nil, req)
	require.NoError(t, err)
	leader, ok = obj.(string)
	require.True(t, ok)
	require.Equal(t, a1Addr, leader)
}

func TestStatusPeers(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/status/peers", nil)
	obj, err := a.srv.StatusPeers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	peers := obj.([]string)
	if len(peers) != 1 {
		t.Fatalf("bad peers: %v", peers)
	}
}

func TestStatusPeersSecondary(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, "datacenter = \"primary\"")
	defer a1.Shutdown()
	a2 := NewTestAgent(t, "datacenter = \"secondary\"")
	defer a2.Shutdown()

	testrpc.WaitForTestAgent(t, a1.RPC, "primary")
	testrpc.WaitForTestAgent(t, a2.RPC, "secondary")

	a1SerfAddr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	a1Addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.ServerPort)
	a2Addr := fmt.Sprintf("127.0.0.1:%d", a2.Config.ServerPort)
	_, err := a2.JoinWAN([]string{a1SerfAddr})
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
		require.Len(r, a2.WANMembers(), 2)
	})

	testrpc.WaitForLeader(t, a1.RPC, "secondary")
	testrpc.WaitForLeader(t, a2.RPC, "primary")

	req, _ := http.NewRequest("GET", "/v1/status/peers?dc=secondary", nil)
	obj, err := a1.srv.StatusPeers(nil, req)
	require.NoError(t, err)
	peers, ok := obj.([]string)
	require.True(t, ok)
	require.Equal(t, []string{a2Addr}, peers)

	req, _ = http.NewRequest("GET", "/v1/status/peers?dc=primary", nil)
	obj, err = a2.srv.StatusPeers(nil, req)
	require.NoError(t, err)
	peers, ok = obj.([]string)
	require.True(t, ok)
	require.Equal(t, []string{a1Addr}, peers)
}

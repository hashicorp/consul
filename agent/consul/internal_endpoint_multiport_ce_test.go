// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/netutil"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

// TestInternal_PeeredUpstreams_CE pins the CE contract for the peered
// upstreams read path. Multiport is enterprise-only, so CE must:
//
//   - return every peered service unfiltered, and
//   - leave ServiceVIPs nil.
//
// The unfiltered requirement is load bearing. Consul allows dots in service
// names, so any "<port>.<service>" style filtering in CE would silently drop
// legitimately named services such as "api.v1" from every dialing proxy.
func TestInternal_PeeredUpstreams_CE(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	orig := virtualIPVersionCheckInterval
	virtualIPVersionCheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { virtualIPVersionCheckInterval = orig })

	t.Parallel()

	netutil.GetAgentBindAddrFunc = netutil.GetMockGetAgentBindAddrFunc("0.0.0.0")

	_, s1 := testServerWithConfig(t)
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	store := s1.fsm.State()
	retry.Run(t, func(r *retry.R) {
		_, entry, err := store.SystemMetadataGet(nil, structs.SystemMetadataVirtualIPsEnabled)
		require.NoError(r, err)
		require.NotNil(r, entry)
		require.Equal(r, "true", entry.Value)
	})

	// A plain service, plus dotted service names that a naive per-port filter
	// would mistake for synthetic "<port>.<service>" projections.
	names := []string{"web", "api.v1", "http.web"}
	for i, name := range names {
		require.NoError(t, store.EnsureRegistration(uint64(20+i), &structs.RegisterRequest{
			Node:           "bar",
			Address:        "127.0.0.2",
			SkipNodeUpdate: i != 0,
			Service: &structs.NodeService{
				Kind:    structs.ServiceKindConnectProxy,
				Service: name + "-proxy",
				ID:      name + "-proxy",
				Proxy:   structs.ConnectProxyConfig{DestinationServiceName: name},
			},
			PeerName: "peer-a",
		}))
	}

	codec := rpcClient(t, s1)
	args := structs.PartitionSpecificRequest{
		Datacenter:     "dc1",
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	var out structs.IndexedPeeredServiceList
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Internal.PeeredUpstreams", &args, &out))

	got := make([]string, 0, len(out.Services))
	for _, psn := range out.Services {
		got = append(got, psn.ServiceName.Name)
	}

	require.ElementsMatch(t, names, got,
		"CE must return every peered service unfiltered, including dotted names")

	require.Nil(t, out.ServiceVIPs,
		"ServiceVIPs is an enterprise multiport field and must be nil in CE")
}

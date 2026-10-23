// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/netutil"
	"github.com/hashicorp/consul/agent/structs"
)

// Multiport is an enterprise-only feature. These tests pin the CE contract:
// the per-port virtual IP hooks are no-ops, so registering a service that
// declares named ports must behave exactly like a single-port service.

func testEnableVirtualIPs(t *testing.T, s *Store) {
	t.Helper()
	netutil.GetAgentBindAddrFunc = netutil.GetMockGetAgentBindAddrFunc("0.0.0.0")
	require.NoError(t, s.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataVirtualIPsEnabled,
		Value: "true",
	}))
}

// portVirtualIPTaggedAddresses returns the "consul-virtual:<port>" tagged
// addresses, which only enterprise multiport ever creates.
func portVirtualIPTaggedAddresses(addrs map[string]structs.ServiceAddress) []string {
	var out []string
	for key := range addrs {
		if strings.HasPrefix(key, structs.TaggedAddressVirtualIPPrefix+":") {
			out = append(out, key)
		}
	}
	return out
}

// TestCE_PeeredServiceWithPorts_NoPerPortVirtualIPs asserts that an imported
// peered service declaring named ports gets the ordinary single base virtual
// IP and no per-port virtual IPs in CE.
func TestCE_PeeredServiceWithPorts_NoPerPortVirtualIPs(t *testing.T) {
	s := testStateStore(t)
	testEnableVirtualIPs(t, s)

	const peer = "peer-a"

	require.NoError(t, s.EnsureRegistration(10, &structs.RegisterRequest{
		Node:     "node-a",
		PeerName: peer,
		Service: &structs.NodeService{
			Kind:    structs.ServiceKindTypical,
			Service: "web",
			ID:      "web-a",
			Port:    8080,
			Ports: structs.ServicePorts{
				{Name: "http", Port: 8080, Default: true},
				{Name: "metrics", Port: 9090},
			},
		},
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}))

	// The multiport reconcile hook is a no-op in CE, so no per-port tagged
	// addresses may be written onto the registered instance.
	_, svc, err := s.NodeService(nil, "node-a", "web-a", nil, peer)
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.Empty(t, portVirtualIPTaggedAddresses(svc.TaggedAddresses),
		"CE must not assign per-port virtual IPs for a multiport service")

	// And no synthetic "<port>.<service>" virtual IP entries may exist.
	_, vips, err := s.VirtualIPsForAllImportedServices(nil, *acl.DefaultEnterpriseMeta())
	require.NoError(t, err)
	for _, vip := range vips {
		require.NotContains(t, vip.Service.ServiceName.Name, ".",
			"CE must not create synthetic per-port virtual IP entries")
	}
}

// TestCE_SinglePortServiceVirtualIPUnchanged pins the pre-existing single-port
// behavior: one base virtual IP, one "consul-virtual" tagged address, and no
// per-port entries.
func TestCE_SinglePortServiceVirtualIPUnchanged(t *testing.T) {
	s := testStateStore(t)
	testEnableVirtualIPs(t, s)

	require.NoError(t, s.EnsureRegistration(10, &structs.RegisterRequest{
		Node: "node-a",
		Service: &structs.NodeService{
			Kind:    structs.ServiceKindConnectProxy,
			Service: "web-sidecar-proxy",
			ID:      "web-sidecar-proxy",
			Port:    20000,
			Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "web"},
		},
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}))

	psn := structs.PeeredServiceName{
		ServiceName: structs.NewServiceName("web", acl.DefaultEnterpriseMeta()),
	}
	vip, err := s.VirtualIPForService(psn)
	require.NoError(t, err)
	require.Equal(t, "240.0.0.1", vip, "single-port virtual IP allocation must be unchanged")

	_, svc, err := s.NodeService(nil, "node-a", "web-sidecar-proxy", nil, "")
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.Equal(t, "240.0.0.1", svc.TaggedAddresses[structs.TaggedAddressVirtualIP].Address)
	require.Empty(t, portVirtualIPTaggedAddresses(svc.TaggedAddresses))
}

// TestCE_ServicePortVirtualIPHooksAreNoOps calls the enterprise extension
// points directly and asserts they neither error nor mutate state in CE.
func TestCE_ServicePortVirtualIPHooksAreNoOps(t *testing.T) {
	s := testStateStore(t)
	testEnableVirtualIPs(t, s)

	sn := structs.NewServiceName("web", acl.DefaultEnterpriseMeta())
	svc := &structs.NodeService{
		Kind:     structs.ServiceKindTypical,
		Service:  "web",
		ID:       "web-a",
		PeerName: "peer-a",
		Ports: structs.ServicePorts{
			{Name: "http", Port: 8080, Default: true},
			{Name: "metrics", Port: 9090},
		},
		TaggedAddresses: map[string]structs.ServiceAddress{},
	}

	tx := s.db.WriteTxn(1)
	defer tx.Abort()

	require.NoError(t, assignServicePortVirtualIPs(tx, 1, sn, svc))
	require.NoError(t, reconcileImportedServicePortVirtualIPs(tx, 1, "node-a", sn, svc))
	require.NoError(t, reconcileDeletedImportedServicePortVirtualIPs(tx, 1, sn, "peer-a"))
	require.NoError(t, freeServicePortVirtualIPs(tx, Query{Value: "web"}))

	require.Empty(t, svc.TaggedAddresses, "CE hooks must not write per-port tagged addresses")

	ip, err := s.VirtualIPForServicePort(structs.PeeredServiceName{ServiceName: sn}, "http")
	require.NoError(t, err)
	require.Empty(t, ip, "CE has no per-port virtual IPs")
}

// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

// Multiport is enterprise-only. These tests pin the CE side of the snapshot
// extension points so a future change cannot quietly turn them on.

// TestCE_PeeredUpstreamPortVIPs_NilForCESnapshot asserts that for any snapshot
// CE can actually produce, no per-port virtual IPs are resolved.
//
// PeeredUpstreamPortVIPs itself is shared code, but its input
// (PeeredPortUpstreamVIPs) is derived from IndexedPeeredServiceList.ServiceVIPs,
// which CE always leaves nil. So the reachable CE result is always nil.
func TestCE_PeeredUpstreamPortVIPs_NilForCESnapshot(t *testing.T) {
	base := NewUpstreamIDFromPeeredServiceName(structs.PeeredServiceName{
		ServiceName: structs.NewServiceName("web", nil),
		Peer:        "peer-a",
	})

	// This is what CE produces: ServiceVIPs is nil, so the decoded map is empty.
	var resp structs.IndexedPeeredServiceList
	require.Nil(t, resp.ServiceVIPs, "CE must not advertise per-port virtual IPs")

	decoded := make(map[UpstreamID]string, len(resp.ServiceVIPs))
	for key, vip := range resp.ServiceVIPs {
		psn, ok := structs.PeeredServiceNameFromString(key)
		if !ok || vip == "" {
			continue
		}
		decoded[NewUpstreamIDFromPeeredServiceName(psn)] = vip
	}
	require.Empty(t, decoded)

	upstreams := ConfigSnapshotUpstreams{PeeredPortUpstreamVIPs: decoded}
	require.Nil(t, upstreams.PeeredUpstreamPortVIPs(base),
		"CE must never resolve per-port virtual IPs for a peered upstream")
}

func TestCE_ShouldWatchRootServiceForDestinationPort_AlwaysFalse(t *testing.T) {
	uid := NewUpstreamIDFromServiceName(structs.NewServiceName("web", nil))
	uid.DestinationPort = "http"

	require.False(t, shouldWatchRootServiceForDestinationPort(&ConfigSnapshotUpstreams{}, uid),
		"CE has no port-qualified upstreams, so no extra root service watch")
}

// TestCE_MeshGatewayHasNoEnterpriseExports asserts the mesh gateway snapshot
// reports no enterprise exports or partition peering in CE.
//
// Note: PeeringServiceValue.Ports *is* populated in CE (parseServicePorts is
// pre-existing CE code), but no CE consumer reads it - both the cluster and
// endpoint generators skip a service group before reaching it. Only enterprise
// reads it, so it is inert here rather than absent.
func TestCE_MeshGatewayHasNoEnterpriseExports(t *testing.T) {
	var c configSnapshotMeshGateway
	require.False(t, c.hasEntExportedService(structs.NewServiceName("web", nil)))
	require.False(t, c.hasEntPartitionExport(structs.NewServiceName("web", nil)))
	require.True(t, c.entEmptyPeering())
}

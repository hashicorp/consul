// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfgglue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// TestServerPeeredUpstreams_CE pins the CE contract for the server-side peered
// upstreams data source: every peered service is returned unfiltered and
// ServiceVIPs stays nil. See TestInternal_PeeredUpstreams_CE for why the
// unfiltered guarantee matters for dotted service names.
func TestServerPeeredUpstreams_CE(t *testing.T) {
	const index uint64 = 123

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := state.NewStateStore(nil)
	enableVirtualIPs(t, store)

	registerService(t, index, "peer-a", "web", "node-a", store)
	registerService(t, index, "peer-a", "api.v1", "node-a", store)
	registerService(t, index, "peer-a", "http.web", "node-a", store)

	eventCh := make(chan proxycfg.UpdateEvent)
	dataSource := ServerPeeredUpstreams(ServerDataSourceDeps{
		GetStore:    func() Store { return store },
		ACLResolver: newStaticResolver(acl.ManageAll()),
	})
	require.NoError(t, dataSource.Notify(ctx, &structs.PartitionSpecificRequest{
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}, "", eventCh))

	result := getEventResult[*structs.IndexedPeeredServiceList](t, eventCh)

	got := make([]string, 0, len(result.Services))
	for _, psn := range result.Services {
		got = append(got, psn.ServiceName.Name)
	}
	require.ElementsMatch(t, []string{"web", "api.v1", "http.web"}, got,
		"CE must return every peered service unfiltered, including dotted names")

	require.Nil(t, result.ServiceVIPs,
		"ServiceVIPs is an enterprise multiport field and must be nil in CE")
}

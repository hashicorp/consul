// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestV1ServiceExportsShim_Integration(t *testing.T) {
	t.Parallel()
	_, srv := testServerDC(t, "dc1")

	shim := NewExportedServicesShim(srv)
	testrpc.WaitForLeader(t, srv.RPC, "dc1")

	v1ServiceExportsShimTests(t, shim, []*structs.ExportedServicesConfigEntry{
		{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "foo",
					Consumers: []structs.ServiceConsumer{
						{Peer: "cluster-01"},
					},
				},
			},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 0,
				ModifyIndex: 1,
			},
		},
		{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "bar",
					Consumers: []structs.ServiceConsumer{
						{Peer: "cluster-01"},
					},
				},
			},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 0,
				ModifyIndex: 2,
			},
		},
	})
}

func v1ServiceExportsShimTests(t *testing.T, shim *v1ServiceExportsShim, configs []*structs.ExportedServicesConfigEntry) {
	ctx := context.Background()

	go shim.Start(context.Background())

	partitions := make(map[string]*acl.EnterpriseMeta)
	for _, config := range configs {
		partitions[config.PartitionOrDefault()] = config.GetEnterpriseMeta()
	}

	for _, entMeta := range partitions {
		exportedServices, err := shim.GetExportedServicesConfigEntry(ctx, entMeta.PartitionOrDefault(), entMeta)
		require.Nil(t, err)
		require.Nil(t, exportedServices)
	}

	for _, config := range configs {
		err := shim.WriteExportedServicesConfigEntry(ctx, config)
		require.NoError(t, err)
		shim.assertPartitionEvent(t, config.PartitionOrDefault())
	}

	for _, entMeta := range partitions {
		err := shim.DeleteExportedServicesConfigEntry(ctx, entMeta.PartitionOrDefault(), entMeta)
		require.NoError(t, err)
		shim.assertPartitionEvent(t, entMeta.PartitionOrDefault())
	}
}

func (s *v1ServiceExportsShim) assertPartitionEvent(t *testing.T, partition string) {
	t.Helper()

	select {
	case event := <-s.eventCh:
		require.Equal(t, partition, event.Obj.Key())
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timeout waiting for view to receive events")
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/gossip/libserf"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	"github.com/hashicorp/consul/testrpc"
)

func updateSerfTags(s *Server, key, value string) {
	libserf.UpdateTag(s.serfLAN, key, value)

	if s.serfWAN != nil {
		libserf.UpdateTag(s.serfWAN, key, value)
	}
}

func TestServer_InitTenancy(t *testing.T) {
	t.Parallel()

	_, conf := testServerConfig(t)
	deps := newDefaultDeps(t, conf)
	deps.Experiments = []string{"v2tenancy"}
	deps.Registry = NewTypeRegistry()

	s, err := newServerWithDeps(t, conf, deps)
	require.NoError(t, err)

	// first initTenancy call happens here
	waitForLeaderEstablishment(t, s)
	testrpc.WaitForLeader(t, s.RPC, "dc1")

	nsID := &pbresource.ID{
		Type:    pbtenancy.NamespaceType,
		Tenancy: resource.DefaultPartitionedTenancy(),
		Name:    resource.DefaultNamespaceName,
	}

	ns, err := s.storageBackend.Read(context.Background(), storage.StrongConsistency, nsID)
	require.NoError(t, err)
	require.Equal(t, resource.DefaultNamespaceName, ns.Id.Name)

	// explicitly call initiTenancy to verify we do not re-create namespace
	err = s.initTenancy(context.Background(), s.storageBackend)
	require.NoError(t, err)

	// read again
	actual, err := s.storageBackend.Read(context.Background(), storage.StrongConsistency, nsID)
	require.NoError(t, err)

	require.Equal(t, ns.Id.Uid, actual.Id.Uid)
	require.Equal(t, ns.Generation, actual.Generation)
	require.Equal(t, ns.Version, actual.Version)
}

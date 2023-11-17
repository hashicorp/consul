// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
	"github.com/hashicorp/consul/testrpc"
)

func TestAgent_ReloadConfig_Reporting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir1, s := testServerWithConfig(t, func(c *Config) {
		c.Reporting.License.Enabled = false
	})
	defer os.RemoveAll(dir1)
	defer s.Shutdown()

	testrpc.WaitForTestAgent(t, s.RPC, "dc1")

	require.Equal(t, false, s.config.Reporting.License.Enabled)

	rc := ReloadableConfig{
		Reporting: Reporting{
			License: License{
				Enabled: true,
			},
		},
	}

	require.NoError(t, s.ReloadConfig(rc))

	// Check config reload is no-op
	require.Equal(t, false, s.config.Reporting.License.Enabled)
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

	ns, err := s.resourceServiceServer.Backend.Read(context.Background(), storage.StrongConsistency, nsID)
	require.NoError(t, err)
	require.Equal(t, resource.DefaultNamespaceName, ns.Id.Name)

	// explicitly call initiTenancy to verify we do not re-create namespace
	err = s.initTenancy(context.Background(), s.resourceServiceServer.Backend)
	require.NoError(t, err)

	// read again
	actual, err := s.resourceServiceServer.Backend.Read(context.Background(), storage.StrongConsistency, nsID)
	require.NoError(t, err)

	require.Equal(t, ns.Id.Uid, actual.Id.Uid)
	require.Equal(t, ns.Generation, actual.Generation)
	require.Equal(t, ns.Version, actual.Version)
}

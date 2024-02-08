// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"

	"github.com/hashicorp/consul/internal/catalog/catalogtest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
)

var (
	cli = rtest.ConfigureTestCLIFlags()
)

func TestCatalog(t *testing.T) {
	t.Parallel()

	cluster, _, _ := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 3,
		BuildOpts:  &libcluster.BuildOptions{Datacenter: "dc1"},
		Cmd:        `-hcl=experiments=["resource-apis"]`,
	})

	followers, err := cluster.Followers()
	require.NoError(t, err)
	client := pbresource.NewResourceServiceClient(followers[0].GetGRPCConn())

	t.Run("one-shot", func(t *testing.T) {
		catalogtest.RunCatalogV2Beta1IntegrationTest(t, client, cli.ClientOptions(t)...)
	})

	t.Run("lifecycle", func(t *testing.T) {
		catalogtest.RunCatalogV2Beta1LifecycleIntegrationTest(t, client, cli.ClientOptions(t)...)
	})
}

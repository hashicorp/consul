// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1
package resource_http

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

var commonGVK = api.GVK{
	Group:   "demo",
	Version: "v2",
	Kind:    "Artist",
}
var commonQueryOptions = api.QueryOptions{
	Namespace: "default",
	Partition: "default",
	Peer:      "local",
}
var commonPayload = api.WriteRequest{
	Metadata: map[string]string{
		"foo": "bar",
	},
	Data: map[string]any{
		"name": "cool",
	},
}

func SetupClusterAndClient(t *testing.T, config *libtopology.ClusterConfig, isServer bool) (*libcluster.Cluster, *api.Client) {
	cluster, _, _ := libtopology.NewCluster(t, config)

	client, err := cluster.GetClient(nil, isServer)
	require.NoError(t, err)

	return cluster, client
}

func Terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

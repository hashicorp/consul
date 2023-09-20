// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

type config struct {
	gvk          api.GVK
	resourceName string
	queryOptions api.QueryOptions
	payload      api.WriteRequest
}
type operation struct {
	action           func(client *api.Client, config config) error
	expectedErrorMsg string
	includeToken     bool
}
type testCase struct {
	description string
	operations  []operation
	config      []config
}

var demoGVK = api.GVK{
	Group:   "demo",
	Version: "v2",
	Kind:    "Artist",
}

var defaultTenancyQueryOptions = api.QueryOptions{
	Namespace: "default",
	Partition: "default",
	Peer:      "local",
}

var fakeTenancyQueryOptions = api.QueryOptions{
	Namespace: "fake-default",
	Partition: "fake-default",
	Peer:      "fake-local",
}

var demoPayload = api.WriteRequest{
	Metadata: map[string]string{
		"foo": "bar",
	},
	Data: map[string]any{
		"name": "cool",
	},
}

var applyResource = func(client *api.Client, config config) error {
	_, _, err := client.Resource().Apply(&config.gvk, config.resourceName, &config.queryOptions, &config.payload)
	return err
}
var readResource = func(client *api.Client, config config) error {
	_, err := client.Resource().Read(&config.gvk, config.resourceName, &config.queryOptions)
	return err
}
var deleteResource = func(client *api.Client, config config) error {
	err := client.Resource().Delete(&config.gvk, config.resourceName, &config.queryOptions)
	return err
}
var listResource = func(client *api.Client, config config) error {
	_, err := client.Resource().List(&config.gvk, &config.queryOptions)
	return err
}

func makeClusterConfig(numOfServers int, numOfClients int, aclEnabled bool) *libtopology.ClusterConfig {
	return &libtopology.ClusterConfig{
		NumServers:  numOfServers,
		NumClients:  numOfClients,
		LogConsumer: &libtopology.TestLogConsumer{},
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			ACLEnabled:             aclEnabled,
		},
		ApplyDefaultProxySettings: false,
	}
}

func SetupClusterAndClient(t *testing.T, clusterConfig *libtopology.ClusterConfig, isServer bool) (*libcluster.Cluster, *api.Client) {
	cluster, _, _ := libtopology.NewCluster(t, clusterConfig)

	client, err := cluster.GetClient(nil, isServer)
	require.NoError(t, err)

	return cluster, client
}

func Terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

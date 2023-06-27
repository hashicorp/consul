// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-version"

	"github.com/hashicorp/consul/internal/catalog/catalogtest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

var minCatalogResourceVersion = version.Must(version.NewVersion("v1.16.0"))

const (
	versionUndetermined = `
Cannot determine the actual version the starting image represents.
Scrutinze test failures to ensure that the starting version should
actually be able to be used for creating the initial data set.
	`
)

func maybeSkipUpgradeTest(t *testing.T, minVersion *version.Version) {
	t.Helper()

	image := utils.DockerImage(utils.GetLatestImageName(), utils.LatestVersion)
	latestVersion, err := utils.DockerImageVersion(image)

	if latestVersion != nil && latestVersion.LessThan(minVersion) {
		t.Skipf("Upgrade test isn't applicable with version %q as the starting version", latestVersion.String())
	}

	if err != nil || latestVersion == nil {
		t.Log(versionUndetermined)
	}
}

// Test upgrade a cluster of latest version to the target version and ensure that the catalog still
// functions properly. Note
func TestCatalogUpgrade(t *testing.T) {
	maybeSkipUpgradeTest(t, minCatalogResourceVersion)
	t.Parallel()

	const numServers = 1
	buildOpts := &libcluster.BuildOptions{
		ConsulImageName:      utils.GetLatestImageName(),
		ConsulVersion:        utils.LatestVersion,
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	}

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		BuildOpts:                 buildOpts,
		ApplyDefaultProxySettings: true,
		Cmd:                       `-hcl=experiments=["resource-apis"]`,
	})

	client := cluster.APIClient(0)

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, numServers)

	leader, err := cluster.Leader()
	require.NoError(t, err)
	rscClient := pbresource.NewResourceServiceClient(leader.GetGRPCConn())

	// Initialize some data
	catalogtest.PublishCatalogV1Alpha1IntegrationTestData(t, rscClient)

	// upgrade the cluster to the Target version
	t.Logf("initiating standard upgrade to version=%q", utils.TargetVersion)
	err = cluster.StandardUpgrade(t, context.Background(), utils.GetTargetImageName(), utils.TargetVersion)

	require.NoError(t, err)
	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, numServers)

	catalogtest.VerifyCatalogV1Alpha1IntegrationTestResults(t, rscClient)
}

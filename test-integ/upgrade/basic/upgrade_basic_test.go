// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package upgrade

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
)

// Test_Upgrade_Standard_Basic_Agentless tests upgrading the agent servers
// of a class and validate service mesh after upgrade
//
// Refer to common.go for the detail of the topology
func Test_Upgrade_Standard_Basic_Agentless(t *testing.T) {
	t.Parallel()

	ct := NewCommonTopo(t)
	ct.Launch(t)

	t.Log("Start standard upgrade ...")
	sp := ct.Sprawl
	cfg := sp.Config()
	require.NoError(t, ct.Sprawl.LoadKVDataToCluster("dc1", 1, &api.WriteOptions{}))
	require.NoError(t, sp.Upgrade(cfg, "dc1", sprawl.UpgradeTypeStandard, utils.TargetImages(), nil, nil))
	t.Log("Finished standard upgrade ...")

	// verify data is not lost
	data, err := ct.Sprawl.GetKV("dc1", "key-0", &api.QueryOptions{})
	require.NoError(t, err)
	require.NotNil(t, data)

	ct.PostUpgradeValidation(t)
}

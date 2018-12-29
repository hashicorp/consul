// +build acceptance clustering policies

package v1

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/nodes"
	th "github.com/gophercloud/gophercloud/testhelper"
)

func TestNodesCRUD(t *testing.T) {
	client, err := clients.NewClusteringV1Client()
	th.AssertNoErr(t, err)

	profile, err := CreateProfile(t, client)
	th.AssertNoErr(t, err)
	defer DeleteProfile(t, client, profile.ID)

	cluster, err := CreateCluster(t, client, profile.ID)
	th.AssertNoErr(t, err)
	defer DeleteCluster(t, client, cluster.ID)

	node, err := CreateNode(t, client, cluster.ID, profile.ID)
	th.AssertNoErr(t, err)
	defer DeleteNode(t, client, node.ID)

	// Test nodes list
	allPages, err := nodes.List(client, nil).AllPages()
	th.AssertNoErr(t, err)

	allNodes, err := nodes.ExtractNodes(allPages)
	th.AssertNoErr(t, err)

	var found bool
	for _, v := range allNodes {
		if v.ID == node.ID {
			found = true
		}
	}

	th.AssertEquals(t, found, true)

	// Test nodes update
	t.Logf("Attempting to update node %s", node.ID)

	updateOpts := nodes.UpdateOpts{
		Metadata: map[string]interface{}{
			"bar": "baz",
		},
	}

	res := nodes.Update(client, node.ID, updateOpts)
	th.AssertNoErr(t, res.Err)

	actionID, err := GetActionID(res.Header)
	th.AssertNoErr(t, err)

	err = WaitForAction(client, actionID)
	th.AssertNoErr(t, err)

	node, err = nodes.Get(client, node.ID).Extract()
	th.AssertNoErr(t, err)

	tools.PrintResource(t, node)
	tools.PrintResource(t, node.Metadata)
}

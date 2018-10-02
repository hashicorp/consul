// +build acceptance blockstorage

package v3

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	th "github.com/gophercloud/gophercloud/testhelper"
)

func TestVolumeTypes(t *testing.T) {
	clients.RequireAdmin(t)

	client, err := clients.NewBlockStorageV3Client()
	th.AssertNoErr(t, err)

	vt, err := CreateVolumeType(t, client)
	th.AssertNoErr(t, err)
	defer DeleteVolumeType(t, client, vt)

	allPages, err := volumetypes.List(client, nil).AllPages()
	th.AssertNoErr(t, err)

	allVolumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)
	th.AssertNoErr(t, err)

	var found bool
	for _, v := range allVolumeTypes {
		tools.PrintResource(t, v)
		if v.ID == vt.ID {
			found = true
		}
	}

	th.AssertEquals(t, found, true)

	var isPublic = false
	updateOpts := volumetypes.UpdateOpts{
		Name:     vt.Name + "-UPDATED",
		IsPublic: &isPublic,
	}

	newVT, err := volumetypes.Update(client, vt.ID, updateOpts).Extract()
	th.AssertNoErr(t, err)
	th.AssertEquals(t, vt.Name+"-UPDATED", newVT.Name)
	th.AssertEquals(t, false, newVT.IsPublic)

	tools.PrintResource(t, newVT)
}

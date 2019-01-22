// +build acceptance blockstorage

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/volumeactions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v2/volumes"
	th "github.com/gophercloud/gophercloud/testhelper"
)

func TestVolumesCreateDestroy(t *testing.T) {
	clients.RequireLong(t)

	client, err := clients.NewBlockStorageV2Client()
	th.AssertNoErr(t, err)

	volume, err := CreateVolume(t, client)
	th.AssertNoErr(t, err)
	defer DeleteVolume(t, client, volume)

	newVolume, err := volumes.Get(client, volume.ID).Extract()
	th.AssertNoErr(t, err)

	allPages, err := volumes.List(client, volumes.ListOpts{}).AllPages()
	th.AssertNoErr(t, err)

	allVolumes, err := volumes.ExtractVolumes(allPages)
	th.AssertNoErr(t, err)

	var found bool
	for _, v := range allVolumes {
		tools.PrintResource(t, volume)
		if v.ID == newVolume.ID {
			found = true
		}
	}

	th.AssertEquals(t, found, true)
}

func TestVolumesCreateForceDestroy(t *testing.T) {
	clients.RequireLong(t)

	client, err := clients.NewBlockStorageV2Client()
	th.AssertNoErr(t, err)

	volume, err := CreateVolume(t, client)
	th.AssertNoErr(t, err)

	newVolume, err := volumes.Get(client, volume.ID).Extract()
	th.AssertNoErr(t, err)

	tools.PrintResource(t, newVolume)

	err = volumeactions.ForceDelete(client, newVolume.ID).ExtractErr()
	th.AssertNoErr(t, err)
}

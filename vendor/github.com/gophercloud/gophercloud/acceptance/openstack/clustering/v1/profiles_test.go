// +build acceptance clustering policies

package v1

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/clustering/v1/profiles"
	th "github.com/gophercloud/gophercloud/testhelper"
)

func TestProfilesCRUD(t *testing.T) {
	client, err := clients.NewClusteringV1Client()
	th.AssertNoErr(t, err)

	profile, err := CreateProfile(t, client)
	th.AssertNoErr(t, err)
	defer DeleteProfile(t, client, profile.ID)

	// Test listing profiles
	allPages, err := profiles.List(client, nil).AllPages()
	th.AssertNoErr(t, err)

	allProfiles, err := profiles.ExtractProfiles(allPages)
	th.AssertNoErr(t, err)

	var found bool
	for _, v := range allProfiles {
		if v.ID == profile.ID {
			found = true
		}
	}

	th.AssertEquals(t, found, true)

	// Test updating profile
	updateOpts := profiles.UpdateOpts{
		Name: profile.Name + "-UPDATED",
	}

	newProfile, err := profiles.Update(client, profile.ID, updateOpts).Extract()
	th.AssertNoErr(t, err)
	th.AssertEquals(t, newProfile.Name, profile.Name+"-UPDATED")

	tools.PrintResource(t, newProfile)
	tools.PrintResource(t, newProfile.UpdatedAt)
}

package testing

import (
	"testing"

	"github.com/gophercloud/gophercloud/openstack/clustering/v1/profiles"
	"github.com/gophercloud/gophercloud/pagination"
	th "github.com/gophercloud/gophercloud/testhelper"
	fake "github.com/gophercloud/gophercloud/testhelper/client"
)

func TestCreateProfile(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleCreateSuccessfully(t)

	networks := []map[string]interface{}{
		{"network": "private-network"},
	}

	props := map[string]interface{}{
		"name":            "test_gopher_cloud_profile",
		"flavor":          "t2.small",
		"image":           "centos7.3-latest",
		"networks":        networks,
		"security_groups": "",
	}

	createOpts := &profiles.CreateOpts{
		Name: "TestProfile",
		Spec: profiles.Spec{
			Type:       "os.nova.server",
			Version:    "1.0",
			Properties: props,
		},
	}

	profile, err := profiles.Create(fake.ServiceClient(), createOpts).Extract()
	if err != nil {
		t.Errorf("Failed to extract profile: %v", err)
	}

	th.AssertDeepEquals(t, ExpectedCreate, *profile)
}

func TestGetProfile(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleGetSuccessfully(t)

	actual, err := profiles.Get(fake.ServiceClient(), ExpectedGet.ID).Extract()
	th.AssertNoErr(t, err)
	th.AssertDeepEquals(t, ExpectedGet, *actual)
}

func TestListProfiles(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleListSuccessfully(t)

	var iFalse bool
	listOpts := profiles.ListOpts{
		GlobalProject: &iFalse,
	}

	count := 0
	err := profiles.List(fake.ServiceClient(), listOpts).EachPage(func(page pagination.Page) (bool, error) {
		count++
		actual, err := profiles.ExtractProfiles(page)
		th.AssertNoErr(t, err)
		th.AssertDeepEquals(t, ExpectedList, actual)

		return true, nil
	})

	th.AssertNoErr(t, err)

	if count != 1 {
		t.Errorf("Expected 1 page of profiles, got %d pages instead", count)
	}
}

func TestUpdateProfile(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleUpdateSuccessfully(t)

	updateOpts := profiles.UpdateOpts{
		Name: "pserver",
	}

	actual, err := profiles.Update(fake.ServiceClient(), ExpectedUpdate.ID, updateOpts).Extract()
	th.AssertNoErr(t, err)
	th.AssertDeepEquals(t, ExpectedUpdate, *actual)
}

func TestDeleteProfile(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	HandleDeleteSuccessfully(t)

	deleteResult := profiles.Delete(fake.ServiceClient(), "6dc6d336e3fc4c0a951b5698cd1236ee")
	th.AssertNoErr(t, deleteResult.ExtractErr())
}

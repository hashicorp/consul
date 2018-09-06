package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/subnetpools"
)

// CreateSubnetPool will create a subnetpool. An error will be returned if the
// subnetpool could not be created.
func CreateSubnetPool(t *testing.T, client *gophercloud.ServiceClient) (*subnetpools.SubnetPool, error) {
	subnetPoolName := tools.RandomString("TESTACC-", 8)
	subnetPoolPrefixes := []string{
		"10.0.0.0/8",
	}
	createOpts := subnetpools.CreateOpts{
		Name:     subnetPoolName,
		Prefixes: subnetPoolPrefixes,
	}

	t.Logf("Attempting to create a subnetpool: %s", subnetPoolName)

	subnetPool, err := subnetpools.Create(client, createOpts).Extract()
	if err != nil {
		return nil, err
	}

	t.Logf("Successfully created the subnetpool.")
	return subnetPool, nil
}

// DeleteSubnetPool will delete a subnetpool with a specified ID.
// A fatal error will occur if the delete was not successful.
func DeleteSubnetPool(t *testing.T, client *gophercloud.ServiceClient, subnetPoolID string) {
	t.Logf("Attempting to delete the subnetpool: %s", subnetPoolID)

	err := subnetpools.Delete(client, subnetPoolID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete subnetpool %s: %v", subnetPoolID, err)
	}

	t.Logf("Deleted subnetpool: %s", subnetPoolID)
}

package v2

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/shares"
)

// CreateShare will create a share with a name, and a size of 1Gb. An
// error will be returned if the share could not be created
func CreateShare(t *testing.T, client *gophercloud.ServiceClient) (*shares.Share, error) {
	if testing.Short() {
		t.Skip("Skipping test that requres share creation in short mode.")
	}

	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		t.Fatalf("Unable to fetch environment information")
	}

	t.Logf("Share network id %s", choices.ShareNetworkID)
	createOpts := shares.CreateOpts{
		Size:           1,
		Name:           "My Test Share",
		ShareProto:     "NFS",
		ShareNetworkID: choices.ShareNetworkID,
	}

	share, err := shares.Create(client, createOpts).Extract()
	if err != nil {
		return share, err
	}

	err = waitForStatus(client, share.ID, "available", 600)
	if err != nil {
		return share, err
	}

	return share, nil
}

// ListShares lists all shares that belong to this tenant's project.
// An error will be returned if the shares could not be listed..
func ListShares(t *testing.T, client *gophercloud.ServiceClient) ([]shares.Share, error) {
	r, err := shares.ListDetail(client, &shares.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	return shares.ExtractShares(r)
}

// GrantAccess will grant access to an existing share. A fatal error will occur if
// this operation fails.
func GrantAccess(t *testing.T, client *gophercloud.ServiceClient, share *shares.Share) (*shares.AccessRight, error) {
	return shares.GrantAccess(client, share.ID, shares.GrantAccessOpts{
		AccessType:  "ip",
		AccessTo:    "0.0.0.0/32",
		AccessLevel: "r",
	}).Extract()
}

// RevokeAccess will revoke an exisiting access of a share. A fatal error will occur
// if this operation fails.
func RevokeAccess(t *testing.T, client *gophercloud.ServiceClient, share *shares.Share, accessRight *shares.AccessRight) error {
	return shares.RevokeAccess(client, share.ID, shares.RevokeAccessOpts{
		AccessID: accessRight.ID,
	}).ExtractErr()
}

// GetAccessRightsSlice will retrieve all access rules assigned to a share.
// A fatal error will occur if this operation fails.
func GetAccessRightsSlice(t *testing.T, client *gophercloud.ServiceClient, share *shares.Share) ([]shares.AccessRight, error) {
	return shares.ListAccessRights(client, share.ID).Extract()
}

// DeleteShare will delete a share. A fatal error will occur if the share
// failed to be deleted. This works best when used as a deferred function.
func DeleteShare(t *testing.T, client *gophercloud.ServiceClient, share *shares.Share) {
	err := shares.Delete(client, share.ID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete share %s: %v", share.ID, err)
	}

	t.Logf("Deleted share: %s", share.ID)
}

// PrintShare prints some information of the share
func PrintShare(t *testing.T, share *shares.Share) {
	asJSON, err := json.MarshalIndent(share, "", " ")
	if err != nil {
		t.Logf("Cannot print the contents of %s", share.ID)
	}

	t.Logf("Share %s", string(asJSON))
}

// PrintAccessRight prints contents of an access rule
func PrintAccessRight(t *testing.T, accessRight *shares.AccessRight) {
	asJSON, err := json.MarshalIndent(accessRight, "", " ")
	if err != nil {
		t.Logf("Cannot print access rule")
	}

	t.Logf("Access rule %s", string(asJSON))
}

// ExtendShare extends the capacity of an existing share
func ExtendShare(t *testing.T, client *gophercloud.ServiceClient, share *shares.Share, newSize int) error {
	return shares.Extend(client, share.ID, &shares.ExtendOpts{NewSize: newSize}).ExtractErr()
}

// ShrinkShare shrinks the capacity of an existing share
func ShrinkShare(t *testing.T, client *gophercloud.ServiceClient, share *shares.Share, newSize int) error {
	return shares.Shrink(client, share.ID, &shares.ShrinkOpts{NewSize: newSize}).ExtractErr()
}

func waitForStatus(c *gophercloud.ServiceClient, id, status string, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		current, err := shares.Get(c, id).Extract()
		if err != nil {
			return false, err
		}

		if current.Status == "error" {
			return true, fmt.Errorf("An error occurred")
		}

		if current.Status == status {
			return true, nil
		}

		return false, nil
	})
}

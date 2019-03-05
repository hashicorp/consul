// +build acceptance networking vpnaas

package vpnaas

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/vpnaas/ipsecpolicies"
)

func TestIPSecPolicyList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	allPages, err := ipsecpolicies.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list IPSec policies: %v", err)
	}

	allPolicies, err := ipsecpolicies.ExtractPolicies(allPages)
	if err != nil {
		t.Fatalf("Unable to extract policies: %v", err)
	}

	for _, policy := range allPolicies {
		tools.PrintResource(t, policy)
	}
}

func TestIPSecPolicyCRUD(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	policy, err := CreateIPSecPolicy(t, client)
	if err != nil {
		t.Fatalf("Unable to create IPSec policy: %v", err)
	}
	defer DeleteIPSecPolicy(t, client, policy.ID)
	tools.PrintResource(t, policy)

	updatedDescription := "Updated policy description"
	updateOpts := ipsecpolicies.UpdateOpts{
		Description: &updatedDescription,
	}

	policy, err = ipsecpolicies.Update(client, policy.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update IPSec policy: %v", err)
	}
	tools.PrintResource(t, policy)

	newPolicy, err := ipsecpolicies.Get(client, policy.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to get IPSec policy: %v", err)
	}
	tools.PrintResource(t, newPolicy)
}

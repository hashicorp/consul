// +build acceptance networking vpnaas

package vpnaas

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/vpnaas/ikepolicies"
)

func TestIKEPolicyList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	allPages, err := ikepolicies.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list IKE policies: %v", err)
	}

	allPolicies, err := ikepolicies.ExtractPolicies(allPages)
	if err != nil {
		t.Fatalf("Unable to extract IKE policies: %v", err)
	}

	for _, policy := range allPolicies {
		tools.PrintResource(t, policy)
	}
}

func TestIKEPolicyCRUD(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	policy, err := CreateIKEPolicy(t, client)
	if err != nil {
		t.Fatalf("Unable to create IKE policy: %v", err)
	}
	defer DeleteIKEPolicy(t, client, policy.ID)

	tools.PrintResource(t, policy)

	newPolicy, err := ikepolicies.Get(client, policy.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to get IKE policy: %v", err)
	}
	tools.PrintResource(t, newPolicy)

	updatedName := "updatedname"
	updatedDescription := "updated policy"
	updateOpts := ikepolicies.UpdateOpts{
		Name:        &updatedName,
		Description: &updatedDescription,
		Lifetime: &ikepolicies.LifetimeUpdateOpts{
			Value: 7000,
		},
	}
	updatedPolicy, err := ikepolicies.Update(client, policy.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update IKE policy: %v", err)
	}
	tools.PrintResource(t, updatedPolicy)

}

// +build acceptance networking vpnaas

package vpnaas

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/vpnaas/endpointgroups"
)

func TestGroupList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	allPages, err := endpointgroups.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list endpoint groups: %v", err)
	}

	allGroups, err := endpointgroups.ExtractEndpointGroups(allPages)
	if err != nil {
		t.Fatalf("Unable to extract endpoint groups: %v", err)
	}

	for _, group := range allGroups {
		tools.PrintResource(t, group)
	}
}

func TestGroupCRUD(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	group, err := CreateEndpointGroup(t, client)
	if err != nil {
		t.Fatalf("Unable to create Endpoint group: %v", err)
	}
	defer DeleteEndpointGroup(t, client, group.ID)
	tools.PrintResource(t, group)

	newGroup, err := endpointgroups.Get(client, group.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to retrieve Endpoint group: %v", err)
	}
	tools.PrintResource(t, newGroup)

	updatedName := "updatedname"
	updatedDescription := "updated description"
	updateOpts := endpointgroups.UpdateOpts{
		Name:        &updatedName,
		Description: &updatedDescription,
	}
	updatedGroup, err := endpointgroups.Update(client, group.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update endpoint group: %v", err)
	}
	tools.PrintResource(t, updatedGroup)

}

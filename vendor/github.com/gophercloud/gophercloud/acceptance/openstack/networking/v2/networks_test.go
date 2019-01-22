// +build acceptance networking

package v2

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/portsecurity"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	th "github.com/gophercloud/gophercloud/testhelper"
)

func TestNetworksList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	type networkWithExt struct {
		networks.Network
		portsecurity.PortSecurityExt
	}

	var allNetworks []networkWithExt

	allPages, err := networks.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list networks: %v", err)
	}

	err = networks.ExtractNetworksInto(allPages, &allNetworks)
	if err != nil {
		t.Fatalf("Unable to extract networks: %v", err)
	}

	for _, network := range allNetworks {
		tools.PrintResource(t, network)
	}
}

func TestNetworksExternalList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		t.Fatalf("Unable to fetch environment information: %s", err)
	}

	type networkWithExt struct {
		networks.Network
		external.NetworkExternalExt
	}

	var allNetworks []networkWithExt

	iTrue := true
	networkListOpts := networks.ListOpts{
		ID: choices.ExternalNetworkID,
	}
	listOpts := external.ListOptsExt{
		ListOptsBuilder: networkListOpts,
		External:        &iTrue,
	}

	allPages, err := networks.List(client, listOpts).AllPages()
	if err != nil {
		t.Fatalf("Unable to list networks: %v", err)
	}

	err = networks.ExtractNetworksInto(allPages, &allNetworks)
	if err != nil {
		t.Fatalf("Unable to extract networks: %v", err)
	}

	var found bool
	for _, network := range allNetworks {
		if network.External == true && network.ID == choices.ExternalNetworkID {
			found = true
		}
	}

	th.AssertEquals(t, found, true)

	iFalse := false
	networkListOpts = networks.ListOpts{
		ID: choices.ExternalNetworkID,
	}
	listOpts = external.ListOptsExt{
		ListOptsBuilder: networkListOpts,
		External:        &iFalse,
	}

	allPages, err = networks.List(client, listOpts).AllPages()
	if err != nil {
		t.Fatalf("Unable to list networks: %v", err)
	}

	v, err := networks.ExtractNetworks(allPages)
	th.AssertEquals(t, len(v), 0)
}

func TestNetworksCRUD(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	// Create a network
	network, err := CreateNetwork(t, client)
	if err != nil {
		t.Fatalf("Unable to create network: %v", err)
	}
	defer DeleteNetwork(t, client, network.ID)

	tools.PrintResource(t, network)

	newName := tools.RandomString("TESTACC-", 8)
	updateOpts := &networks.UpdateOpts{
		Name: newName,
	}

	_, err = networks.Update(client, network.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update network: %v", err)
	}

	newNetwork, err := networks.Get(client, network.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to retrieve network: %v", err)
	}

	tools.PrintResource(t, newNetwork)
}

func TestNetworksPortSecurityCRUD(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	// Create a network without port security
	network, err := CreateNetworkWithoutPortSecurity(t, client)
	if err != nil {
		t.Fatalf("Unable to create network: %v", err)
	}
	defer DeleteNetwork(t, client, network.ID)

	var networkWithExtensions struct {
		networks.Network
		portsecurity.PortSecurityExt
	}

	err = networks.Get(client, network.ID).ExtractInto(&networkWithExtensions)
	if err != nil {
		t.Fatalf("Unable to retrieve network: %v", err)
	}

	tools.PrintResource(t, networkWithExtensions)

	iTrue := true
	networkUpdateOpts := networks.UpdateOpts{}
	updateOpts := portsecurity.NetworkUpdateOptsExt{
		UpdateOptsBuilder:   networkUpdateOpts,
		PortSecurityEnabled: &iTrue,
	}

	err = networks.Update(client, network.ID, updateOpts).ExtractInto(&networkWithExtensions)
	if err != nil {
		t.Fatalf("Unable to update network: %v", err)
	}

	tools.PrintResource(t, networkWithExtensions)
}

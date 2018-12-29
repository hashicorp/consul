// +build acceptance networking vpnaas

package vpnaas

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	networks "github.com/gophercloud/gophercloud/acceptance/openstack/networking/v2"
	layer3 "github.com/gophercloud/gophercloud/acceptance/openstack/networking/v2/extensions/layer3"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"

	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/vpnaas/siteconnections"
)

func TestConnectionList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	allPages, err := siteconnections.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list IPSec site connections: %v", err)
	}

	allConnections, err := siteconnections.ExtractConnections(allPages)
	if err != nil {
		t.Fatalf("Unable to extract IPSec site connections: %v", err)
	}

	for _, connection := range allConnections {
		tools.PrintResource(t, connection)
	}
}

func TestConnectionCRUD(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	// Create Network
	network, err := networks.CreateNetwork(t, client)
	if err != nil {
		t.Fatalf("Unable to create network: %v", err)
	}
	defer networks.DeleteNetwork(t, client, network.ID)

	// Create Subnet
	subnet, err := networks.CreateSubnet(t, client, network.ID)
	if err != nil {
		t.Fatalf("Unable to create subnet: %v", err)
	}
	defer networks.DeleteSubnet(t, client, subnet.ID)

	router, err := layer3.CreateExternalRouter(t, client)
	if err != nil {
		t.Fatalf("Unable to create router: %v", err)
	}
	defer layer3.DeleteRouter(t, client, router.ID)

	// Link router and subnet
	aiOpts := routers.AddInterfaceOpts{
		SubnetID: subnet.ID,
	}

	_, err = routers.AddInterface(client, router.ID, aiOpts).Extract()
	if err != nil {
		t.Fatalf("Failed to add interface to router: %v", err)
	}
	defer func() {
		riOpts := routers.RemoveInterfaceOpts{
			SubnetID: subnet.ID,
		}
		routers.RemoveInterface(client, router.ID, riOpts)
	}()

	// Create all needed resources for the connection
	service, err := CreateService(t, client, router.ID)
	if err != nil {
		t.Fatalf("Unable to create service: %v", err)
	}
	defer DeleteService(t, client, service.ID)

	ikepolicy, err := CreateIKEPolicy(t, client)
	if err != nil {
		t.Fatalf("Unable to create IKE policy: %v", err)
	}
	defer DeleteIKEPolicy(t, client, ikepolicy.ID)

	ipsecpolicy, err := CreateIPSecPolicy(t, client)
	if err != nil {
		t.Fatalf("Unable to create IPSec Policy: %v", err)
	}
	defer DeleteIPSecPolicy(t, client, ipsecpolicy.ID)

	peerEPGroup, err := CreateEndpointGroup(t, client)
	if err != nil {
		t.Fatalf("Unable to create Endpoint Group with CIDR endpoints: %v", err)
	}
	defer DeleteEndpointGroup(t, client, peerEPGroup.ID)

	localEPGroup, err := CreateEndpointGroupWithSubnet(t, client, subnet.ID)
	if err != nil {
		t.Fatalf("Unable to create Endpoint Group with subnet endpoints: %v", err)
	}
	defer DeleteEndpointGroup(t, client, localEPGroup.ID)

	conn, err := CreateSiteConnection(t, client, ikepolicy.ID, ipsecpolicy.ID, service.ID, peerEPGroup.ID, localEPGroup.ID)
	if err != nil {
		t.Fatalf("Unable to create IPSec Site Connection: %v", err)
	}
	defer DeleteSiteConnection(t, client, conn.ID)

	newConnection, err := siteconnections.Get(client, conn.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to get connection: %v", err)
	}

	tools.PrintResource(t, conn)
	tools.PrintResource(t, newConnection)

}

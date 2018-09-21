// +build acceptance networking layer3 router

package layer3

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	networking "github.com/gophercloud/gophercloud/acceptance/openstack/networking/v2"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
)

func TestLayer3RouterList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	listOpts := routers.ListOpts{}
	allPages, err := routers.List(client, listOpts).AllPages()
	if err != nil {
		t.Fatalf("Unable to list routers: %v", err)
	}

	allRouters, err := routers.ExtractRouters(allPages)
	if err != nil {
		t.Fatalf("Unable to extract routers: %v", err)
	}

	for _, router := range allRouters {
		tools.PrintResource(t, router)
	}
}

func TestLayer3ExternalRouterCreateDelete(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	router, err := CreateExternalRouter(t, client)
	if err != nil {
		t.Fatalf("Unable to create router: %v", err)
	}
	defer DeleteRouter(t, client, router.ID)

	tools.PrintResource(t, router)

	newName := tools.RandomString("TESTACC-", 8)
	updateOpts := routers.UpdateOpts{
		Name: newName,
	}

	_, err = routers.Update(client, router.ID, updateOpts).Extract()
	if err != nil {
		t.Fatalf("Unable to update router: %v", err)
	}

	newRouter, err := routers.Get(client, router.ID).Extract()
	if err != nil {
		t.Fatalf("Unable to get router: %v", err)
	}

	tools.PrintResource(t, newRouter)
}

func TestLayer3RouterInterface(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a compute client: %v", err)
	}

	choices, err := clients.AcceptanceTestChoicesFromEnv()
	if err != nil {
		t.Fatalf("Unable to get choices: %v", err)
	}

	netid, err := networks.IDFromName(client, choices.NetworkName)
	if err != nil {
		t.Fatalf("Unable to find network id: %v", err)
	}

	subnet, err := networking.CreateSubnet(t, client, netid)
	if err != nil {
		t.Fatalf("Unable to create subnet: %v", err)
	}
	defer networking.DeleteSubnet(t, client, subnet.ID)

	tools.PrintResource(t, subnet)

	router, err := CreateExternalRouter(t, client)
	if err != nil {
		t.Fatalf("Unable to create router: %v", err)
	}
	defer DeleteRouter(t, client, router.ID)

	aiOpts := routers.AddInterfaceOpts{
		SubnetID: subnet.ID,
	}

	iface, err := routers.AddInterface(client, router.ID, aiOpts).Extract()
	if err != nil {
		t.Fatalf("Failed to add interface to router: %v", err)
	}

	tools.PrintResource(t, router)
	tools.PrintResource(t, iface)

	riOpts := routers.RemoveInterfaceOpts{
		SubnetID: subnet.ID,
	}

	_, err = routers.RemoveInterface(client, router.ID, riOpts).Extract()
	if err != nil {
		t.Fatalf("Failed to remove interface from router: %v", err)
	}
}

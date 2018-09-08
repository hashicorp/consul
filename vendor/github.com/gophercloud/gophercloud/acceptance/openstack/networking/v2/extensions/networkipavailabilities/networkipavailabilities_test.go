// +build acceptance networking networkipavailabilities

package networkipavailabilities

import (
	"testing"

	"github.com/gophercloud/gophercloud/acceptance/clients"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/networkipavailabilities"
)

func TestNetworkIPAvailabilityList(t *testing.T) {
	client, err := clients.NewNetworkV2Client()
	if err != nil {
		t.Fatalf("Unable to create a network client: %v", err)
	}

	allPages, err := networkipavailabilities.List(client, nil).AllPages()
	if err != nil {
		t.Fatalf("Unable to list network IP availabilities: %v", err)
	}

	allAvailabilities, err := networkipavailabilities.ExtractNetworkIPAvailabilities(allPages)
	if err != nil {
		t.Fatalf("Unable to extract network IP availabilities: %v", err)
	}

	for _, availability := range allAvailabilities {
		tools.PrintResource(t, availability)
	}
}

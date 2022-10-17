package assert

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// peeringStatus verifies the peering connection is active with a default retry.
func PeeringStatus(t *testing.T, client *api.Client, peerName string, status api.PeeringState) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultTimeout, Wait: defaultWait}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		peering, _, err := client.Peerings().Read(context.Background(), peerName, &api.QueryOptions{})
		if err != nil {
			r.Fatal("error reading peering data")
		}
		if status != peering.State {
			r.Fatal("peering state did not match: got ", peering.State, " want ", status)
		}
	})
}

func PeeringExports(t *testing.T, client *api.Client, peerName string, exports int) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultTimeout, Wait: defaultWait}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		peering, _, err := client.Peerings().Read(context.Background(), peerName, &api.QueryOptions{})
		if err != nil {
			r.Fatal("error reading peering data")
		}
		if exports != len(peering.StreamStatus.ExportedServices) {
			r.Fatal("peering exported services did not match: got ", len(peering.StreamStatus.ExportedServices), " want ", exports)
		}
	})
}

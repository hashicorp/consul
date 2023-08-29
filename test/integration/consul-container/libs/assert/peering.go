// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package assert

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PeeringStatus verifies the peering connection is the specified state with a default retry.
func PeeringStatus(t *testing.T, client *api.Client, peerName string, status api.PeeringState) {
	PeeringStatusOpts(t, client, peerName, status, nil)
}

// PeeringStatusOpts verifies the peering connection is the specified
// state with a default retry with options.
func PeeringStatusOpts(t *testing.T, client *api.Client, peerName string, status api.PeeringState, opts *api.QueryOptions) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 180 * time.Second, Wait: defaultWait}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		peering, _, err := client.Peerings().Read(context.Background(), peerName, opts)
		if err != nil {
			r.Fatal("error reading peering data")
		}
		if peering == nil {
			r.Fatal("peering not found")
		}
		if status != peering.State {
			r.Fatal("peering state did not match: got ", peering.State, " want ", status)
		}
	})
}

// PeeringExports verifies the correct number of exported services with a default retry.
func PeeringExports(t *testing.T, client *api.Client, peerName string, exports int) {
	PeeringExportsOpts(t, client, peerName, exports, nil)
}

// PeeringExportsOpts verifies the correct number of exported services
// with a default retry with options.
func PeeringExportsOpts(t *testing.T, client *api.Client, peerName string, exports int, opts *api.QueryOptions) {
	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: defaultTimeout, Wait: defaultWait}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		peering, _, err := client.Peerings().Read(context.Background(), peerName, opts)
		require.Nil(r, err, "reading peering data")
		require.NotNilf(r, peering, "peering not found %q", peerName)
		assert.Len(r, peering.StreamStatus.ExportedServices, exports, "peering exported services")
	})
}

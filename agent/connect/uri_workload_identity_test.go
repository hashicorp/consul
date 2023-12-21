// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpiffeIDWorkloadURI(t *testing.T) {
	t.Run("spiffe id workload uri default tenancy", func(t *testing.T) {
		wl := &SpiffeIDWorkloadIdentity{
			TrustDomain:      "1234.consul",
			WorkloadIdentity: "web",
			Partition:        "default",
			Namespace:        "default",
		}
		require.Equal(t, "spiffe://1234.consul/ap/default/ns/default/identity/web", wl.URI().String())
	})
	t.Run("spiffe id workload uri non-default tenancy", func(t *testing.T) {
		wl := &SpiffeIDWorkloadIdentity{
			TrustDomain:      "1234.consul",
			WorkloadIdentity: "web",
			Partition:        "part1",
			Namespace:        "dev",
		}
		require.Equal(t, "spiffe://1234.consul/ap/part1/ns/dev/identity/web", wl.URI().String())
	})
}

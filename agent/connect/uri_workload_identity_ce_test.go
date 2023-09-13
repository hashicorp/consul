// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpiffeIDWorkloadURI(t *testing.T) {
	t.Run("default partition; default namespace", func(t *testing.T) {
		wl := &SpiffeIDWorkloadIdentity{
			TrustDomain:      "1234.consul",
			WorkloadIdentity: "web",
		}
		require.Equal(t, "spiffe://1234.consul/ap/default/ns/default/identity/web", wl.URI().String())
	})

	t.Run("namespaces are ignored", func(t *testing.T) {
		wl := &SpiffeIDWorkloadIdentity{
			TrustDomain:      "1234.consul",
			WorkloadIdentity: "web",
			Namespace:        "other",
		}
		require.Equal(t, "spiffe://1234.consul/ap/default/ns/default/identity/web", wl.URI().String())
	})

	t.Run("partitions are not ignored", func(t *testing.T) {
		wl := &SpiffeIDWorkloadIdentity{
			TrustDomain:      "1234.consul",
			WorkloadIdentity: "web",
			Partition:        "other",
		}
		require.Equal(t, "spiffe://1234.consul/ap/other/ns/default/identity/web", wl.URI().String())
	})
}

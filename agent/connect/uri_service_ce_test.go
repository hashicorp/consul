// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpiffeIDServiceURI(t *testing.T) {
	t.Run("default partition; default namespace", func(t *testing.T) {
		svc := &SpiffeIDService{
			Host:       "1234.consul",
			Datacenter: "dc1",
			Service:    "web",
		}
		require.Equal(t, "spiffe://1234.consul/ns/default/dc/dc1/svc/web", svc.URI().String())
	})

	t.Run("namespaces are ignored", func(t *testing.T) {
		svc := &SpiffeIDService{
			Host:       "1234.consul",
			Namespace:  "other",
			Datacenter: "dc1",
			Service:    "web",
		}
		require.Equal(t, "spiffe://1234.consul/ns/default/dc/dc1/svc/web", svc.URI().String())
	})
}

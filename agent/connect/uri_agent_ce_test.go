// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpiffeIDAgentURI(t *testing.T) {
	t.Run("default partition", func(t *testing.T) {
		agent := &SpiffeIDAgent{
			Host:       "1234.consul",
			Datacenter: "dc1",
			Agent:      "123",
		}

		require.Equal(t, "spiffe://1234.consul/agent/client/dc/dc1/id/123", agent.URI().String())
	})

	t.Run("partitions are ignored", func(t *testing.T) {
		agent := &SpiffeIDAgent{
			Host:       "1234.consul",
			Partition:  "foobar",
			Datacenter: "dc1",
			Agent:      "123",
		}

		require.Equal(t, "spiffe://1234.consul/agent/client/dc/dc1/id/123", agent.URI().String())
	})
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_StatusLeader(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	leader, err := status.Leader()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("Expected leader, found empty string")
	}
}

func TestAPI_StatusLeaderWithQueryOptions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "dc1",
	}

	leader, err := status.LeaderWithQueryOptions(&opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("Expected leader, found empty string")
	}
}

func TestAPI_StatusPeers(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	peers, err := status.Peers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) == 0 {
		t.Fatalf("Expected peers, found %d", len(peers))
	}
}

func TestAPI_StatusPeersWithQueryOptions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "dc1",
	}

	peers, err := status.PeersWithQueryOptions(&opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) == 0 {
		t.Fatalf("Expected peers, found %d", len(peers))
	}
}

func TestAPI_StatusLeader_WrongDC(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "wrong_dc1",
	}

	_, err := status.LeaderWithQueryOptions(&opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "No path to datacenter")
}

func TestAPI_StatusPeers_WrongDC(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "wrong_dc1",
	}
	_, err := status.PeersWithQueryOptions(&opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "No path to datacenter")
}

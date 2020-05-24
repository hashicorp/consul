package api

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPI_StatusLeader(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "dc1",
	}

	leader, err := status.Leader(&opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("Expected leader")
	}
}

func TestAPI_StatusPeers(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "dc1",
	}
	peers, err := status.Peers(&opts)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) == 0 {
		t.Fatalf("Expected peers ")
	}
}

func TestAPI_StatusLeader_WrongDC(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "wrong_dc1",
	}
	_, err := status.Leader(&opts)
	require.Error(err)
	require.Contains(err.Error(), "No path to datacenter")
}

func TestAPI_StatusPeers_WrongDC(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t)
	defer s.Stop()
	s.WaitForSerfCheck(t)

	status := c.Status()

	opts := QueryOptions{
		Datacenter: "wrong_dc1",
	}
	_, err := status.Peers(&opts)
	require.Error(err)
	require.Contains(err.Error(), "No path to datacenter")
}

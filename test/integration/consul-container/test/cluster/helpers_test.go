// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package cluster

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// server is one Consul server of the datacenter under test. It hides how the
// server is run so that the tests only describe cluster behaviour.
type server interface {
	// Name is the Consul node name of the server.
	Name() string
	Client() *api.Client
	// Halt stops the server abruptly, keeping its data.
	Halt(t *testing.T)
	// Start brings a halted server back with its previous data.
	Start(t *testing.T)
	// CLI runs `consul <args>` against this server and returns the combined
	// output and the exit code.
	CLI(t *testing.T, args ...string) (string, int)
	// WriteDataFile creates a file, relative to the server's data directory.
	WriteDataFile(t *testing.T, name, content string)
}

// settle is how long cluster-level changes (elections, dead server cleanup,
// promotions) get before a test gives up.
func settle() *retry.Timer { return &retry.Timer{Timeout: 90 * time.Second, Wait: time.Second} }

// testCluster is a datacenter of Consul servers.
type testCluster struct {
	servers []server
	// add starts one more server that joins the datacenter.
	add func(t *testing.T) server

	// out holds servers that were halted or have left, so they are not queried.
	out map[string]bool
}

func (c *testCluster) live() []server {
	var out []server
	for _, s := range c.servers {
		if !c.out[s.Name()] {
			out = append(out, s)
		}
	}
	return out
}

func (c *testCluster) byName(name string) server {
	for _, s := range c.servers {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func (c *testCluster) setOut(s server, out bool) {
	if c.out == nil {
		c.out = map[string]bool{}
	}
	c.out[s.Name()] = out
}

func (c *testCluster) halt(t *testing.T, s server) {
	t.Helper()
	s.Halt(t)
	c.setOut(s, true)
}

func (c *testCluster) start(t *testing.T, s server) {
	t.Helper()
	s.Start(t)
	c.setOut(s, false)
}

func (c *testCluster) addServer(t *testing.T) server {
	t.Helper()
	s := c.add(t)
	c.servers = append(c.servers, s)
	return s
}

// leave makes a server leave gracefully. The agent exits afterwards.
func (c *testCluster) leave(t *testing.T, s server) {
	t.Helper()
	require.NoError(t, s.Client().Agent().Leave())
	c.setOut(s, true)
}

// anyLive returns a running server that is none of the excluded ones.
func (c *testCluster) anyLive(excluded ...server) server {
	for _, s := range c.live() {
		skip := false
		for _, e := range excluded {
			if e.Name() == s.Name() {
				skip = true
			}
		}
		if !skip {
			return s
		}
	}
	return nil
}

// raftConfig returns the raft configuration as seen by a running server.
func (c *testCluster) raftConfig(t retry.TestingTB) []*api.RaftServer {
	s := c.anyLive()
	require.NotNil(t, s, "no running server")
	conf, err := s.Client().Operator().RaftGetConfiguration(nil)
	require.NoError(t, err)
	return conf.Servers
}

// currentLeader returns the single running server that raft reports as leader.
func (c *testCluster) currentLeader(t retry.TestingTB) server {
	var leader server
	for _, rs := range c.raftConfig(t) {
		if rs.Leader {
			require.Nil(t, leader, "more than one leader")
			leader = c.byName(rs.Node)
		}
	}
	require.NotNil(t, leader, "no leader")
	require.False(t, c.out[leader.Name()], "the leader is a server that was stopped")
	return leader
}

// waitForLeader waits until a running server is the single raft leader and
// returns it.
func (c *testCluster) waitForLeader(t *testing.T) server {
	t.Helper()
	var leader server
	retry.RunWith(settle(), t, func(r *retry.R) {
		leader = c.currentLeader(r)
	})
	return leader
}

// waitForVoters waits until the raft configuration has exactly the given
// servers, all of them voting.
func (c *testCluster) waitForVoters(t *testing.T, want ...server) {
	t.Helper()
	retry.RunWith(settle(), t, func(r *retry.R) {
		var got []string
		for _, rs := range c.raftConfig(r) {
			require.True(r, rs.Voter, "%s is not a voter yet", rs.Node)
			got = append(got, rs.Node)
		}
		var names []string
		for _, s := range want {
			names = append(names, s.Name())
		}
		require.ElementsMatch(r, names, got)
	})
}

// waitForRaftPeers waits until the raft configuration contains exactly the
// given servers, whether or not they vote.
func (c *testCluster) waitForRaftPeers(t *testing.T, want ...server) {
	t.Helper()
	retry.RunWith(settle(), t, func(r *retry.R) {
		var got, names []string
		for _, rs := range c.raftConfig(r) {
			got = append(got, rs.Node)
		}
		for _, s := range want {
			names = append(names, s.Name())
		}
		require.ElementsMatch(r, names, got)
	})
}

// serfStatuses names the numeric serf member status reported by the API.
var serfStatuses = map[int]string{0: "none", 1: "alive", 2: "leaving", 3: "left", 4: "failed"}

// memberStatuses maps node name to serf status, as seen by a running server.
func (c *testCluster) memberStatuses(t retry.TestingTB) map[string]string {
	s := c.anyLive()
	require.NotNil(t, s, "no running server")
	members, err := s.Client().Agent().Members(false)
	require.NoError(t, err)
	out := map[string]string{}
	for _, m := range members {
		out[m.Name] = serfStatuses[m.Status]
	}
	return out
}

func (c *testCluster) waitForMemberStatus(t *testing.T, s server, status string) {
	t.Helper()
	retry.RunWith(settle(), t, func(r *retry.R) {
		require.Equal(r, status, c.memberStatuses(r)[s.Name()])
	})
}

// requireWrite checks that the datacenter accepts a write through the given
// server, retrying while a leader is being established.
func requireWrite(t *testing.T, s server, key, value string) {
	t.Helper()
	retry.RunWith(settle(), t, func(r *retry.R) {
		_, err := s.Client().KV().Put(&api.KVPair{Key: key, Value: []byte(value)}, nil)
		require.NoError(r, err)
	})
}

// requireStaleValue checks the value of a key as held by one specific server.
func requireStaleValue(t *testing.T, s server, key, value string) {
	t.Helper()
	retry.RunWith(settle(), t, func(r *retry.R) {
		pair, _, err := s.Client().KV().Get(key, &api.QueryOptions{AllowStale: true})
		require.NoError(r, err)
		require.NotNil(r, pair, "%s has no key %q", s.Name(), key)
		require.Equal(r, value, string(pair.Value))
	})
}

func requireCLI(t *testing.T, s server, args ...string) string {
	t.Helper()
	out, code := s.CLI(t, args...)
	require.Equal(t, 0, code, "consul %s failed: %s", strings.Join(args, " "), out)
	return out
}

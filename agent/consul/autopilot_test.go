// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul/autopilotevents"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestAutopilot_IdempotentShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir1, s1 := testServerWithConfig(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	retry.Run(t, func(r *retry.R) { r.Check(waitForLeader(s1)) })

	s1.autopilot.Start(context.Background())
	s1.autopilot.Start(context.Background())
	s1.autopilot.Start(context.Background())
	<-s1.autopilot.Stop()
	<-s1.autopilot.Stop()
	<-s1.autopilot.Stop()
}

func TestAutopilot_CleanupDeadServer(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dc := "dc1"
	conf := func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = false
		c.BootstrapExpect = 5
	}
	dir1, s1 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	dir5, s5 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir5)
	defer s5.Shutdown()

	servers := []*Server{s1, s2, s3, s4, s5}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	joinLAN(t, s4, s1)
	joinLAN(t, s5, s1)

	for _, s := range servers {
		testrpc.WaitForLeader(t, s.RPC, dc)
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 5)) })
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	leaderIndex := -1
	for i, s := range servers {
		if s.IsLeader() {
			leaderIndex = i
			break
		}
	}
	require.NotEqual(t, leaderIndex, -1)

	// Shutdown two non-leader servers
	killed := make(map[string]struct{})
	for i, s := range servers {
		if i != leaderIndex {
			s.Shutdown()
			killed[string(s.config.NodeID)] = struct{}{}
		}
		if len(killed) == 2 {
			break
		}
	}

	retry.Run(t, func(r *retry.R) {
		alive := 0
		for _, m := range servers[leaderIndex].LANMembersInAgentPartition() {
			if m.Status == serf.StatusAlive {
				alive++
			}
		}
		if alive != 3 {
			r.Fatalf("Expected three alive servers instead of %d", alive)
		}
	})

	// Make sure the dead servers are removed and we're back to 3 total peers
	for _, s := range servers {
		_, killed := killed[string(s.config.NodeID)]
		if !killed {
			retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
		}
	}
}

func TestAutopilot_CleanupDeadNonvoter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.AutopilotConfig = &structs.AutopilotConfig{
			CleanupDeadServers:      true,
			ServerStabilizationTime: 100 * time.Millisecond,
		}
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	// we have to wait for autopilot to be running long enough for the server stabilization time
	// to kick in for this test to work.
	time.Sleep(100 * time.Millisecond)

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Have s2 join and then shut it down immediately before it gets a chance to
	// be promoted to a voter.
	joinLAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1, s2}))
	})
	s2.Shutdown()

	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft([]*Server{s1}))
	})
}

func TestAutopilot_CleanupDeadServerPeriodic(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	conf := func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
	}

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	dir5, s5 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir5)
	defer s5.Shutdown()

	servers := []*Server{s1, s2, s3, s4, s5}

	// Join the servers to s1, and wait until they are all promoted to
	// voters.
	for _, s := range servers[1:] {
		joinLAN(t, s, s1)
	}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 5))
		}
	})

	// Kill a non-leader server
	s4.Shutdown()

	// Should be removed from the peers automatically
	servers = []*Server{s1, s2, s3, s5}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 4))
		}
	})
}

func TestAutopilot_RollingUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	conf := func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
	}

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	// Join the servers to s1, and wait until they are all promoted to
	// voters.
	servers := []*Server{s1, s2, s3}
	for _, s := range servers[1:] {
		joinLAN(t, s, s1)
	}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	// Add one more server like we are doing a rolling update.
	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()
	joinLAN(t, s1, s4)
	servers = append(servers, s4)
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
	})

	// Now kill one of the "old" nodes like we are doing a rolling update.
	s3.Shutdown()

	isVoter := func() bool {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		for _, s := range future.Configuration().Servers {
			if string(s.ID) == string(s4.config.NodeID) {
				return s.Suffrage == raft.Voter
			}
		}
		t.Fatalf("didn't find s4")
		return false
	}

	// Wait for s4 to stabilize, get promoted to a voter, and for s3 to be
	// removed.
	servers = []*Server{s1, s2, s4}
	retry.Run(t, func(r *retry.R) {
		r.Check(wantRaft(servers))
		for _, s := range servers {
			r.Check(wantPeers(s, 3))
		}
		if !isVoter() {
			r.Fatalf("should be a voter")
		}
	})
}

func TestAutopilot_CleanupStaleRaftServer(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerDCBootstrap(t, "dc1", true)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerDCBootstrap(t, "dc1", false)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	servers := []*Server{s1, s2, s3}

	// Join the servers to s1
	for _, s := range servers[1:] {
		joinLAN(t, s, s1)
	}

	for _, s := range servers {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// Add s4 to peers directly
	addVoterFuture := s1.raft.AddVoter(raft.ServerID(s4.config.NodeID), raft.ServerAddress(joinAddrLAN(s4)), 0, 0)
	if err := addVoterFuture.Error(); err != nil {
		t.Fatal(err)
	}

	// Verify we have 4 peers
	peers, err := s1.autopilot.NumVoters()
	if err != nil {
		t.Fatal(err)
	}
	if peers != 4 {
		t.Fatalf("bad: %v", peers)
	}

	// Wait for s4 to be removed
	for _, s := range []*Server{s1, s2, s3} {
		retry.Run(t, func(r *retry.R) { r.Check(wantPeers(s, 3)) })
	}
}

func TestAutopilot_PromoteNonVoter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = true
		c.AutopilotConfig.ServerStabilizationTime = 200 * time.Millisecond
		c.ServerHealthInterval = 100 * time.Millisecond
		c.AutopilotInterval = 100 * time.Millisecond
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	// this may seem arbitrary but we need to get past the server stabilization time
	// so that we start factoring in that time for newly connected nodes.
	time.Sleep(100 * time.Millisecond)

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc1"
		c.Bootstrap = false
		c.RaftConfig.ProtocolVersion = 3
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	joinLAN(t, s2, s1)

	// Make sure we see it as a nonvoter initially. We wait until half
	// the stabilization period has passed.
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		servers := future.Configuration().Servers
		if len(servers) != 2 {
			r.Fatalf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Nonvoter {
			r.Fatalf("bad: %v", servers)
		}
		health := s1.autopilot.GetServerHealth(servers[1].ID)
		if health == nil {
			r.Fatal("nil health")
		}
		if !health.Healthy {
			r.Fatalf("bad: %v", health)
		}
		if time.Since(health.StableSince) < s1.config.AutopilotConfig.ServerStabilizationTime/2 {
			r.Fatal("stable period not elapsed")
		}
	})

	// Make sure it ends up as a voter.
	retry.Run(t, func(r *retry.R) {
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			r.Fatal(err)
		}

		servers := future.Configuration().Servers
		if len(servers) != 2 {
			r.Fatalf("bad: %v", servers)
		}
		if servers[1].Suffrage != raft.Voter {
			r.Fatalf("bad: %v", servers)
		}
	})
}

func TestAutopilot_MinQuorum(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	dc := "dc1"
	conf := func(c *Config) {
		c.Datacenter = dc
		c.Bootstrap = false
		c.BootstrapExpect = 4
		c.AutopilotConfig.MinQuorum = 3
		c.AutopilotInterval = 100 * time.Millisecond
	}
	dir1, s1 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()

	dir2, s2 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	dir3, s3 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir3)
	defer s3.Shutdown()

	dir4, s4 := testServerWithConfig(t, conf)
	defer os.RemoveAll(dir4)
	defer s4.Shutdown()

	servers := map[string]*Server{s1.config.NodeName: s1,
		s2.config.NodeName: s2,
		s3.config.NodeName: s3,
		s4.config.NodeName: s4}

	// Try to join
	joinLAN(t, s2, s1)
	joinLAN(t, s3, s1)
	joinLAN(t, s4, s1)

	//Differentiate between leader and server
	findStatus := func(leader bool) *Server {
		for _, mem := range servers {
			if mem.IsLeader() == leader {
				return mem
			}
			if !mem.IsLeader() == !leader {
				return mem
			}
		}

		return nil
	}
	testrpc.WaitForLeader(t, s1.RPC, dc)

	// Have autopilot take one into left
	dead := findStatus(false)
	if dead == nil {
		t.Fatalf("no members set")
	}
	require.NoError(t, dead.Shutdown())
	retry.Run(t, func(r *retry.R) {
		leader := findStatus(true)
		if leader == nil {
			r.Fatalf("no members set")
		}
		for _, m := range leader.LANMembersInAgentPartition() {
			if m.Name == dead.config.NodeName && m.Status != serf.StatusLeft {
				r.Fatalf("%v should be left, got %v", m.Name, m.Status.String())
			}
		}
	})

	delete(servers, dead.config.NodeName)
	//Autopilot should not take this one into left
	dead = findStatus(false)
	require.NoError(t, dead.Shutdown())

	retry.Run(t, func(r *retry.R) {
		leader := findStatus(true)
		if leader == nil {
			r.Fatalf("no members set")
		}
		for _, m := range leader.LANMembersInAgentPartition() {
			if m.Name == dead.config.NodeName && m.Status != serf.StatusFailed {
				r.Fatalf("%v should be failed, got %v", m.Name, m.Status.String())
			}
		}
	})
}

func TestAutopilot_EventPublishing(t *testing.T) {
	// This is really an integration level test. The general flow this test will follow is:
	//
	// 1. Start a 3 server cluster
	// 2. Subscribe to the ready server events
	// 3. Observe the first event which will be pretty immediately ready as it is the
	//    snapshot event.
	// 4. Wait for multiple iterations of the autopilot state updater and ensure no
	//    other events are seen. The state update interval is 50ms for tests unless
	//    overridden.
	// 5. Add a fouth server.
	// 6. Wait for an event to be emitted containing 4 ready servers.

	// 1. create the test cluster
	cluster := newTestCluster(t, &testClusterConfig{
		Servers:    3,
		ServerConf: testServerACLConfig,
		// We want to wait until each server has registered itself in the Catalog. Otherwise
		// the first snapshot even we see might have no servers in it while things are being
		// initialized. Doing this wait ensure that things are in the right state to start
		// the subscription.
	})

	// 2. subscribe to ready server events
	req := stream.SubscribeRequest{
		Topic:   autopilotevents.EventTopicReadyServers,
		Subject: stream.SubjectNone,
		Token:   TestDefaultInitialManagementToken,
	}
	sub, err := cluster.Servers[0].publisher.Subscribe(&req)
	require.NoError(t, err)
	t.Cleanup(sub.Unsubscribe)

	// 3. Observe that an event was generated which should be the snapshot event.
	//    As we have just bootstrapped the cluster with 3 servers we expect to
	//    see those 3 here.
	validatePayload(t, 3, mustGetEventWithTimeout(t, sub, 50*time.Millisecond))

	// TODO - its kind of annoying that the EventPublisher doesn't have a mode where
	// it knows each event is a full state of the world. The ramifications are that
	// we have to expect/ignore the framing events for EndOfSnapshot.
	event := mustGetEventWithTimeout(t, sub, 10*time.Millisecond)
	require.True(t, event.IsFramingEvent())

	// 4. Wait for 3 iterations of the ServerHealthInterval to ensure no events
	//    are being published when the autopilot state is not changing.
	eventNotEmitted(t, sub, 150*time.Millisecond)

	// 5. Add a fourth server
	_, srv := testServerWithConfig(t, testServerACLConfig, func(c *Config) {
		c.Bootstrap = false
		c.BootstrapExpect = 0
	})
	joinLAN(t, srv, cluster.Servers[0])

	// 6. Now wait for the event for the fourth server being added. This may take a little
	//    while as the joinLAN operation above doesn't wait for the server to actually get
	//    added to Raft.
	validatePayload(t, 4, mustGetEventWithTimeout(t, sub, time.Second))
}

// mustGetEventWithTimeout is a helper function for validating that a Subscription.Next call will return
// an event within the given time. It also validates that no error is returned.
func mustGetEventWithTimeout(t *testing.T, subscription *stream.Subscription, timeout time.Duration) stream.Event {
	t.Helper()
	event, err := getEventWithTimeout(t, subscription, timeout)
	require.NoError(t, err)
	return event
}

// getEventWithTimeout is a helper function for retrieving a Event from a Subscription within the specified timeout.
func getEventWithTimeout(t *testing.T, subscription *stream.Subscription, timeout time.Duration) (stream.Event, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := subscription.Next(ctx)
	return event, err
}

// eventNotEmitted is a helper to validate that no Event is emitted for the given Subscription
func eventNotEmitted(t *testing.T, subscription *stream.Subscription, timeout time.Duration) {
	t.Helper()
	var event stream.Event
	var err error
	event, err = getEventWithTimeout(t, subscription, timeout)
	require.Equal(t, context.DeadlineExceeded, err, fmt.Sprintf("event:%v", event))
}

func validatePayload(t *testing.T, expectedNumServers int, event stream.Event) {
	t.Helper()
	require.Equal(t, autopilotevents.EventTopicReadyServers, event.Topic)
	readyServers, ok := event.Payload.(autopilotevents.EventPayloadReadyServers)
	require.True(t, ok)
	require.Len(t, readyServers, expectedNumServers)
}

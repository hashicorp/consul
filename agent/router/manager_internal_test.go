// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package router

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/metadata"
)

var (
	localLogBuffer *bytes.Buffer
)

func init() {
	localLogBuffer = new(bytes.Buffer)
}

func GetBufferedLogger() hclog.Logger {
	localLogBuffer = new(bytes.Buffer)
	return hclog.New(&hclog.LoggerOptions{Output: localLogBuffer})
}

type fauxConnPool struct {
	// failPct between 0.0 and 1.0 == pct of time a Ping should fail
	failPct float64
}

func (cp *fauxConnPool) Ping(string, string, net.Addr) (bool, error) {
	var success bool
	successProb := rand.Float64()
	if successProb > cp.failPct {
		success = true
	}
	return success, nil
}

type fauxSerf struct {
	numNodes int
}

func (s *fauxSerf) NumNodes() int {
	return s.numNodes
}

func testManager() (m *Manager) {
	logger := GetBufferedLogger()
	shutdownCh := make(chan struct{})
	m = New(logger, shutdownCh, &fauxSerf{numNodes: 16384}, &fauxConnPool{}, "", noopRebalancer)
	return m
}

func noopRebalancer() {}

func testManagerFailProb(failPct float64) (m *Manager) {
	logger := GetBufferedLogger()
	shutdownCh := make(chan struct{})
	m = New(logger, shutdownCh, &fauxSerf{}, &fauxConnPool{failPct: failPct}, "", noopRebalancer)
	return m
}

// func (l *serverList) cycleServer() (servers []*metadata.Server) {
func TestManagerInternal_cycleServer(t *testing.T) {
	m := testManager()
	l := m.getServerList()

	server0 := &metadata.Server{Name: "server1"}
	server1 := &metadata.Server{Name: "server2"}
	server2 := &metadata.Server{Name: "server3"}
	l.servers = append(l.servers, server0, server1, server2)
	m.saveServerList(l)

	l = m.getServerList()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server0 &&
		l.servers[1] != server1 &&
		l.servers[2] != server2 {
		t.Fatalf("initial server ordering not correct")
	}

	l.servers = l.cycleServer()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server1 &&
		l.servers[1] != server2 &&
		l.servers[2] != server0 {
		t.Fatalf("server ordering after one cycle not correct")
	}

	l.servers = l.cycleServer()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server2 &&
		l.servers[1] != server0 &&
		l.servers[2] != server1 {
		t.Fatalf("server ordering after two cycles not correct")
	}

	l.servers = l.cycleServer()
	if len(l.servers) != 3 {
		t.Fatalf("server length incorrect: %d/3", len(l.servers))
	}
	if l.servers[0] != server0 &&
		l.servers[1] != server1 &&
		l.servers[2] != server2 {
		t.Fatalf("server ordering after three cycles not correct")
	}
}

// func (m *Manager) getServerList() serverList {
func TestManagerInternal_getServerList(t *testing.T) {
	m := testManager()
	l := m.getServerList()
	if l.servers == nil {
		t.Fatalf("serverList.servers nil")
	}

	if len(l.servers) != 0 {
		t.Fatalf("serverList.servers length not zero")
	}
}

func TestManagerInternal_New(t *testing.T) {
	m := testManager()
	if m == nil {
		t.Fatalf("Manager nil")
	}

	if m.clusterInfo == nil {
		t.Fatalf("Manager.clusterInfo nil")
	}

	if m.logger == nil {
		t.Fatalf("Manager.logger nil")
	}

	if m.shutdownCh == nil {
		t.Fatalf("Manager.shutdownCh nil")
	}
}

// func (m *Manager) reconcileServerList(l *serverList) bool {
func TestManagerInternal_reconcileServerList(t *testing.T) {
	tests := []int{0, 1, 2, 3, 4, 5, 10, 100}
	for _, n := range tests {
		ok, err := test_reconcileServerList(n)
		if !ok {
			t.Errorf("Expected %d to pass: %v", n, err)
		}
	}
}

func test_reconcileServerList(maxServers int) (bool, error) {
	// Build a server list, reconcile, verify the missing servers are
	// missing, the added have been added, and the original server is
	// present.
	const failPct = 0.5
	m := testManagerFailProb(failPct)

	var failedServers, healthyServers []*metadata.Server
	for i := 0; i < maxServers; i++ {
		nodeName := fmt.Sprintf("s%02d", i)

		node := &metadata.Server{Name: nodeName}
		// Add 66% of servers to Manager
		if rand.Float64() > 0.33 {
			m.AddServer(node)

			// Of healthy servers, (ab)use connPoolPinger to
			// failPct of the servers for the reconcile.  This
			// allows for the selected server to no longer be
			// healthy for the reconcile below.
			if ok, _ := m.connPoolPinger.Ping(node.Datacenter, node.ShortName, node.Addr); ok {
				// Will still be present
				healthyServers = append(healthyServers, node)
			} else {
				// Will be missing
				failedServers = append(failedServers, node)
			}
		} else {
			// Will be added from the call to reconcile
			healthyServers = append(healthyServers, node)
		}
	}

	// Randomize Manager's server list
	m.RebalanceServers()
	selectedServer := m.FindServer()

	var selectedServerFailed bool
	for _, s := range failedServers {
		if selectedServer.Key().Equal(s.Key()) {
			selectedServerFailed = true
			break
		}
	}

	// Update Manager's server list to be "healthy" based on Serf.
	// Reconcile this with origServers, which is shuffled and has a live
	// connection, but possibly out of date.
	origServers := m.getServerList()
	m.saveServerList(serverList{servers: healthyServers})

	// This should always succeed with non-zero server lists
	if !selectedServerFailed && !m.reconcileServerList(&origServers) &&
		len(m.getServerList().servers) != 0 &&
		len(origServers.servers) != 0 {
		// If the random gods are unfavorable and we end up with zero
		// length lists, expect things to fail and retry the test.
		return false, fmt.Errorf("Expected reconcile to succeed: %v %d %d",
			selectedServerFailed,
			len(m.getServerList().servers),
			len(origServers.servers))
	}

	// If we have zero-length server lists, test succeeded in degenerate
	// case.
	if len(m.getServerList().servers) == 0 &&
		len(origServers.servers) == 0 {
		// Failed as expected w/ zero length list
		return true, nil
	}

	resultingServerMap := make(map[metadata.Key]bool)
	for _, s := range m.getServerList().servers {
		resultingServerMap[*s.Key()] = true
	}

	// Test to make sure no failed servers are in the Manager's
	// list.  Error if there are any failedServers in l.servers
	for _, s := range failedServers {
		_, ok := resultingServerMap[*s.Key()]
		if ok {
			return false, fmt.Errorf("Found failed server %v in merged list %v", s, resultingServerMap)
		}
	}

	// Test to make sure all healthy servers are in the healthy list.
	if len(healthyServers) != len(m.getServerList().servers) {
		return false, fmt.Errorf("Expected healthy map and servers to match: %d/%d", len(healthyServers), len(healthyServers))
	}

	// Test to make sure all healthy servers are in the resultingServerMap list.
	for _, s := range healthyServers {
		_, ok := resultingServerMap[*s.Key()]
		if !ok {
			return false, fmt.Errorf("Server %v missing from healthy map after merged lists", s)
		}
	}
	return true, nil
}

func TestRebalanceDelayer(t *testing.T) {
	type testCase struct {
		servers  int
		nodes    int
		expected time.Duration
	}

	testCases := []testCase{
		{servers: 0, nodes: 1},
		{servers: 0, nodes: 100},
		{servers: 0, nodes: 65535},
		{servers: 0, nodes: 1000000},

		{servers: 1, nodes: 100},
		{servers: 1, nodes: 1024},
		{servers: 1, nodes: 8192},
		{servers: 1, nodes: 11520},
		{servers: 1, nodes: 11521, expected: 3*time.Minute + 15625*time.Microsecond},
		{servers: 1, nodes: 16384, expected: 4*time.Minute + 16*time.Second},
		{servers: 1, nodes: 65535, expected: 17*time.Minute + 3984375000},
		{servers: 1, nodes: 1000000, expected: 4*time.Hour + 20*time.Minute + 25*time.Second},

		{servers: 2, nodes: 100},
		{servers: 2, nodes: 16384},
		{servers: 2, nodes: 23040},
		{servers: 2, nodes: 23041, expected: 3*time.Minute + 7812500},
		{servers: 2, nodes: 65535, expected: 8*time.Minute + 31992187500},
		{servers: 2, nodes: 1000000, expected: 2*time.Hour + 10*time.Minute + 12500*time.Millisecond},

		{servers: 3, nodes: 0},
		{servers: 3, nodes: 100},
		{servers: 3, nodes: 1024},
		{servers: 3, nodes: 16384},
		{servers: 3, nodes: 34560},
		{servers: 3, nodes: 34561, expected: 3*time.Minute + 5208333},
		{servers: 3, nodes: 65535, expected: 5*time.Minute + 41328125000},
		{servers: 3, nodes: 1000000, expected: 86*time.Minute + 48333333333},

		{servers: 5, nodes: 0},
		{servers: 5, nodes: 1024},
		{servers: 5, nodes: 16384},
		{servers: 5, nodes: 32768},
		{servers: 5, nodes: 57600},
		{servers: 5, nodes: 65535, expected: 3*time.Minute + 24796875000},
		{servers: 5, nodes: 1000000, expected: 52*time.Minute + 5*time.Second},

		{servers: 7, nodes: 65535},
		{servers: 7, nodes: 80500},
		{servers: 7, nodes: 131070, expected: 4*time.Minute + 52566964285},

		{servers: 11, nodes: 1000000, expected: 23*time.Minute + 40454545454},
		{servers: 19, nodes: 1000000, expected: 13*time.Minute + 42368421052},
	}

	for _, tc := range testCases {
		delay := delayer.Delay(tc.servers, tc.nodes)

		if tc.expected != 0 {
			assert.Equal(t, tc.expected, delay, "nodes=%d, servers=%d", tc.nodes, tc.servers)
			continue
		}

		min := 2 * time.Minute
		max := 3 * time.Minute
		if delay < min {
			t.Errorf("nodes=%d, servers=%d, expected >%v, actual %v", tc.nodes, tc.servers, min, delay)
		}
		if delay > max {
			t.Errorf("nodes=%d, servers=%d, expected <%v, actual %v", tc.nodes, tc.servers, max, delay)
		}
	}
}

// func (m *Manager) saveServerList(l serverList) {
func TestManagerInternal_saveServerList(t *testing.T) {
	m := testManager()

	// Initial condition
	func() {
		l := m.getServerList()
		if len(l.servers) != 0 {
			t.Fatalf("Manager.saveServerList failed to load init config")
		}

		newServer := new(metadata.Server)
		l.servers = append(l.servers, newServer)
		m.saveServerList(l)
	}()

	// Test that save works
	func() {
		l1 := m.getServerList()
		t1NumServers := len(l1.servers)
		if t1NumServers != 1 {
			t.Fatalf("Manager.saveServerList failed to save mutated config")
		}
	}()

	// Verify mutation w/o a save doesn't alter the original
	func() {
		newServer := new(metadata.Server)
		l := m.getServerList()
		l.servers = append(l.servers, newServer)

		l_orig := m.getServerList()
		origNumServers := len(l_orig.servers)
		if origNumServers >= len(l.servers) {
			t.Fatalf("Manager.saveServerList unsaved config overwrote original")
		}
	}()
}

func TestManager_healthyServer(t *testing.T) {
	t.Run("checking itself", func(t *testing.T) {
		m := testManager()
		m.serverName = "s1"
		server := metadata.Server{Name: m.serverName}
		require.True(t, m.healthyServer(&server))
	})
	t.Run("checking another server with successful ping", func(t *testing.T) {
		m := testManager()
		server := metadata.Server{Name: "s1"}
		require.True(t, m.healthyServer(&server))
	})
	t.Run("checking another server with failed ping", func(t *testing.T) {
		m := testManagerFailProb(1)
		server := metadata.Server{Name: "s1"}
		require.False(t, m.healthyServer(&server))
	})
}

func TestManager_Rebalance(t *testing.T) {
	t.Run("single server cluster checking itself", func(t *testing.T) {
		m := testManager()
		m.serverName = "s1"
		m.AddServer(&metadata.Server{Name: m.serverName})
		m.RebalanceServers()
		require.False(t, m.IsOffline())
	})
	t.Run("multi server cluster is unhealthy when pings always fail", func(t *testing.T) {
		m := testManagerFailProb(1)
		m.AddServer(&metadata.Server{Name: "s1"})
		m.AddServer(&metadata.Server{Name: "s2"})
		m.AddServer(&metadata.Server{Name: "s3"})
		for i := 0; i < 100; i++ {
			m.RebalanceServers()
			require.True(t, m.IsOffline())
		}
	})
	t.Run("multi server cluster checking itself remains healthy despite pings always fail", func(t *testing.T) {
		m := testManagerFailProb(1)
		m.serverName = "s1"
		m.AddServer(&metadata.Server{Name: m.serverName})
		m.AddServer(&metadata.Server{Name: "s2"})
		m.AddServer(&metadata.Server{Name: "s3"})
		for i := 0; i < 100; i++ {
			m.RebalanceServers()
			require.False(t, m.IsOffline())
		}
	})
}

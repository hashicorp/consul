package consul

import (
	"fmt"
	"net"
	"regexp"
	"testing"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func TestUtil_CanServersUnderstandProtocol(t *testing.T) {
	t.Parallel()
	var members []serf.Member

	// All empty list cases should return false.
	for v := ProtocolVersionMin; v <= ProtocolVersionMax; v++ {
		grok, err := CanServersUnderstandProtocol(members, v)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if grok {
			t.Fatalf("empty list should always return false")
		}
	}

	// Add a non-server member.
	members = append(members, serf.Member{
		Tags: map[string]string{
			"vsn_min": fmt.Sprintf("%d", ProtocolVersionMin),
			"vsn_max": fmt.Sprintf("%d", ProtocolVersionMax),
		},
	})

	// Make sure it doesn't get counted.
	for v := ProtocolVersionMin; v <= ProtocolVersionMax; v++ {
		grok, err := CanServersUnderstandProtocol(members, v)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if grok {
			t.Fatalf("non-server members should not be counted")
		}
	}

	// Add a server member.
	members = append(members, serf.Member{
		Tags: map[string]string{
			"role":    "consul",
			"vsn_min": fmt.Sprintf("%d", ProtocolVersionMin),
			"vsn_max": fmt.Sprintf("%d", ProtocolVersionMax),
		},
	})

	// Now it should report that it understands.
	for v := ProtocolVersionMin; v <= ProtocolVersionMax; v++ {
		grok, err := CanServersUnderstandProtocol(members, v)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !grok {
			t.Fatalf("server should grok")
		}
	}

	// Nobody should understand anything from the future.
	for v := uint8(ProtocolVersionMax + 1); v <= uint8(ProtocolVersionMax+10); v++ {
		grok, err := CanServersUnderstandProtocol(members, v)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if grok {
			t.Fatalf("server should not grok")
		}
	}

	// Add an older server.
	members = append(members, serf.Member{
		Tags: map[string]string{
			"role":    "consul",
			"vsn_min": fmt.Sprintf("%d", ProtocolVersionMin),
			"vsn_max": fmt.Sprintf("%d", ProtocolVersionMax-1),
		},
	})

	// The servers should no longer understand the max version.
	for v := ProtocolVersionMin; v <= ProtocolVersionMax; v++ {
		grok, err := CanServersUnderstandProtocol(members, v)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		expected := v < ProtocolVersionMax
		if grok != expected {
			t.Fatalf("bad: %v != %v", grok, expected)
		}
	}

	// Try a version that's too low for the minimum.
	{
		grok, err := CanServersUnderstandProtocol(members, 0)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if grok {
			t.Fatalf("server should not grok")
		}
	}
}

func TestIsConsulNode(t *testing.T) {
	t.Parallel()
	m := serf.Member{
		Tags: map[string]string{
			"role": "node",
			"dc":   "east-aws",
		},
	}
	valid, dc := isConsulNode(m)
	if !valid || dc != "east-aws" {
		t.Fatalf("bad: %v %v", valid, dc)
	}
}

func TestGenerateUUID(t *testing.T) {
	t.Parallel()
	prev := generateUUID()
	re, err := regexp.Compile("[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}")
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		id := generateUUID()
		if prev == id {
			t.Fatalf("Should get a new ID!")
		}

		matched := re.MatchString(id)
		if !matched {
			t.Fatalf("expected match %s %v %s", id, matched, err)
		}
	}
}

type testServersProvider []metadata.Server

func (p testServersProvider) CheckServers(datacenter string, fn func(*metadata.Server) bool) {
	for _, srv := range p {
		// filter these out - now I don't have to modify the tests. Originally the dc filtering
		// happened in the ServersInDCMeetMinimumVersion, now we get a list of servers to check
		// through the routing infrastructure or server lookup which will map a datacenter to a
		// list of metadata.Server structs that are all in that datacenter.
		if srv.Datacenter != datacenter {
			continue
		}

		if !fn(&srv) {
			return
		}
	}
}

func TestServersInDCMeetMinimumVersion(t *testing.T) {
	t.Parallel()
	makeServer := func(versionStr string, datacenter string) metadata.Server {
		return metadata.Server{
			Name:        "foo",
			ShortName:   "foo",
			ID:          "asdf",
			Port:        10000,
			Expect:      3,
			RaftVersion: 3,
			Status:      serf.StatusAlive,
			WanJoinPort: 1234,
			Version:     1,
			Build:       *version.Must(version.NewVersion(versionStr)),
			Datacenter:  datacenter,
		}
	}

	cases := []struct {
		servers       testServersProvider
		ver           *version.Version
		expected      bool
		expectedFound bool
	}{
		// One server, meets reqs
		{
			servers: testServersProvider{
				makeServer("0.7.5", "primary"),
				makeServer("0.7.3", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.7.5")),
			expected:      true,
			expectedFound: true,
		},
		// One server, doesn't meet reqs
		{
			servers: testServersProvider{
				makeServer("0.7.5", "primary"),
				makeServer("0.8.1", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.8.0")),
			expected:      false,
			expectedFound: true,
		},
		// Multiple servers, meets req version
		{
			servers: testServersProvider{
				makeServer("0.7.5", "primary"),
				makeServer("0.8.0", "primary"),
				makeServer("0.7.0", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.7.5")),
			expected:      true,
			expectedFound: true,
		},
		// Multiple servers, doesn't meet req version
		{
			servers: testServersProvider{
				makeServer("0.7.5", "primary"),
				makeServer("0.8.0", "primary"),
				makeServer("0.9.1", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.8.0")),
			expected:      false,
			expectedFound: true,
		},
		{
			servers: testServersProvider{
				makeServer("0.7.5", "secondary"),
				makeServer("0.8.0", "secondary"),
				makeServer("0.9.1", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.7.0")),
			expected:      true,
			expectedFound: false,
		},
	}

	for _, tc := range cases {
		result, found := ServersInDCMeetMinimumVersion(tc.servers, "primary", tc.ver)
		require.Equal(t, tc.expected, result)
		require.Equal(t, tc.expectedFound, found)
	}
}

func TestServersGetACLMode(t *testing.T) {
	t.Parallel()
	makeServer := func(datacenter string, acls structs.ACLMode, status serf.MemberStatus, addr net.IP) metadata.Server {
		return metadata.Server{
			Name:        "foo",
			ShortName:   "foo",
			ID:          "asdf",
			Port:        10000,
			Expect:      3,
			RaftVersion: 3,
			Status:      status,
			WanJoinPort: 1234,
			Version:     1,
			Addr:        &net.TCPAddr{IP: addr, Port: 10000},
			// shouldn't matter for these tests
			Build:      *version.Must(version.NewVersion("1.7.0")),
			Datacenter: datacenter,
			ACLs:       acls,
		}
	}

	type tcase struct {
		servers      testServersProvider
		leaderAddr   string
		datacenter   string
		foundServers bool
		minMode      structs.ACLMode
		leaderMode   structs.ACLMode
	}

	cases := map[string]tcase{
		"filter-members": {
			servers: testServersProvider{
				makeServer("primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeServer("primary", structs.ACLModeLegacy, serf.StatusFailed, net.IP([]byte{127, 0, 0, 2})),
				// filtered datacenter
				makeServer("secondary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 4})),
				// filtered status
				makeServer("primary", structs.ACLModeUnknown, serf.StatusLeaving, net.IP([]byte{127, 0, 0, 5})),
				// filtered status
				makeServer("primary", structs.ACLModeUnknown, serf.StatusLeft, net.IP([]byte{127, 0, 0, 6})),
				// filtered status
				makeServer("primary", structs.ACLModeUnknown, serf.StatusNone, net.IP([]byte{127, 0, 0, 7})),
			},
			foundServers: true,
			leaderAddr:   "127.0.0.1:10000",
			datacenter:   "primary",
			minMode:      structs.ACLModeLegacy,
			leaderMode:   structs.ACLModeLegacy,
		},
		"disabled": {
			servers: testServersProvider{
				makeServer("primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeServer("primary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
				makeServer("primary", structs.ACLModeDisabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 3})),
			},
			foundServers: true,
			leaderAddr:   "127.0.0.1:10000",
			datacenter:   "primary",
			minMode:      structs.ACLModeDisabled,
			leaderMode:   structs.ACLModeLegacy,
		},
		"unknown": {
			servers: testServersProvider{
				makeServer("primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeServer("primary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
			},
			foundServers: true,
			leaderAddr:   "127.0.0.1:10000",
			datacenter:   "primary",
			minMode:      structs.ACLModeUnknown,
			leaderMode:   structs.ACLModeLegacy,
		},
		"legacy": {
			servers: testServersProvider{
				makeServer("primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeServer("primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
			},
			foundServers: true,
			leaderAddr:   "127.0.0.1:10000",
			datacenter:   "primary",
			minMode:      structs.ACLModeLegacy,
			leaderMode:   structs.ACLModeEnabled,
		},
		"enabled": {
			servers: testServersProvider{
				makeServer("primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeServer("primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
				makeServer("primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 3})),
			},
			foundServers: true,
			leaderAddr:   "127.0.0.1:10000",
			datacenter:   "primary",
			minMode:      structs.ACLModeEnabled,
			leaderMode:   structs.ACLModeEnabled,
		},
		"failed-members": {
			servers: testServersProvider{
				makeServer("primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeServer("primary", structs.ACLModeUnknown, serf.StatusFailed, net.IP([]byte{127, 0, 0, 2})),
				makeServer("primary", structs.ACLModeLegacy, serf.StatusFailed, net.IP([]byte{127, 0, 0, 3})),
			},
			foundServers: true,
			leaderAddr:   "127.0.0.1:10000",
			datacenter:   "primary",
			minMode:      structs.ACLModeUnknown,
			leaderMode:   structs.ACLModeLegacy,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			actualServers, actualMinMode, actualLeaderMode := ServersGetACLMode(tc.servers, tc.leaderAddr, tc.datacenter)

			require.Equal(t, tc.minMode, actualMinMode)
			require.Equal(t, tc.leaderMode, actualLeaderMode)
			require.Equal(t, tc.foundServers, actualServers)
		})
	}
}

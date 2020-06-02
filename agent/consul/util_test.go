package consul

import (
	"errors"
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

func TestGetPrivateIP(t *testing.T) {
	t.Parallel()
	ip, _, err := net.ParseCIDR("10.1.2.3/32")
	if err != nil {
		t.Fatalf("failed to parse private cidr: %v", err)
	}

	pubIP, _, err := net.ParseCIDR("8.8.8.8/32")
	if err != nil {
		t.Fatalf("failed to parse public cidr: %v", err)
	}

	tests := []struct {
		addrs    []net.Addr
		expected net.IP
		err      error
	}{
		{
			addrs: []net.Addr{
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: pubIP,
				},
			},
			expected: ip,
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{
					IP: pubIP,
				},
			},
			err: errors.New("No private IP address found"),
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: pubIP,
				},
			},
			err: errors.New("Multiple private IPs found. Please configure one."),
		},
	}

	for _, test := range tests {
		ip, err := getPrivateIP(test.addrs)
		switch {
		case test.err != nil && err != nil:
			if err.Error() != test.err.Error() {
				t.Fatalf("unexpected error: %v != %v", test.err, err)
			}
		case (test.err == nil && err != nil) || (test.err != nil && err == nil):
			t.Fatalf("unexpected error: %v != %v", test.err, err)
		default:
			if !test.expected.Equal(ip) {
				t.Fatalf("unexpected ip: %v != %v", ip, test.expected)
			}
		}
	}
}

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()
	if !isPrivateIP("192.168.1.1") {
		t.Fatalf("bad")
	}
	if !isPrivateIP("172.16.45.100") {
		t.Fatalf("bad")
	}
	if !isPrivateIP("10.1.2.3") {
		t.Fatalf("bad")
	}
	if !isPrivateIP("100.115.110.19") {
		t.Fatalf("bad")
	}
	if isPrivateIP("8.8.8.8") {
		t.Fatalf("bad")
	}
	if !isPrivateIP("127.0.0.1") {
		t.Fatalf("bad")
	}
}

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

func TestByteConversion(t *testing.T) {
	t.Parallel()
	var val uint64 = 2 << 50
	raw := uint64ToBytes(val)
	if bytesToUint64(raw) != val {
		t.Fatalf("no match")
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

func TestGetPublicIPv6(t *testing.T) {
	t.Parallel()
	ip, _, err := net.ParseCIDR("fe80::1/128")
	if err != nil {
		t.Fatalf("failed to parse link-local cidr: %v", err)
	}

	ip2, _, err := net.ParseCIDR("::1/128")
	if err != nil {
		t.Fatalf("failed to parse loopback cidr: %v", err)
	}

	ip3, _, err := net.ParseCIDR("fc00::1/128")
	if err != nil {
		t.Fatalf("failed to parse ULA cidr: %v", err)
	}

	pubIP, _, err := net.ParseCIDR("2001:0db8:85a3::8a2e:0370:7334/128")
	if err != nil {
		t.Fatalf("failed to parse public cidr: %v", err)
	}

	tests := []struct {
		addrs    []net.Addr
		expected net.IP
		err      error
	}{
		{
			addrs: []net.Addr{
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: ip2,
				},
				&net.IPAddr{
					IP: ip3,
				},
				&net.IPAddr{
					IP: pubIP,
				},
			},
			expected: pubIP,
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: ip2,
				},
				&net.IPAddr{
					IP: ip3,
				},
			},
			err: errors.New("No public IPv6 address found"),
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: ip,
				},
				&net.IPAddr{
					IP: pubIP,
				},
				&net.IPAddr{
					IP: pubIP,
				},
			},
			err: errors.New("Multiple public IPv6 addresses found. Please configure one."),
		},
	}

	for _, test := range tests {
		ip, err := getPublicIPv6(test.addrs)
		switch {
		case test.err != nil && err != nil:
			if err.Error() != test.err.Error() {
				t.Fatalf("unexpected error: %v != %v", test.err, err)
			}
		case (test.err == nil && err != nil) || (test.err != nil && err == nil):
			t.Fatalf("unexpected error: %v != %v", test.err, err)
		default:
			if !test.expected.Equal(ip) {
				t.Fatalf("unexpected ip: %v != %v", ip, test.expected)
			}
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
		"filter-members": tcase{
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
		"disabled": tcase{
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
		"unknown": tcase{
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
		"legacy": tcase{
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
		"enabled": tcase{
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
		"failed-members": tcase{
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

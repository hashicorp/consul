package consul

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"testing"

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
	for i := 0; i < 100; i++ {
		id := generateUUID()
		if prev == id {
			t.Fatalf("Should get a new ID!")
		}

		matched, err := regexp.MatchString(
			"[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}", id)
		if !matched || err != nil {
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

func TestServersMeetMinimumVersion(t *testing.T) {
	t.Parallel()
	makeMember := func(version string) serf.Member {
		return serf.Member{
			Name: "foo",
			Addr: net.IP([]byte{127, 0, 0, 1}),
			Tags: map[string]string{
				"role":          "consul",
				"id":            "asdf",
				"dc":            "east-aws",
				"port":          "10000",
				"build":         version,
				"wan_join_port": "1234",
				"vsn":           "1",
				"expect":        "3",
				"raft_vsn":      "3",
			},
			Status: serf.StatusAlive,
		}
	}

	cases := []struct {
		members  []serf.Member
		ver      *version.Version
		expected bool
	}{
		// One server, meets reqs
		{
			members: []serf.Member{
				makeMember("0.7.5"),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// One server, doesn't meet reqs
		{
			members: []serf.Member{
				makeMember("0.7.5"),
			},
			ver:      version.Must(version.NewVersion("0.8.0")),
			expected: false,
		},
		// Multiple servers, meets req version
		{
			members: []serf.Member{
				makeMember("0.7.5"),
				makeMember("0.8.0"),
			},
			ver:      version.Must(version.NewVersion("0.7.5")),
			expected: true,
		},
		// Multiple servers, doesn't meet req version
		{
			members: []serf.Member{
				makeMember("0.7.5"),
				makeMember("0.8.0"),
			},
			ver:      version.Must(version.NewVersion("0.8.0")),
			expected: false,
		},
	}

	for _, tc := range cases {
		result := ServersMeetMinimumVersion(tc.members, tc.ver)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.ver.String(), tc)
		}
	}
}

func TestServersInDCMeetMinimumVersion(t *testing.T) {
	t.Parallel()
	makeMember := func(version string, datacenter string) serf.Member {
		return serf.Member{
			Name: "foo",
			Addr: net.IP([]byte{127, 0, 0, 1}),
			Tags: map[string]string{
				"role":          "consul",
				"id":            "asdf",
				"dc":            datacenter,
				"port":          "10000",
				"build":         version,
				"wan_join_port": "1234",
				"vsn":           "1",
				"expect":        "3",
				"raft_vsn":      "3",
			},
			Status: serf.StatusAlive,
		}
	}

	cases := []struct {
		members       []serf.Member
		ver           *version.Version
		expected      bool
		expectedFound bool
	}{
		// One server, meets reqs
		{
			members: []serf.Member{
				makeMember("0.7.5", "primary"),
				makeMember("0.7.3", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.7.5")),
			expected:      true,
			expectedFound: true,
		},
		// One server, doesn't meet reqs
		{
			members: []serf.Member{
				makeMember("0.7.5", "primary"),
				makeMember("0.8.1", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.8.0")),
			expected:      false,
			expectedFound: true,
		},
		// Multiple servers, meets req version
		{
			members: []serf.Member{
				makeMember("0.7.5", "primary"),
				makeMember("0.8.0", "primary"),
				makeMember("0.7.0", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.7.5")),
			expected:      true,
			expectedFound: true,
		},
		// Multiple servers, doesn't meet req version
		{
			members: []serf.Member{
				makeMember("0.7.5", "primary"),
				makeMember("0.8.0", "primary"),
				makeMember("0.9.1", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.8.0")),
			expected:      false,
			expectedFound: true,
		},
		{
			members: []serf.Member{
				makeMember("0.7.5", "secondary"),
				makeMember("0.8.0", "secondary"),
				makeMember("0.9.1", "secondary"),
			},
			ver:           version.Must(version.NewVersion("0.7.0")),
			expected:      true,
			expectedFound: false,
		},
	}

	for _, tc := range cases {
		result, found := ServersInDCMeetMinimumVersion(tc.members, "primary", tc.ver)
		require.Equal(t, tc.expected, result)
		require.Equal(t, tc.expectedFound, found)
	}
}

func TestInterpolateHIL(t *testing.T) {
	for _, test := range []struct {
		name string
		in   string
		vars map[string]string
		exp  string
		ok   bool
	}{
		// valid HIL
		{
			"empty",
			"",
			map[string]string{},
			"",
			true,
		},
		{
			"no vars",
			"nothing",
			map[string]string{},
			"nothing",
			true,
		},
		{
			"just var",
			"${item}",
			map[string]string{"item": "value"},
			"value",
			true,
		},
		{
			"var in middle",
			"before ${item}after",
			map[string]string{"item": "value"},
			"before valueafter",
			true,
		},
		{
			"two vars",
			"before ${item}after ${more}",
			map[string]string{"item": "value", "more": "xyz"},
			"before valueafter xyz",
			true,
		},
		{
			"missing map val",
			"${item}",
			map[string]string{"item": ""},
			"",
			true,
		},
		// "weird" HIL, but not technically invalid
		{
			"just end",
			"}",
			map[string]string{},
			"}",
			true,
		},
		{
			"var without start",
			" item }",
			map[string]string{"item": "value"},
			" item }",
			true,
		},
		{
			"two vars missing second start",
			"before ${ item }after  more }",
			map[string]string{"item": "value", "more": "xyz"},
			"before valueafter  more }",
			true,
		},
		// invalid HIL
		{
			"just start",
			"${",
			map[string]string{},
			"",
			false,
		},
		{
			"backwards",
			"}${",
			map[string]string{},
			"",
			false,
		},
		{
			"no varname",
			"${}",
			map[string]string{},
			"",
			false,
		},
		{
			"missing map key",
			"${item}",
			map[string]string{},
			"",
			false,
		},
		{
			"var without end",
			"${ item ",
			map[string]string{"item": "value"},
			"",
			false,
		},
		{
			"two vars missing first end",
			"before ${ item after ${ more }",
			map[string]string{"item": "value", "more": "xyz"},
			"",
			false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := InterpolateHIL(test.in, test.vars)
			if test.ok {
				require.NoError(t, err)
				require.Equal(t, test.exp, out)
			} else {
				require.NotNil(t, err)
				require.Equal(t, out, "")
			}
		})
	}
}

func TestServersGetACLMode(t *testing.T) {
	t.Parallel()
	makeMember := func(role string, datacenter string, acls structs.ACLMode, status serf.MemberStatus, addr net.IP) serf.Member {
		return serf.Member{
			Name: "foo",
			Addr: addr,
			Tags: map[string]string{
				"role":          role,
				"id":            "asdf",
				"dc":            datacenter,
				"port":          "10000",
				"build":         "1.6.0",
				"wan_join_port": "1234",
				"vsn":           "1",
				"expect":        "3",
				"raft_vsn":      "3",
				"acls":          string(acls),
			},
			Status: status,
		}
	}

	type tcase struct {
		members    []serf.Member
		leaderAddr string
		datacenter string
		numServers int
		minMode    structs.ACLMode
		leaderMode structs.ACLMode
	}

	cases := map[string]tcase{
		"filter-members": tcase{
			members: []serf.Member{
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusFailed, net.IP([]byte{127, 0, 0, 2})),
				// filter non-server
				makeMember("client", "primary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 3})),
				// filtered datacenter
				makeMember("consul", "secondary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 4})),
				// filtered status
				makeMember("consul", "primary", structs.ACLModeUnknown, serf.StatusLeaving, net.IP([]byte{127, 0, 0, 5})),
				// filtered status
				makeMember("consul", "primary", structs.ACLModeUnknown, serf.StatusLeft, net.IP([]byte{127, 0, 0, 6})),
				// filtered status
				makeMember("consul", "primary", structs.ACLModeUnknown, serf.StatusNone, net.IP([]byte{127, 0, 0, 7})),
			},
			numServers: 2,
			leaderAddr: "127.0.0.1:10000",
			datacenter: "primary",
			minMode:    structs.ACLModeLegacy,
			leaderMode: structs.ACLModeLegacy,
		},
		"disabled": tcase{
			members: []serf.Member{
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeMember("consul", "primary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
				makeMember("consul", "primary", structs.ACLModeDisabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 3})),
			},
			numServers: 3,
			leaderAddr: "127.0.0.1:10000",
			datacenter: "",
			minMode:    structs.ACLModeDisabled,
			leaderMode: structs.ACLModeLegacy,
		},
		"unknown": tcase{
			members: []serf.Member{
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeMember("consul", "primary", structs.ACLModeUnknown, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
			},
			numServers: 2,
			leaderAddr: "127.0.0.1:10000",
			datacenter: "",
			minMode:    structs.ACLModeUnknown,
			leaderMode: structs.ACLModeLegacy,
		},
		"legacy": tcase{
			members: []serf.Member{
				makeMember("consul", "primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
			},
			numServers: 2,
			leaderAddr: "127.0.0.1:10000",
			datacenter: "",
			minMode:    structs.ACLModeLegacy,
			leaderMode: structs.ACLModeEnabled,
		},
		"enabled": tcase{
			members: []serf.Member{
				makeMember("consul", "primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeMember("consul", "primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 2})),
				makeMember("consul", "primary", structs.ACLModeEnabled, serf.StatusAlive, net.IP([]byte{127, 0, 0, 3})),
			},
			numServers: 3,
			leaderAddr: "127.0.0.1:10000",
			datacenter: "",
			minMode:    structs.ACLModeEnabled,
			leaderMode: structs.ACLModeEnabled,
		},
		"failed-members": tcase{
			members: []serf.Member{
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusAlive, net.IP([]byte{127, 0, 0, 1})),
				makeMember("consul", "primary", structs.ACLModeUnknown, serf.StatusFailed, net.IP([]byte{127, 0, 0, 2})),
				makeMember("consul", "primary", structs.ACLModeLegacy, serf.StatusFailed, net.IP([]byte{127, 0, 0, 3})),
			},
			numServers: 3,
			leaderAddr: "127.0.0.1:10000",
			datacenter: "",
			minMode:    structs.ACLModeUnknown,
			leaderMode: structs.ACLModeLegacy,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			actualServers, actualMinMode, actualLeaderMode := ServersGetACLMode(tc.members, tc.leaderAddr, tc.datacenter)

			require.Equal(t, tc.numServers, actualServers)
			require.Equal(t, tc.minMode, actualMinMode)
			require.Equal(t, tc.leaderMode, actualLeaderMode)
		})
	}
}

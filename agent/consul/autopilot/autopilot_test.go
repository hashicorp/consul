package autopilot

import (
	"errors"
	"net"
	"testing"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
)

func TestMinRaftProtocol(t *testing.T) {
	t.Parallel()
	makeMember := func(version string) serf.Member {
		return serf.Member{
			Name: "foo",
			Addr: net.IP([]byte{127, 0, 0, 1}),
			Tags: map[string]string{
				"role":     "consul",
				"dc":       "dc1",
				"port":     "10000",
				"vsn":      "1",
				"raft_vsn": version,
			},
			Status: serf.StatusAlive,
		}
	}

	cases := []struct {
		members  []serf.Member
		expected int
		err      error
	}{
		// No servers, error
		{
			members:  []serf.Member{},
			expected: -1,
			err:      errors.New("No servers found"),
		},
		// One server
		{
			members: []serf.Member{
				makeMember("1"),
			},
			expected: 1,
		},
		// One server, bad version formatting
		{
			members: []serf.Member{
				makeMember("asdf"),
			},
			expected: -1,
			err:      errors.New(`strconv.Atoi: parsing "asdf": invalid syntax`),
		},
		// Multiple servers, different versions
		{
			members: []serf.Member{
				makeMember("1"),
				makeMember("2"),
			},
			expected: 1,
		},
		// Multiple servers, same version
		{
			members: []serf.Member{
				makeMember("2"),
				makeMember("2"),
			},
			expected: 2,
		},
	}

	serverFunc := func(m serf.Member) (*ServerInfo, error) {
		return &ServerInfo{}, nil
	}
	for _, tc := range cases {
		result, err := minRaftProtocol(tc.members, serverFunc)
		if result != tc.expected {
			t.Fatalf("bad: %v, %v, %v", result, tc.expected, tc)
		}
		if tc.err != nil {
			if err == nil || tc.err.Error() != err.Error() {
				t.Fatalf("bad: %v, %v, %v", err, tc.err, tc)
			}
		}
	}
}

func TestAutopilot_canRemoveServers(t *testing.T) {
	type test struct {
		peers       int
		minQuorum   int
		deadServers int
		ok          bool
	}

	tests := []test{
		{1, 1, 1, false},
		{3, 3, 1, false},
		{4, 3, 3, false},
		{5, 3, 3, false},
		{5, 3, 2, true},
		{5, 3, 1, true},
		{9, 3, 5, false},
	}
	for _, test := range tests {
		ok, msg := canRemoveServers(test.peers, test.minQuorum, test.deadServers)
		require.Equal(t, test.ok, ok)
		t.Logf("%+v: %s", test, msg)
	}
}

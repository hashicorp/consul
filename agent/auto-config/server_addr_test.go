package autoconf

import (
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

type testAddr struct {
	Host string
	Port int
}

func makeIPHelper(address string, port int) net.TCPAddr {
	return net.TCPAddr{IP: net.ParseIP(address), Port: port}
}
func makeIPArrayHelper(addr []testAddr) []net.TCPAddr {
	if addr == nil {
		return nil
	}
	tcp := make([]net.TCPAddr, len(addr))
	for i, v := range addr {
		tcp[i] = makeIPHelper(v.Host, v.Port)
	}
	return tcp
}

func TestResolv(t *testing.T) {

	tests := []struct {
		address string
		hosts   []testAddr
	}{
		//	{"example.org", "example.org", -1, false},
		//	{"example.org:10", "example.org", 10, false},
		{"example.org:10:10", nil},

		// IPv4 addresses
		{"10.0.0.1", []testAddr{{Host: "10.0.0.1", Port: 1234}}},
		{"10.0.0.1:10", []testAddr{{Host: "10.0.0.1", Port: 10}}},
		{"10.0.0.1:10:4", nil},

		// IPv6 addresses w/o port
		{"::1", []testAddr{{"::1", 1234}}},
		{"fe80::1", []testAddr{{"fe80::1", 1234}}},
		{"2001:db8::1", []testAddr{{"2001:db8::1", 1234}}},
		{"2a05:d014:d9e:c303:e4d3:d281:a61d:8ebd", []testAddr{{"2a05:d014:d9e:c303:e4d3:d281:a61d:8ebd", 1234}}},

		//
		//// well formed IPv6 addresses with port
		{"[::1]:10", []testAddr{{"::1", 10}}},
		{"[fe80::1]:10", []testAddr{{"fe80::1", 10}}},
		{"[2001:db8::1]:10", []testAddr{{"2001:db8::1", 10}}},

		// Various wierd IPv6 shaped things should do the right thing
		{"2001:db8:1:2:3:4:5:6:7:8", nil},
		{"[::ffff:172.16.5.4]", []testAddr{{"::ffff:172.16.5.4", 1234}}},
		{"[::ffff:172.16.5.4]:10", []testAddr{{"::ffff:172.16.5.4", 10}}},
		{"2001:db8:1:2:3:4:5:6:7", []testAddr{{"2001:db8:1:2:3:4:5:6", 7}}},
	}

	datacenter := "foo"
	nodeName := "bar"

	mcfg := newMockedConfig(t)

	ac := AutoConfig{
		config: &config.RuntimeConfig{
			Datacenter: datacenter,
			NodeName:   nodeName,
			RetryJoinLAN: []string{
				"198.18.0.1:1234", "198.18.0.2:3456",
			},
			ServerPort: 1234,
		},
		acConfig: mcfg.Config,
		logger:   testutil.Logger(t),
	}

	require.Equal(t, ac.config.ServerPort, 1234)

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			host := ac.resolveHost(tt.address)

			hostTCP := makeIPArrayHelper(tt.hosts)
			require.Equal(t, hostTCP, host)
		})
	}
}

package resolver

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
)

func TestServerResolverBuilder(t *testing.T) {
	const agentDC = "dc1"

	type testcase struct {
		name       string
		agentType  string // server/client
		serverType string // server/leader
		requestDC  string
		expectLAN  bool
	}

	run := func(t *testing.T, tc testcase) {
		rs := NewServerResolverBuilder(newConfig(t, agentDC, tc.agentType))

		endpoint := ""
		if tc.serverType == "leader" {
			endpoint = "leader.local"
		} else {
			endpoint = tc.serverType + "." + tc.requestDC
		}

		cc := &fakeClientConn{}
		_, err := rs.Build(resolver.Target{
			Scheme:    "consul",
			Authority: rs.Authority(),
			URL:       url.URL{Opaque: endpoint},
		}, cc, resolver.BuildOptions{})
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			dc := fmt.Sprintf("dc%d", i+1)
			for j := 0; j < 3; j++ {
				wanIP := fmt.Sprintf("127.1.%d.%d", i+1, j+10)
				name := fmt.Sprintf("%s-server-%d", dc, j+1)
				wanMeta := newServerMeta(name, dc, wanIP, true)

				if tc.agentType == "server" {
					rs.AddServer(types.AreaWAN, wanMeta)
				}

				if dc == agentDC {
					// register LAN/WAN pairs for the same instances
					lanIP := fmt.Sprintf("127.0.%d.%d", i+1, j+10)
					lanMeta := newServerMeta(name, dc, lanIP, false)
					rs.AddServer(types.AreaLAN, lanMeta)

					if j == 0 {
						rs.UpdateLeaderAddr(dc, lanIP)
					}
				}
			}
		}

		if tc.serverType == "leader" {
			assert.Len(t, cc.state.Addresses, 1)
		} else {
			assert.Len(t, cc.state.Addresses, 3)
		}

		for _, addr := range cc.state.Addresses {
			addrPrefix := tc.requestDC + "-"
			if tc.expectLAN {
				addrPrefix += "127.0."
			} else {
				addrPrefix += "127.1."
			}
			assert.True(t, strings.HasPrefix(addr.Addr, addrPrefix),
				"%q does not start with %q (returned WAN for LAN request)", addr.Addr, addrPrefix)

			if tc.expectLAN {
				assert.False(t, strings.Contains(addr.ServerName, ".dc"),
					"%q ends with datacenter suffix (returned WAN for LAN request)", addr.ServerName)
			} else {
				assert.True(t, strings.HasSuffix(addr.ServerName, "."+tc.requestDC),
					"%q does not end with %q", addr.ServerName, "."+tc.requestDC)
			}
		}
	}

	cases := []testcase{
		{
			name:       "server requesting local servers",
			agentType:  "server",
			serverType: "server",
			requestDC:  agentDC,
			expectLAN:  true,
		},
		{
			name:       "server requesting remote servers in dc2",
			agentType:  "server",
			serverType: "server",
			requestDC:  "dc2",
			expectLAN:  false,
		},
		{
			name:       "server requesting remote servers in dc3",
			agentType:  "server",
			serverType: "server",
			requestDC:  "dc3",
			expectLAN:  false,
		},
		// ---------------
		{
			name:       "server requesting local leader",
			agentType:  "server",
			serverType: "leader",
			requestDC:  agentDC,
			expectLAN:  true,
		},
		// ---------------
		{
			name:       "client requesting local server",
			agentType:  "client",
			serverType: "server",
			requestDC:  agentDC,
			expectLAN:  true,
		},
		{
			name:       "client requesting local leader",
			agentType:  "client",
			serverType: "leader",
			requestDC:  agentDC,
			expectLAN:  true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func newServerMeta(name, dc, ip string, wan bool) *metadata.Server {
	fullname := name
	if wan {
		fullname = name + "." + dc
	}
	return &metadata.Server{
		ID:         name,
		Name:       fullname,
		ShortName:  name,
		Datacenter: dc,
		Addr:       &net.IPAddr{IP: net.ParseIP(ip)},
		UseTLS:     false,
	}
}

func newConfig(t *testing.T, dc, agentType string) Config {
	n := t.Name()
	s := strings.Replace(n, "/", "", -1)
	s = strings.Replace(s, "_", "", -1)
	return Config{
		Datacenter: dc,
		AgentType:  agentType,
		Authority:  strings.ToLower(s),
	}
}

// fakeClientConn implements resolver.ClientConn for tests
type fakeClientConn struct {
	state resolver.State
}

var _ resolver.ClientConn = (*fakeClientConn)(nil)

func (f *fakeClientConn) UpdateState(state resolver.State) error {
	f.state = state
	return nil
}

func (*fakeClientConn) ReportError(error)                       {}
func (*fakeClientConn) NewAddress(addresses []resolver.Address) {}
func (*fakeClientConn) NewServiceConfig(serviceConfig string)   {}
func (*fakeClientConn) ParseServiceConfig(serviceConfigJSON string) *serviceconfig.ParseResult {
	return nil
}

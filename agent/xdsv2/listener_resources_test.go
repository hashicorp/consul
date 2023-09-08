package xdsv2

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMakeEnvoyFilterChainMatch(t *testing.T) {
	cases := map[string]struct {
		input          *pbproxystate.Match
		expectedOutput *envoy_listener_v3.FilterChainMatch
	}{
		"server names": {
			input: &pbproxystate.Match{
				DestinationPort: response.MakeUint32Value(8080),
				ServerNames:     []string{"server-1", "server-2"},
			},
			expectedOutput: &envoy_listener_v3.FilterChainMatch{
				DestinationPort: response.MakeUint32Value(8080),
				ServerNames:     []string{"server-1", "server-2"},
			},
		},
		"prefix ranges": {
			input: &pbproxystate.Match{
				DestinationPort: response.MakeUint32Value(8080),
				PrefixRanges: []*pbproxystate.CidrRange{
					{
						AddressPrefix: "192.168.0.1",
						PrefixLen:     response.MakeUint32Value(16),
					},
				},
			},
			expectedOutput: &envoy_listener_v3.FilterChainMatch{
				DestinationPort: response.MakeUint32Value(8080),
				PrefixRanges: []*envoy_core_v3.CidrRange{
					{
						AddressPrefix: "192.168.0.1",
						PrefixLen:     response.MakeUint32Value(16),
					},
				},
			},
		},
		"source prefix ranges": {
			input: &pbproxystate.Match{
				DestinationPort: response.MakeUint32Value(8080),
				SourcePrefixRanges: []*pbproxystate.CidrRange{
					{
						AddressPrefix: "192.168.0.1",
						PrefixLen:     response.MakeUint32Value(16),
					},
				},
			},
			expectedOutput: &envoy_listener_v3.FilterChainMatch{
				DestinationPort: response.MakeUint32Value(8080),
				SourcePrefixRanges: []*envoy_core_v3.CidrRange{
					{
						AddressPrefix: "192.168.0.1",
						PrefixLen:     response.MakeUint32Value(16),
					},
				},
			},
		},
		"alpn protocols": {
			input: &pbproxystate.Match{
				DestinationPort: response.MakeUint32Value(8080),
				AlpnProtocols:   []string{"http", "http2"},
			},
			expectedOutput: &envoy_listener_v3.FilterChainMatch{
				DestinationPort:      response.MakeUint32Value(8080),
				ApplicationProtocols: []string{"http", "http2"},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.expectedOutput, makeEnvoyFilterChainMatch(c.input))
		})
	}
}

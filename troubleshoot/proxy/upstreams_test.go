package troubleshoot

import (
	"io"
	"os"
	"testing"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGetUpstreamIPsFromFilterChain(t *testing.T) {
	file, err := os.Open("testdata/listeners.json")
	require.NoError(t, err)
	jsonBytes, err := io.ReadAll(file)
	require.NoError(t, err)

	expected := []UpstreamIP{
		{
			IPs: []string{
				"10.244.0.63",
				"10.244.0.64",
			},
			IsVirtual: false,
			ClusterNames: map[string]struct{}{
				"passthrough~foo.default.dc1.internal.dc1.consul": {},
			},
		},
		{
			IPs: []string{
				"10.96.5.96",
				"240.0.0.1",
			},
			IsVirtual: true,
			ClusterNames: map[string]struct{}{
				"foo.default.dc1.internal.dc1.consul": {},
			},
		},
	}

	var listener envoy_listener_v3.Listener
	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = unmarshal.Unmarshal(jsonBytes, &listener)
	require.NoError(t, err)

	upstream_ips, err := getUpstreamIPsFromFilterChain(listener.GetFilterChains())
	require.NoError(t, err)

	require.Equal(t, expected, upstream_ips)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package troubleshoot

import (
	"io"
	"os"
	"testing"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestGetUpstreamIPsFromFilterChain(t *testing.T) {
	file, err := os.Open("testdata/upstreams/config.json")
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
		{
			IPs: []string{
				"10.4.6.160",
				"240.0.0.3",
			},
			IsVirtual: true,
			ClusterNames: map[string]struct{}{
				"backend.default.dc1.internal.domain.consul":  {},
				"backend2.default.dc1.internal.domain.consul": {},
			},
		},
	}

	var upstreamIPs []UpstreamIP
	cfgDump := &envoy_admin_v3.ConfigDump{}
	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = unmarshal.Unmarshal(jsonBytes, cfgDump)
	require.NoError(t, err)

	for _, cfg := range cfgDump.Configs {
		switch cfg.TypeUrl {
		case listenersType:
			lcd := &envoy_admin_v3.ListenersConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), lcd)
			require.NoError(t, err)

			for _, listener := range lcd.GetDynamicListeners() {
				l := &envoy_listener_v3.Listener{}
				err = proto.Unmarshal(listener.GetActiveState().GetListener().GetValue(), l)
				require.NoError(t, err)

				upstreamIPs, err = getUpstreamIPsFromFilterChain(l.GetFilterChains(), cfgDump)
				require.NoError(t, err)

			}
		}
	}

	require.NoError(t, err)
	require.Equal(t, expected, upstreamIPs)
}

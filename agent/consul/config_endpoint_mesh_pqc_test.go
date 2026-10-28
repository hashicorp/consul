// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestMeshEntryConfiguresECDHCurves(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		entry structs.ConfigEntry
		want  bool
	}{
		"non-mesh entry": {
			entry: &structs.ServiceConfigEntry{Kind: structs.ServiceDefaults, Name: "web"},
			want:  false,
		},
		"mesh entry without TLS": {
			entry: &structs.MeshConfigEntry{},
			want:  false,
		},
		"mesh entry with TLS but no directions": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{}},
			want:  false,
		},
		"mesh entry with TLS version only": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Incoming: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_3},
			}},
			want: false,
		},
		"mesh entry with empty curve list": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Outgoing: &structs.MeshDirectionalTLSConfig{ECDHCurves: []string{}},
			}},
			want: false,
		},
		"mesh entry with incoming curves": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Incoming: &structs.MeshDirectionalTLSConfig{
					TLSMinVersion: types.TLSv1_3,
					ECDHCurves:    []string{"X25519MLKEM768"},
				},
			}},
			want: true,
		},
		"mesh entry with outgoing curves": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Outgoing: &structs.MeshDirectionalTLSConfig{
					TLSMinVersion: types.TLSv1_3,
					ECDHCurves:    []string{"X25519"},
				},
			}},
			want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, meshEntryConfiguresECDHCurves(tc.entry))
		})
	}
}

func TestMeshEntryEnablesPQC(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		entry structs.ConfigEntry
		want  bool
	}{
		"non-mesh entry": {
			entry: &structs.ProxyConfigEntry{Kind: structs.ProxyDefaults, Name: structs.ProxyConfigGlobal},
			want:  false,
		},
		"mesh entry without TLS": {
			entry: &structs.MeshConfigEntry{},
			want:  false,
		},
		"TLSv1_2 minimum without curves": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Incoming: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_2},
			}},
			want: false,
		},
		"TLS_AUTO minimum without curves": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Outgoing: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSVersionAuto},
			}},
			want: false,
		},
		"TLSv1_3 incoming minimum triggers automatic PQC": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Incoming: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_3},
			}},
			want: true,
		},
		"TLSv1_3 outgoing minimum triggers automatic PQC": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Outgoing: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_3},
			}},
			want: true,
		},
		"explicit curves": {
			entry: &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
				Outgoing: &structs.MeshDirectionalTLSConfig{
					TLSMinVersion: types.TLSv1_2,
					ECDHCurves:    []string{"X25519MLKEM768", "P-256"},
				},
			}},
			want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, meshEntryEnablesPQC(tc.entry))
		})
	}
}

func TestServersSupportMeshECDHCurves(t *testing.T) {
	t.Parallel()

	makeServer := func(name, build, dc string, status serf.MemberStatus) metadata.Server {
		return metadata.Server{
			Name:       name,
			ShortName:  name,
			Status:     status,
			Build:      *version.Must(version.NewVersion(build)),
			Datacenter: dc,
		}
	}

	cases := map[string]struct {
		servers testServersProvider
		wantErr bool
	}{
		"all servers on 2.1.0": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.1.0", "dc1", serf.StatusAlive),
			},
		},
		"servers on newer versions": {
			servers: testServersProvider{
				makeServer("s1", "2.1.3", "dc1", serf.StatusAlive),
				makeServer("s2", "3.0.0", "dc1", serf.StatusAlive),
			},
		},
		"2.1.0 pre-release servers are supported": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0-rc1", "dc1", serf.StatusAlive),
				makeServer("s2", "3.0.0-dev", "dc1", serf.StatusAlive),
			},
		},
		"one alive server on 2.0.x": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc1", serf.StatusAlive),
			},
			wantErr: true,
		},
		"failed server on 2.0.x still blocks": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc1", serf.StatusFailed),
			},
			wantErr: true,
		},
		"left server on 2.0.x is ignored": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc1", serf.StatusLeft),
			},
		},
		"old servers in other datacenters are ignored": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc2", serf.StatusAlive),
			},
		},
		"no servers known in datacenter": {
			servers: testServersProvider{
				makeServer("s2", "2.0.4", "dc2", serf.StatusAlive),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := serversSupportMeshECDHCurves(tc.servers, "dc1")
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "all servers in the datacenter must run Consul 2.1.0 or later before configuring ECDHCurves")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAgentsWithoutMeshPQCSupport(t *testing.T) {
	t.Parallel()

	member := func(name, role, build string, status serf.MemberStatus) serf.Member {
		return serf.Member{
			Name:   name,
			Status: status,
			Tags:   map[string]string{"role": role, "build": build},
		}
	}

	cases := map[string]struct {
		members []serf.Member
		want    []string
	}{
		"no members": {
			members: nil,
			want:    nil,
		},
		"all agents upgraded": {
			members: []serf.Member{
				member("server-1", "consul", "2.1.0:abcdef", serf.StatusAlive),
				member("client-1", "node", "2.1.0+ent:abcdef", serf.StatusAlive),
				member("client-2", "node", "3.0.0-dev:abcdef", serf.StatusAlive),
			},
			want: nil,
		},
		"pre-release 2.1.0 agents are supported": {
			members: []serf.Member{
				member("client-1", "node", "2.1.0-rc2:abcdef", serf.StatusAlive),
			},
			want: nil,
		},
		"old clients and servers are reported sorted": {
			members: []serf.Member{
				member("client-b", "node", "2.0.4:abcdef", serf.StatusAlive),
				member("server-1", "consul", "2.1.0:abcdef", serf.StatusAlive),
				member("client-a", "node", "1.22.3+ent:abcdef", serf.StatusAlive),
				member("server-2", "consul", "2.0.4:abcdef", serf.StatusAlive),
			},
			want: []string{"client-a", "client-b", "server-2"},
		},
		"non-alive members are ignored": {
			members: []serf.Member{
				member("client-left", "node", "2.0.4:abcdef", serf.StatusLeft),
				member("client-failed", "node", "2.0.4:abcdef", serf.StatusFailed),
			},
			want: nil,
		},
		"members that are not Consul agents are ignored": {
			members: []serf.Member{
				member("other", "", "2.0.4:abcdef", serf.StatusAlive),
			},
			want: nil,
		},
		"members with unparsable build are ignored": {
			members: []serf.Member{
				member("client-1", "node", "", serf.StatusAlive),
				member("client-2", "node", "garbage", serf.StatusAlive),
			},
			want: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, agentsWithoutMeshPQCSupport(tc.members))
		})
	}
}

func TestFormatAgentsWithoutMeshPQCSupport(t *testing.T) {
	t.Parallel()

	require.Equal(t, "a, b", formatNodeNames([]string{"a", "b"}, 5))

	var many []string
	for i := 0; i < 12; i++ {
		many = append(many, fmt.Sprintf("n%02d", i))
	}
	require.Equal(t, "n00, n01, n02 (and 9 more)", formatNodeNames(many, 3))
}

func TestConfigEntry_Apply_MeshECDHCurvesServerVersionGate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	meshWithCurves := &structs.MeshConfigEntry{
		TLS: &structs.MeshTLSConfig{
			Outgoing: &structs.MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				TLSMaxVersion: types.TLSv1_3,
				ECDHCurves:    []string{"X25519MLKEM768", "X25519"},
			},
		},
	}
	meshWithoutCurves := &structs.MeshConfigEntry{
		TLS: &structs.MeshTLSConfig{
			Outgoing: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_3},
		},
	}

	t.Run("old server rejects ECDHCurves", func(t *testing.T) {
		t.Parallel()
		dir, s := testServerWithConfig(t, func(c *Config) { c.Build = "2.0.4" })
		defer os.RemoveAll(dir)
		defer s.Shutdown()
		codec := rpcClient(t, s)
		defer codec.Close()
		testrpc.WaitForLeader(t, s.RPC, "dc1")

		var out bool
		err := msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry:      meshWithCurves,
		}, &out)
		require.Error(t, err)
		require.Contains(t, err.Error(), "all servers in the datacenter must run Consul 2.1.0 or later before configuring ECDHCurves")

		// Entries that do not set ECDHCurves are unaffected by the gate.
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry:      meshWithoutCurves,
		}, &out))
		require.True(t, out)
	})

	t.Run("current server accepts ECDHCurves even with an old client agent", func(t *testing.T) {
		t.Parallel()
		dir, s := testServer(t)
		defer os.RemoveAll(dir)
		defer s.Shutdown()
		codec := rpcClient(t, s)
		defer codec.Close()
		testrpc.WaitForLeader(t, s.RPC, "dc1")

		dirC, c := testClientWithConfig(t, func(c *Config) { c.Build = "2.0.4" })
		defer os.RemoveAll(dirC)
		defer c.Shutdown()
		joinLAN(t, c, s)

		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
			Datacenter: "dc1",
			Entry:      meshWithCurves,
		}, &out))
		require.True(t, out)

		_, entry, err := s.fsm.State().ConfigEntry(nil, structs.MeshConfig, structs.MeshConfigMesh, nil)
		require.NoError(t, err)
		require.Equal(t, []string{"X25519MLKEM768", "X25519"}, entry.(*structs.MeshConfigEntry).TLS.Outgoing.ECDHCurves)
	})
}

func TestReportAgentsWithoutMeshPQCSupport_MetricAndLog(t *testing.T) {
	t.Parallel()

	member := func(name, build string) serf.Member {
		return serf.Member{Name: name, Status: serf.StatusAlive, Tags: map[string]string{"role": "node", "build": build}}
	}

	type emitted struct {
		key []string
		val float32
	}

	t.Run("outdated agents emit the metric and a warning", func(t *testing.T) {
		var buf bytes.Buffer
		logger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Warn})
		var got []emitted
		n := reportAgentsWithoutMeshPQCSupport(logger, []serf.Member{
			member("client-new", "2.1.0:abcdef"),
			member("client-b", "2.0.4:abcdef"),
			member("client-a", "1.22.3:abcdef"),
		}, func(key []string, val float32) { got = append(got, emitted{key, val}) })

		require.Equal(t, 2, n)
		require.Equal(t, []emitted{{key: []string{"config_entry", "mesh", "pqc_outdated_agents"}, val: 2}}, got)
		require.Contains(t, buf.String(), "[WARN]")
		require.Contains(t, buf.String(), "post-quantum TLS key exchange")
		require.Contains(t, buf.String(), "agents=\"client-a, client-b\"")
		require.NotContains(t, buf.String(), "client-new")
	})

	t.Run("no outdated agents emit nothing", func(t *testing.T) {
		var buf bytes.Buffer
		logger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Trace})
		called := false
		n := reportAgentsWithoutMeshPQCSupport(logger, []serf.Member{
			member("client-1", "2.1.0:abcdef"),
		}, func([]string, float32) { called = true })

		require.Zero(t, n)
		require.False(t, called)
		require.Empty(t, buf.String())
	})

	t.Run("metric is registered for Prometheus", func(t *testing.T) {
		var names [][]string
		for _, d := range ConfigEntryCounters {
			names = append(names, d.Name)
		}
		require.Contains(t, names, []string{"config_entry", "mesh", "pqc_outdated_agents"})
	})
}

// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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
		servers     testServersProvider
		dcs         []string
		wantErr     []string
		wantNoInErr []string
	}{
		"all servers on 2.1.0": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.1.0", "dc1", serf.StatusAlive),
			},
			dcs: []string{"dc1"},
		},
		"servers on newer and pre-release versions": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0-rc1", "dc1", serf.StatusAlive),
				makeServer("s2", "3.0.0-dev", "dc1", serf.StatusAlive),
			},
			dcs: []string{"dc1"},
		},
		"one alive server on 2.0.x": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc1", serf.StatusAlive),
			},
			dcs:         []string{"dc1"},
			wantErr:     []string{"must run Consul 2.1.0 or later", "dc1/s2 (2.0.4)"},
			wantNoInErr: []string{"force-leave"},
		},
		"failed server on 2.0.x blocks and explains force-leave": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s3", "2.0.4", "dc1", serf.StatusFailed),
			},
			dcs:     []string{"dc1"},
			wantErr: []string{"dc1/s3 (2.0.4)", "failed", "consul force-leave"},
		},
		"left server on 2.0.x is ignored": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc1", serf.StatusLeft),
			},
			dcs: []string{"dc1"},
		},
		"old server in a secondary datacenter blocks": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc2", serf.StatusAlive),
			},
			dcs:     []string{"dc1", "dc2"},
			wantErr: []string{"dc2/s2 (2.0.4)"},
		},
		"old servers in several datacenters are all reported sorted": {
			servers: testServersProvider{
				makeServer("s9", "2.0.4", "dc3", serf.StatusAlive),
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "1.22.0", "dc2", serf.StatusAlive),
			},
			dcs:     []string{"dc3", "dc1", "dc2"},
			wantErr: []string{"dc2/s2 (1.22.0), dc3/s9 (2.0.4)"},
		},
		"datacenters that are not checked are ignored": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
				makeServer("s2", "2.0.4", "dc2", serf.StatusAlive),
			},
			dcs: []string{"dc1"},
		},
		"datacenter without known servers": {
			servers: testServersProvider{
				makeServer("s1", "2.1.0", "dc1", serf.StatusAlive),
			},
			dcs: []string{"dc1", "dc9"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := serversSupportMeshECDHCurves(tc.servers, tc.dcs)
			if len(tc.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tc.wantErr {
				require.Contains(t, err.Error(), want)
			}
			for _, notWant := range tc.wantNoInErr {
				require.NotContains(t, err.Error(), notWant)
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
		"members listed more than once are reported once": {
			members: []serf.Member{
				member("server-2", "consul", "2.0.4:abcdef", serf.StatusAlive),
				member("server-2", "consul", "2.0.4:abcdef", serf.StatusAlive),
			},
			want: []string{"server-2"},
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

func TestLogAgentsWithoutMeshPQCSupport(t *testing.T) {
	t.Parallel()

	member := func(name, build string) serf.Member {
		return serf.Member{Name: name, Status: serf.StatusAlive, Tags: map[string]string{"role": "node", "build": build}}
	}

	t.Run("outdated agents are logged with their partition", func(t *testing.T) {
		var buf bytes.Buffer
		logger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Warn})
		n := logAgentsWithoutMeshPQCSupport(logger, "default", []serf.Member{
			member("client-new", "2.1.0:abcdef"),
			member("client-b", "2.0.4:abcdef"),
			member("client-a", "1.22.3:abcdef"),
		})

		require.Equal(t, 2, n)
		require.Contains(t, buf.String(), "[WARN]")
		require.Contains(t, buf.String(), "post-quantum TLS key exchange")
		require.Contains(t, buf.String(), "partition=default")
		require.Contains(t, buf.String(), "agents=\"client-a, client-b\"")
		require.NotContains(t, buf.String(), "client-new")
	})

	t.Run("no outdated agents log nothing", func(t *testing.T) {
		var buf bytes.Buffer
		logger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Trace})
		n := logAgentsWithoutMeshPQCSupport(logger, "default", []serf.Member{member("client-1", "2.1.0:abcdef")})
		require.Zero(t, n)
		require.Empty(t, buf.String())
	})
}

func TestMeshPQCOutdatedAgentCounts(t *testing.T) {
	t.Parallel()

	member := func(name, build string) serf.Member {
		return serf.Member{Name: name, Status: serf.StatusAlive, Tags: map[string]string{"role": "node", "build": build}}
	}
	pqcMesh := func() *structs.MeshConfigEntry {
		return &structs.MeshConfigEntry{
			TLS: &structs.MeshTLSConfig{
				Outgoing: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_3},
			},
		}
	}
	plainMesh := &structs.MeshConfigEntry{
		TLS: &structs.MeshTLSConfig{
			Outgoing: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_2},
		},
	}

	membersByPartition := map[string][]serf.Member{
		"default": {member("old-1", "2.0.4:abc"), member("new-1", "2.1.0:abc"), member("old-2", "1.22.0:abc")},
	}
	membersFn := func(partition string) ([]serf.Member, error) {
		m, ok := membersByPartition[partition]
		if !ok {
			return nil, fmt.Errorf("partition %q not found", partition)
		}
		return m, nil
	}

	t.Run("entry enabling PQC counts outdated agents in its partition", func(t *testing.T) {
		got := meshPQCOutdatedAgentCounts([]structs.ConfigEntry{pqcMesh()}, membersFn, hclog.NewNullLogger())
		require.Equal(t, map[string]int{"default": 2}, got)
	})

	t.Run("entry without PQC reports zero", func(t *testing.T) {
		got := meshPQCOutdatedAgentCounts([]structs.ConfigEntry{plainMesh}, membersFn, hclog.NewNullLogger())
		require.Equal(t, map[string]int{"default": 0}, got)
	})

	t.Run("non-mesh entries are ignored", func(t *testing.T) {
		got := meshPQCOutdatedAgentCounts([]structs.ConfigEntry{
			&structs.ServiceConfigEntry{Kind: structs.ServiceDefaults, Name: "web"},
		}, membersFn, hclog.NewNullLogger())
		require.Empty(t, got)
	})

	t.Run("member listing errors are skipped", func(t *testing.T) {
		got := meshPQCOutdatedAgentCounts([]structs.ConfigEntry{pqcMesh()}, func(string) ([]serf.Member, error) {
			return nil, fmt.Errorf("boom")
		}, hclog.NewNullLogger())
		require.Empty(t, got)
	})
}

func TestEmitMeshPQCOutdatedAgentGauges(t *testing.T) {
	t.Parallel()

	type gauge struct {
		key       []string
		val       float32
		partition string
	}
	var got []gauge
	set := func(key []string, val float32, labels []metrics.Label) {
		g := gauge{key: key, val: val}
		for _, l := range labels {
			if l.Name == "partition" {
				g.partition = l.Value
			}
		}
		got = append(got, g)
	}
	key := []string{"mesh", "pqc_outdated_agents"}

	reported := emitMeshPQCOutdatedAgentGauges(map[string]int{"default": 3}, nil, set)
	require.Equal(t, []gauge{{key: key, val: 3, partition: "default"}}, got)

	// A partition that is no longer reported is reset to zero once.
	got = nil
	reported = emitMeshPQCOutdatedAgentGauges(map[string]int{}, reported, set)
	require.Equal(t, []gauge{{key: key, val: 0, partition: "default"}}, got)

	got = nil
	emitMeshPQCOutdatedAgentGauges(map[string]int{}, reported, set)
	require.Empty(t, got)

	var names [][]string
	for _, d := range ConfigEntryGauges {
		names = append(names, d.Name)
	}
	require.Contains(t, names, key)
}

func meshWithOutgoing(min types.TLSVersion, curves ...string) *structs.MeshConfigEntry {
	return &structs.MeshConfigEntry{
		TLS: &structs.MeshTLSConfig{
			Outgoing: &structs.MeshDirectionalTLSConfig{
				TLSMinVersion: min,
				TLSMaxVersion: types.TLSv1_3,
				ECDHCurves:    curves,
			},
		},
	}
}

func applyMeshEntry(t *testing.T, codec rpc.ClientCodec, entry *structs.MeshConfigEntry, token string) error {
	t.Helper()
	var out bool
	return msgpackrpc.CallWithCodec(codec, "ConfigEntry.Apply", &structs.ConfigEntryRequest{
		Datacenter:   "dc1",
		Entry:        entry,
		WriteRequest: structs.WriteRequest{Token: token},
	}, &out)
}

func TestConfigEntry_Apply_MeshECDHCurvesServerVersionGate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	meshWithCurves := meshWithOutgoing(types.TLSv1_3, "X25519MLKEM768", "X25519")
	meshWithoutCurves := meshWithOutgoing(types.TLSv1_3)

	t.Run("old server rejects ECDHCurves", func(t *testing.T) {
		t.Parallel()
		dir, s := testServerWithConfig(t, func(c *Config) { c.Build = "2.0.4" })
		defer os.RemoveAll(dir)
		defer s.Shutdown()
		codec := rpcClient(t, s)
		defer codec.Close()
		testrpc.WaitForLeader(t, s.RPC, "dc1")

		err := applyMeshEntry(t, codec, meshWithCurves, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "must run Consul 2.1.0 or later")

		// Entries that do not set ECDHCurves are unaffected by the gate.
		require.NoError(t, applyMeshEntry(t, codec, meshWithoutCurves, ""))
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

		require.NoError(t, applyMeshEntry(t, codec, meshWithCurves, ""))

		_, entry, err := s.fsm.State().ConfigEntry(nil, structs.MeshConfig, structs.MeshConfigMesh, nil)
		require.NoError(t, err)
		require.Equal(t, []string{"X25519MLKEM768", "X25519"}, entry.(*structs.MeshConfigEntry).TLS.Outgoing.ECDHCurves)
	})

	t.Run("identical rewrite is a no-op even when servers are outdated", func(t *testing.T) {
		t.Parallel()
		dir, s := testServerWithConfig(t, func(c *Config) { c.Build = "2.0.4" })
		defer os.RemoveAll(dir)
		defer s.Shutdown()
		codec := rpcClient(t, s)
		defer codec.Close()
		testrpc.WaitForLeader(t, s.RPC, "dc1")

		// Simulate an entry written before the servers were downgraded or
		// before an outdated server joined.
		stored := meshWithOutgoing(types.TLSv1_3, "X25519MLKEM768", "X25519")
		require.NoError(t, stored.Normalize())
		idx, _, err := s.fsm.State().ConfigEntry(nil, structs.MeshConfig, structs.MeshConfigMesh, nil)
		require.NoError(t, err)
		require.NoError(t, s.fsm.State().EnsureConfigEntry(idx+1, stored))

		require.NoError(t, applyMeshEntry(t, codec, meshWithCurves, ""))

		err = applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_3, "X25519MLKEM768"), "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "must run Consul 2.1.0 or later")
	})
}

func TestConfigEntry_Apply_MeshECDHCurvesChecksRunAfterAuthorization(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServerWithConfig(t, func(c *Config) {
		c.Build = "2.0.4"
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	testrpc.WaitForTestAgent(t, s.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s)
	defer codec.Close()

	readOnly := createToken(t, codec, `mesh = "read"`)

	err := applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_2, "X25519"), readOnly)
	require.True(t, acl.IsErrPermissionDenied(err), "expected permission denied, got %v", err)
	require.NotContains(t, err.Error(), "2.0.4")
	require.NotContains(t, err.Error(), "P-256")

	// With write permission the compatibility checks run.
	err = applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_3, "X25519"), "root")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must run Consul 2.1.0 or later")
}

func TestConfigEntry_Apply_MeshECDHCurvesSecondaryDatacenter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()
	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1"
		c.Build = "2.0.4"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	testrpc.WaitForLeader(t, s2.RPC, "dc2")
	joinWAN(t, s2, s1)
	retry.Run(t, func(r *retry.R) {
		require.Contains(r, s1.router.GetDatacenters(), "dc2")
	})

	err := applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_3, "X25519MLKEM768", "X25519"), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dc2/")
	require.Contains(t, err.Error(), "(2.0.4)")

	require.NoError(t, applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_3), ""))
}

func TestConfigEntry_Apply_MeshOutgoingP256OnlyForChangedCurves(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	defer codec.Close()
	testrpc.WaitForLeader(t, s.RPC, "dc1")

	// New outgoing curves that cannot work with TLS 1.2 are rejected.
	err := applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_2, "X25519MLKEM768", "X25519"), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), `ecdh_curves must include "P-256"`)

	// Incoming curves without P-256 are accepted with a warning.
	incomingOnly := &structs.MeshConfigEntry{TLS: &structs.MeshTLSConfig{
		Incoming: &structs.MeshDirectionalTLSConfig{TLSMinVersion: types.TLSv1_2, ECDHCurves: []string{"X25519"}},
	}}
	require.NoError(t, applyMeshEntry(t, codec, incomingOnly, ""))

	// An entry stored before this validation existed can be rewritten as long
	// as its outgoing curves are unchanged.
	stored := meshWithOutgoing(types.TLSv1_2, "X25519MLKEM768", "X25519")
	idx, _, err := s.fsm.State().ConfigEntry(nil, structs.MeshConfig, structs.MeshConfigMesh, nil)
	require.NoError(t, err)
	require.NoError(t, s.fsm.State().EnsureConfigEntry(idx+1, stored))

	rewritten := meshWithOutgoing(types.TLSv1_2, "X25519MLKEM768", "X25519")
	rewritten.AllowEnablingPermissiveMutualTLS = true
	require.NoError(t, applyMeshEntry(t, codec, rewritten, ""))

	changed := meshWithOutgoing(types.TLSv1_2, "X25519")
	err = applyMeshEntry(t, codec, changed, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), `ecdh_curves must include "P-256"`)
}

func TestServer_UpdateMeshPQCOutdatedAgentGauges(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir, s := testServer(t)
	defer os.RemoveAll(dir)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	defer codec.Close()
	testrpc.WaitForLeader(t, s.RPC, "dc1")

	retry.Run(t, func(r *retry.R) {
		require.True(r, s.leaderRoutineManager.IsRunning(meshPQCMetricsRoutineName))
	})

	dirC, c := testClientWithConfig(t, func(c *Config) { c.Build = "2.0.4" })
	defer os.RemoveAll(dirC)
	defer c.Shutdown()
	joinLAN(t, c, s)

	type gauge struct {
		val       float32
		partition string
	}
	collect := func(reported map[string]struct{}) (map[string]struct{}, []gauge) {
		var got []gauge
		reported = s.updateMeshPQCOutdatedAgentGauges(reported, func(key []string, val float32, labels []metrics.Label) {
			require.Equal(t, []string{"mesh", "pqc_outdated_agents"}, key)
			g := gauge{val: val}
			for _, l := range labels {
				if l.Name == "partition" {
					g.partition = l.Value
				}
			}
			got = append(got, g)
		})
		return reported, got
	}

	reported, got := collect(nil)
	require.Empty(t, got, "no mesh entry enables PQC yet")

	require.NoError(t, applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_3), ""))
	retry.Run(t, func(r *retry.R) {
		var g []gauge
		reported, g = collect(reported)
		require.Equal(r, []gauge{{val: 1, partition: "default"}}, g)
	})

	require.NoError(t, applyMeshEntry(t, codec, meshWithOutgoing(types.TLSv1_2), ""))
	_, got = collect(reported)
	require.Equal(t, []gauge{{val: 0, partition: "default"}}, got)
}

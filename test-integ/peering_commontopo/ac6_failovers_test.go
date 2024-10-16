// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

type ac6FailoversSuite struct {
	// inputs
	// with all false, this gives us a scenario with:
	// - a "near" server in the accepter cluster (DC1), partitition default, namespace default
	// - a "far" server in the dialer cluster (DC2), partition default, namespace default
	// - a client in the accepter cluster (DC1), partition default, namespace default, with:
	//   - upstream near server (DC1)
	//   - failover to far server (DC2)
	//
	// TODO: technically if NearInDial && !FarInAcc (i.e., near == far), then we're not doing peering at all,
	// and could do this test in a single DC

	// when true, put the client (and its default upstream server) in the dialer peer; otherwise, put client in accepter
	NearInDial bool
	// when true, put the client (and its default upstream server) in the nondefault partition/namespace; otherwise in the default
	NearInPartAlt bool
	NearInNSAlt   bool
	// when true, put far server to the accepter peer; otherwise the dialer
	FarInAcc bool
	// when true, put far server to nondefault partition/namespace (ENT-only); otherwise, failover to default
	FarInPartAlt bool
	FarInNSAlt   bool

	// launch outputs, for querying during test
	clientSID topology.ID
	// near = same DC as client; far = other DC
	nearServerSID topology.ID
	// used to remove the node and trigger failover
	nearServerNode topology.NodeID
	farServerSID   topology.ID
	farServerNode  topology.NodeID
}

// Note: this test cannot share topo
func TestAC6Failovers(t *testing.T) {
	// bit banging to get all permutations of all params
	const nParams = 3
	// i.e 2**nParams
	const n = int(1) << nParams
	for i := 0; i < n; i++ {
		s := ac6FailoversSuite{
			// xth bit == 1
			NearInDial:    (i>>0)&1 == 1,
			NearInPartAlt: (i>>1)&1 == 1,
			FarInPartAlt:  (i>>2)&1 == 1,
		}
		// ensure the servers are always in separate DCs
		s.FarInAcc = s.NearInDial
		t.Run(fmt.Sprintf("%02d_%s", i, s.testName()), func(t *testing.T) {
			t.Parallel()
			ct := NewCommonTopo(t)
			s.setup(t, ct)
			ct.Launch(t)
			s.test(t, ct)
		})
	}
}

func TestNET5029Failovers(t *testing.T) {
	// TODO: *.{a,b} are not actually peering tests, and should technically be moved elsewhere
	suites := map[string]ac6FailoversSuite{
		"1.a": {
			FarInAcc:     true,
			FarInPartAlt: true,
		},
		"1.b": {
			FarInAcc:   true,
			FarInNSAlt: true,
		},
		"1.c": {
			FarInNSAlt: true,
		},
		"1.d": {
			FarInPartAlt: true,
		},
		"2.a": {
			FarInAcc:      true,
			NearInPartAlt: true,
		},
		"2.b": {
			FarInAcc:    true,
			NearInNSAlt: true,
		},
		"2.c": {
			NearInDial:  true,
			NearInNSAlt: true,
			FarInAcc:    true,
		},
		"2.d": {
			NearInDial:    true,
			NearInPartAlt: true,
			FarInAcc:      true,
		},
	}
	for name, s := range suites {
		s := s
		t.Run(fmt.Sprintf("%s_%s", name, s.testName()), func(t *testing.T) {
			if name == "1.b" {
				t.Skip("TODO: fails with 503/504")
			}
			t.Parallel()
			ct := NewCommonTopo(t)
			s.setup(t, ct)
			ct.Launch(t)
			s.test(t, ct)
		})
	}
}

func TestAC6Failovers_AllPermutations(t *testing.T) {
	//
	t.Skip("Too many permutations")
	// bit banging to get all permutations of all params
	const nParams = 6
	// i.e 2**nParams
	const n = int(1) << nParams
	for i := 0; i < n; i++ {
		s := ac6FailoversSuite{
			// xth bit == 1
			NearInDial:    (i>>0)&1 == 1,
			FarInAcc:      (i>>1)&1 == 1,
			NearInPartAlt: (i>>2)&1 == 1,
			FarInPartAlt:  (i>>3)&1 == 1,
			NearInNSAlt:   (i>>4)&1 == 1,
			FarInNSAlt:    (i>>5)&1 == 1,
		}
		t.Run(fmt.Sprintf("%02d_%s", i, s.testName()), func(t *testing.T) {
			t.Parallel()
			ct := NewCommonTopo(t)
			s.setup(t, ct)
			ct.Launch(t)
			s.test(t, ct)
		})
	}
}

func (s *ac6FailoversSuite) testName() (ret string) {
	switch s.NearInDial {
	case true:
		ret += "dial"
	default:
		ret += "acc"
	}
	ret += "."
	switch s.NearInPartAlt {
	case true:
		ret += "alt"
	default:
		ret += "default"
	}
	ret += "."
	switch s.NearInNSAlt {
	case true:
		ret += "alt"
	default:
		ret += "default"
	}

	ret += "->"

	switch s.FarInAcc {
	case true:
		ret += "acc"
	default:
		ret += "dial"
	}
	ret += "."
	switch s.FarInPartAlt {
	case true:
		ret += "alt"
	default:
		ret += "default"
	}
	ret += "."
	switch s.FarInNSAlt {
	case true:
		ret += "alt"
	default:
		ret += "default"
	}

	return
}

func (s *ac6FailoversSuite) setup(t *testing.T, ct *commonTopo) {
	if !utils.IsEnterprise() {
		if s.NearInPartAlt || s.FarInPartAlt {
			t.Skip("ENT required for nondefault partitions")
		}
		if s.NearInNSAlt || s.FarInNSAlt {
			t.Skip("ENT required for nondefault namespaces")
		}
	}

	nearClu := ct.DC1
	farClu := ct.DC2
	if s.NearInDial {
		nearClu = ct.DC2
	}
	if s.FarInAcc {
		farClu = ct.DC1
	}

	// - server in clientPartition/DC (main target)
	nearServerSID := topology.ID{
		Name:      "ac6-server",
		Partition: defaultToEmptyForCE("default"),
		Namespace: defaultToEmptyForCE("default"),
	}
	if s.NearInPartAlt {
		nearServerSID.Partition = "part1"
	}
	if s.NearInNSAlt {
		nearServerSID.Namespace = "ns1"
	}
	nearServer := NewFortioServiceWithDefaults(
		nearClu.Datacenter,
		nearServerSID,
		nil,
	)
	nearServerNode := ct.AddServiceNode(nearClu, serviceExt{Workload: nearServer})

	nearClu.InitialConfigEntries = append(nearClu.InitialConfigEntries,
		&api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      nearServerSID.Name,
			Partition: defaultToEmptyForCE(nearServerSID.Partition),
			Namespace: defaultToEmptyForCE(nearServerSID.Namespace),
			Protocol:  "http",
		},
	)
	// - server in otherPartition/otherDC
	farServerSID := topology.ID{
		Name:      nearServerSID.Name,
		Partition: defaultToEmptyForCE("default"),
		Namespace: defaultToEmptyForCE("default"),
	}
	if s.FarInPartAlt {
		farServerSID.Partition = "part1"
	}
	if s.FarInNSAlt {
		farServerSID.Namespace = "ns1"
	}
	farServer := NewFortioServiceWithDefaults(
		farClu.Datacenter,
		farServerSID,
		nil,
	)
	farServerNode := ct.AddServiceNode(farClu, serviceExt{Workload: farServer})
	if nearClu != farClu {
		ct.ExportService(farClu, farServerSID.Partition,
			api.ExportedService{
				Name:      farServerSID.Name,
				Namespace: defaultToEmptyForCE(farServerSID.Namespace),
				Consumers: []api.ServiceConsumer{
					{
						Peer: LocalPeerName(nearClu, nearServerSID.Partition),
					},
				},
			},
		)
	} else if nearClu == farClu && farServerSID.Partition != nearServerSID.Partition {
		ct.ExportService(farClu, farServerSID.Partition,
			api.ExportedService{
				Name:      farServerSID.Name,
				Namespace: defaultToEmptyForCE(farServerSID.Namespace),
				Consumers: []api.ServiceConsumer{
					{
						// this must not be "", or else it is basically ignored altogether
						// TODO: bug? if this whole struct is empty, that should be an error
						Partition: topology.PartitionOrDefault(nearServerSID.Partition),
					},
				},
			},
		)
	}

	var targets []api.ServiceResolverFailoverTarget
	if nearClu != farClu {
		targets = []api.ServiceResolverFailoverTarget{
			{
				Service:   farServerSID.Name,
				Peer:      LocalPeerName(farClu, farServerSID.Partition),
				Namespace: defaultToEmptyForCE(farServerSID.Namespace),
			},
		}
	} else {
		part := ConfigEntryPartition(farServerSID.Partition)
		// weird exception here where target partition set to "" means "inherit from parent"
		// TODO: bug? docs say "" -> default:
		// https://developer.hashicorp.com/consul/docs/connect/config-entries/service-resolver#failover-targets-partition
		if farServerSID.Partition == "default" && nearServerSID.Partition != "default" {
			part = "default"
		}
		targets = []api.ServiceResolverFailoverTarget{
			{
				Service:   farServerSID.Name,
				Partition: defaultToEmptyForCE(part),
				Namespace: defaultToEmptyForCE(farServerSID.Namespace),
			},
		}
	}

	nearClu.InitialConfigEntries = append(nearClu.InitialConfigEntries,
		&api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      farServerSID.Name,
			Partition: defaultToEmptyForCE(farServerSID.Partition),
			Namespace: defaultToEmptyForCE(farServerSID.Namespace),
			Protocol:  "http",
		},
		&api.ServiceResolverConfigEntry{
			Kind:      api.ServiceResolver,
			Name:      nearServerSID.Name,
			Partition: defaultToEmptyForCE(nearServerSID.Partition),
			Namespace: defaultToEmptyForCE(nearServerSID.Namespace),
			Failover: map[string]api.ServiceResolverFailover{
				"*": {
					Targets: targets,
				},
			},
		},
	)

	clientSID := topology.ID{
		Name:      "ac6-client",
		Partition: defaultToEmptyForCE(nearServerSID.Partition),
		Namespace: defaultToEmptyForCE(nearServerSID.Namespace),
	}
	client := NewFortioServiceWithDefaults(
		nearClu.Datacenter,
		clientSID,
		func(s *topology.Workload) {
			// Upstream per partition
			s.Upstreams = []*topology.Upstream{
				{
					ID: topology.ID{
						Name:      nearServerSID.Name,
						Partition: defaultToEmptyForCE(nearServerSID.Partition),
						Namespace: defaultToEmptyForCE(nearServerSID.Namespace),
					},
					LocalPort: 5000,
					// exposed so we can hit it directly
					// TODO: we shouldn't do this; it's not realistic
					LocalAddress: "0.0.0.0",
				},
			}
		},
	)
	ct.AddServiceNode(nearClu, serviceExt{Workload: client})
	nearClu.InitialConfigEntries = append(nearClu.InitialConfigEntries,
		&api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      clientSID.Name,
			Partition: defaultToEmptyForCE(clientSID.Partition),
			Namespace: defaultToEmptyForCE(clientSID.Namespace),
			Protocol:  "http",
		},
	)

	// intentions
	nearClu.InitialConfigEntries = append(nearClu.InitialConfigEntries,
		&api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      nearServerSID.Name,
			Partition: defaultToEmptyForCE(nearServerSID.Partition),
			Namespace: defaultToEmptyForCE(nearServerSID.Namespace),
			Sources: []*api.SourceIntention{{
				Name:      clientSID.Name,
				Namespace: defaultToEmptyForCE(clientSID.Namespace),
				// in this field, "" -> destination partition, so no ConfigEntryPartition :eyeroll:
				// https://developer.hashicorp.com/consul/docs/connect/config-entries/service-intentions#sources-partition
				Partition: defaultToEmptyForCE(clientSID.Partition),
				Action:    api.IntentionActionAllow,
			}},
		},
	)
	farSource := api.SourceIntention{
		Name:      clientSID.Name,
		Namespace: defaultToEmptyForCE(clientSID.Namespace),
		Peer:      LocalPeerName(nearClu, clientSID.Partition),
		Action:    api.IntentionActionAllow,
	}
	if nearClu == farClu {
		farSource.Peer = ""
		// in this field, "" -> destination partition, so no ConfigEntryPartition :eyeroll:
		// https://developer.hashicorp.com/consul/docs/connect/config-entries/service-intentions#sources-partition
		farSource.Partition = topology.PartitionOrDefault(clientSID.Partition)
	}
	farClu.InitialConfigEntries = append(farClu.InitialConfigEntries,
		&api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      farServerSID.Name,
			Partition: defaultToEmptyForCE(farServerSID.Partition),
			Namespace: defaultToEmptyForCE(farServerSID.Namespace),
			Sources:   []*api.SourceIntention{&farSource},
		},
	)

	s.clientSID = clientSID
	s.nearServerSID = nearServerSID
	s.farServerSID = farServerSID
	s.nearServerNode = nearServerNode.ID()
	s.farServerNode = farServerNode.ID()
}

func (s *ac6FailoversSuite) test(t *testing.T, ct *commonTopo) {
	// NOTE: *not parallel* because we mutate resources that are shared
	// between test cases (disable/enable nodes)

	nearClu := ct.Sprawl.Topology().Clusters["dc1"]
	farClu := ct.Sprawl.Topology().Clusters["dc2"]
	if s.NearInDial {
		nearClu = ct.Sprawl.Topology().Clusters["dc2"]
	}
	if s.FarInAcc {
		farClu = ct.Sprawl.Topology().Clusters["dc1"]
	}

	svcs := nearClu.WorkloadsByID(s.clientSID)
	require.Len(t, svcs, 1, "expected exactly one client in datacenter")

	client := svcs[0]
	require.Len(t, client.Upstreams, 1, "expected one upstream for client")
	upstream := client.Upstreams[0]

	fmt.Println("### preconditions")

	// this is the server in the same DC and partitions as client
	serverSID := s.nearServerSID
	serverSID.Normalize()
	ct.Assert.FortioFetch2FortioName(t, client, upstream, nearClu.Name, serverSID)

	ct.Assert.CatalogServiceExists(t, nearClu.Name, upstream.ID.Name, utils.CompatQueryOpts(&api.QueryOptions{
		Partition: upstream.ID.Partition,
		Namespace: upstream.ID.Namespace,
	}))

	if t.Failed() {
		t.Fatal("failed preconditions")
	}

	fmt.Println("### failover")

	cfg := ct.Sprawl.Config()
	DisableNode(t, cfg, nearClu.Name, s.nearServerNode)
	require.NoError(t, ct.Sprawl.RelaunchWithPhase(cfg, sprawl.LaunchPhaseRegular))
	// Clusters for imported services rely on outlier detection for
	// failovers, NOT eds_health_status. This means that killing the
	// node above does not actually make the envoy cluster UNHEALTHY
	// so we do not assert for it.
	expectSID := s.farServerSID
	expectSID.Normalize()
	ct.Assert.FortioFetch2FortioName(t, client, upstream, farClu.Name, expectSID)
}

func defaultToEmptyForCE(tenancy string) string {
	if utils.IsEnterprise() {
		return tenancy
	}
	return topology.DefaultToEmpty(tenancy)
}

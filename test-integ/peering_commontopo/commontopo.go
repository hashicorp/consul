// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/test-integ/topoutil"
)

// commonTopo helps create a shareable topology configured to represent
// the common denominator between tests.
//
// Use NewCommonTopo to create.
//
// Compatible suites should implement sharedTopoSuite.
//
// Style:
//   - avoid referencing components using strings, prefer IDs like Service ID, etc.
//   - avoid passing addresses and ports, etc. Instead, look up components in sprawl.Topology
//     by ID to find a concrete type, then pass that to helper functions that know which port to use
//   - minimize the surface area of information passed between setup and test code (via members)
//     to those that are strictly necessary
type commonTopo struct {
	//
	Cfg *topology.Config
	// shortcuts to corresponding entry in Cfg
	DC1 *topology.Cluster
	DC2 *topology.Cluster
	DC3 *topology.Cluster

	// set after Launch. Should be considered read-only
	Sprawl *sprawl.Sprawl
	Assert *topoutil.Asserter

	// track per-DC services to prevent duplicates
	services map[string]map[topology.ServiceID]struct{}
}

const agentlessDC = "dc2"

func NewCommonTopo(t *testing.T) *commonTopo {
	t.Helper()

	ct := commonTopo{}

	const nServers = 3

	// Make 3-server clusters in dc1 and dc2
	// For simplicity, the Name and Datacenter of the clusters are the same.
	// dc1 and dc2 should be symmetric.
	dc1 := clusterWithJustServers("dc1", nServers)
	ct.DC1 = dc1
	dc2 := clusterWithJustServers("dc2", nServers)
	ct.DC2 = dc2
	// dc3 is a failover cluster for both dc1 and dc2
	dc3 := clusterWithJustServers("dc3", 1)
	// dc3 is only used for certain failover scenarios and does not need tenancies
	dc3.Partitions = []*topology.Partition{{Name: "default"}}
	ct.DC3 = dc3

	injectTenancies(dc1)
	injectTenancies(dc2)
	// dc3 is only used for certain failover scenarios and does not need tenancies
	dc3.Partitions = []*topology.Partition{{Name: "default"}}

	ct.services = map[string]map[topology.ServiceID]struct{}{}
	for _, dc := range []*topology.Cluster{dc1, dc2, dc3} {
		ct.services[dc.Datacenter] = map[topology.ServiceID]struct{}{}
	}

	peerings := addPeerings(dc1, dc2)
	peerings = append(peerings, addPeerings(dc1, dc3)...)
	peerings = append(peerings, addPeerings(dc2, dc3)...)

	addMeshGateways(dc1)
	addMeshGateways(dc2)
	addMeshGateways(dc3)

	setupGlobals(dc1)
	setupGlobals(dc2)
	setupGlobals(dc3)

	// Build final configuration
	ct.Cfg = &topology.Config{
		Images: utils.TargetImages(),
		Networks: []*topology.Network{
			{Name: dc1.Datacenter}, // "dc1" LAN
			{Name: dc2.Datacenter}, // "dc2" LAN
			{Name: dc3.Datacenter}, // "dc3" LAN
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			dc1,
			dc2,
			dc3,
		},
		Peerings: peerings,
	}
	return &ct
}

// calls sprawltest.Launch followed by s.postLaunchChecks
func (ct *commonTopo) Launch(t *testing.T) {
	if ct.Sprawl != nil {
		t.Fatalf("Launch must only be called once")
	}
	ct.Sprawl = sprawltest.Launch(t, ct.Cfg)

	ct.Assert = topoutil.NewAsserter(ct.Sprawl)
	ct.postLaunchChecks(t)
}

// tests that use Relaunch might want to call this again afterwards
func (ct *commonTopo) postLaunchChecks(t *testing.T) {
	t.Logf("TESTING RELATIONSHIPS: \n%s",
		topology.RenderRelationships(ct.Sprawl.Topology().ComputeRelationships()),
	)

	// check that exports line up as expected
	for _, clu := range ct.Sprawl.Topology().Clusters {
		// expected exports per peer
		type key struct {
			peer      string
			partition string
			namespace string
		}
		eepp := map[key]int{}
		for _, e := range clu.InitialConfigEntries {
			if e.GetKind() == api.ExportedServices {
				asExport := e.(*api.ExportedServicesConfigEntry)
				// do we care about the partition?
				for _, svc := range asExport.Services {
					for _, con := range svc.Consumers {
						// do we care about con.Partition?
						// TODO: surely there is code to normalize this
						partition := asExport.Partition
						if partition == "" {
							partition = "default"
						}
						namespace := svc.Namespace
						if namespace == "" {
							namespace = "default"
						}
						eepp[key{peer: con.Peer, partition: partition, namespace: namespace}] += 1
					}
				}
			}
		}
		cl := ct.APIClientForCluster(t, clu)
		// TODO: these could probably be done in parallel
		for k, v := range eepp {
			retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
				peering, _, err := cl.Peerings().Read(context.Background(), k.peer, utils.CompatQueryOpts(&api.QueryOptions{
					Partition: k.partition,
					Namespace: k.namespace,
				}))
				require.Nil(r, err, "reading peering data")
				require.NotNilf(r, peering, "peering not found %q", k.peer)
				assert.Len(r, peering.StreamStatus.ExportedServices, v, "peering exported services")
			})
		}
	}

	if t.Failed() {
		t.Fatal("failing fast: post-Launch assertions failed")
	}
}

// PeerName is how you'd address a remote dc+partition locally
// as your peer name.
func LocalPeerName(clu *topology.Cluster, partition string) string {
	return fmt.Sprintf("peer-%s-%s", clu.Datacenter, partition)
}

// TODO: move these to topology
// TODO: alternatively, delete it: we only use it in one place, to bundle up args
type serviceExt struct {
	*topology.Service

	Exports    []api.ServiceConsumer
	Config     *api.ServiceConfigEntry
	Intentions *api.ServiceIntentionsConfigEntry
}

func (ct *commonTopo) AddServiceNode(clu *topology.Cluster, svc serviceExt) *topology.Node {
	clusterName := clu.Name
	if _, ok := ct.services[clusterName][svc.ID]; ok {
		panic(fmt.Sprintf("duplicate service %q in cluster %q", svc.ID, clusterName))
	}
	ct.services[clusterName][svc.ID] = struct{}{}

	// TODO: inline
	serviceHostnameString := func(dc string, id topology.ServiceID) string {
		n := id.Name
		// prepend <namespace>- and <partition>- if they are not default/empty
		// avoids hostname limit of 63 chars in most cases
		// TODO: this obviously isn't scalable
		if id.Namespace != "default" && id.Namespace != "" {
			n = id.Namespace + "-" + n
		}
		if id.Partition != "default" && id.Partition != "" {
			n = id.Partition + "-" + n
		}
		n = dc + "-" + n
		// TODO: experimentally, when this is larger than 63, docker can't start
		// the host. confirmed by internet rumor https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27763
		if len(n) > 63 {
			panic(fmt.Sprintf("docker hostname must not be longer than 63 chars: %q", n))
		}
		return n
	}

	nodeKind := topology.NodeKindClient
	// TODO: bug in deployer somewhere; it should guard against a KindDataplane node with
	// DisableServiceMesh services on it; dataplane is only for service-mesh
	if !svc.DisableServiceMesh && clu.Datacenter == agentlessDC {
		nodeKind = topology.NodeKindDataplane
	}

	node := &topology.Node{
		Kind:      nodeKind,
		Name:      serviceHostnameString(clu.Datacenter, svc.ID),
		Partition: svc.ID.Partition,
		Addresses: []*topology.Address{
			{Network: clu.Datacenter},
		},
		Services: []*topology.Service{
			svc.Service,
		},
		Cluster: clusterName,
	}
	clu.Nodes = append(clu.Nodes, node)

	// Export if necessary
	if len(svc.Exports) > 0 {
		ct.ExportService(clu, svc.ID.Partition, api.ExportedService{
			Name:      svc.ID.Name,
			Namespace: svc.ID.Namespace,
			Consumers: svc.Exports,
		})
	}

	// Add any config entries
	if svc.Config != nil {
		clu.InitialConfigEntries = append(clu.InitialConfigEntries, svc.Config)
	}
	if svc.Intentions != nil {
		clu.InitialConfigEntries = append(clu.InitialConfigEntries, svc.Intentions)
	}

	return node
}

func (ct *commonTopo) APIClientForCluster(t *testing.T, clu *topology.Cluster) *api.Client {
	cl, err := ct.Sprawl.APIClientForCluster(clu.Name, "")
	require.NoError(t, err)
	return cl
}

// ExportService looks for an existing ExportedServicesConfigEntry for the given partition
// and inserts svcs. If none is found, it inserts a new ExportedServicesConfigEntry.
func (ct *commonTopo) ExportService(clu *topology.Cluster, partition string, svcs ...api.ExportedService) {
	var found bool
	for _, ce := range clu.InitialConfigEntries {
		// We check Name because it must be "default" in CE whereas Partition will be "".
		if ce.GetKind() == api.ExportedServices && ce.GetName() == partition {
			found = true
			e := ce.(*api.ExportedServicesConfigEntry)
			e.Services = append(e.Services, svcs...)
		}
	}
	if !found {
		clu.InitialConfigEntries = append(clu.InitialConfigEntries,
			&api.ExportedServicesConfigEntry{
				Name:      partition, // this NEEDs to be "default" in CE
				Partition: ConfigEntryPartition(partition),
				Services:  svcs,
			},
		)
	}
}

func (ct *commonTopo) ClusterByDatacenter(t *testing.T, name string) *topology.Cluster {
	t.Helper()

	for _, clu := range ct.Cfg.Clusters {
		if clu.Datacenter == name {
			return clu
		}
	}
	t.Fatalf("cluster %q not found", name)
	return nil
}

// Since CE config entries do not contain the partition field,
// this func converts default partition to empty string.
func ConfigEntryPartition(p string) string {
	if p == "default" {
		return "" // make this CE friendly
	}
	return p
}

// DisableNode is a no-op if the node is already disabled.
func DisableNode(t *testing.T, cfg *topology.Config, clusterName string, nid topology.NodeID) *topology.Config {
	changed, err := cfg.DisableNode(clusterName, nid)
	require.NoError(t, err)
	if changed {
		t.Logf("disabling node %s in cluster %s", nid.String(), clusterName)
	}
	return cfg
}

// EnableNode is a no-op if the node is already enabled.
func EnableNode(t *testing.T, cfg *topology.Config, clusterName string, nid topology.NodeID) *topology.Config {
	changed, err := cfg.EnableNode(clusterName, nid)
	require.NoError(t, err)
	if changed {
		t.Logf("enabling node %s in cluster %s", nid.String(), clusterName)
	}
	return cfg
}

func setupGlobals(clu *topology.Cluster) {
	for _, part := range clu.Partitions {
		clu.InitialConfigEntries = append(clu.InitialConfigEntries,
			&api.ProxyConfigEntry{
				Name:      api.ProxyConfigGlobal,
				Kind:      api.ProxyDefaults,
				Partition: ConfigEntryPartition(part.Name),
				MeshGateway: api.MeshGatewayConfig{
					// Although we define service-defaults for most upstreams in
					// this test suite, failover tests require a global mode
					// because the default for peered targets is MeshGatewayModeRemote.
					Mode: api.MeshGatewayModeLocal,
				},
			},
			&api.MeshConfigEntry{
				Peering: &api.PeeringMeshConfig{
					PeerThroughMeshGateways: true,
				},
			},
		)
	}
}

// addMeshGateways adds a mesh gateway for every partition in the cluster.
// Assumes that the LAN network name is equal to datacenter name.
func addMeshGateways(c *topology.Cluster) {
	nodeKind := topology.NodeKindClient
	if c.Datacenter == agentlessDC {
		nodeKind = topology.NodeKindDataplane
	}
	for _, p := range c.Partitions {
		c.Nodes = topology.MergeSlices(c.Nodes, topoutil.NewTopologyMeshGatewaySet(
			nodeKind,
			p.Name,
			fmt.Sprintf("%s-%s-mgw", c.Name, p.Name),
			1,
			[]string{c.Datacenter, "wan"},
			nil,
		))
	}
}

func clusterWithJustServers(name string, numServers int) *topology.Cluster {
	return &topology.Cluster{
		Enterprise: utils.IsEnterprise(),
		Name:       name,
		Datacenter: name,
		Nodes: topoutil.NewTopologyServerSet(
			name+"-server",
			numServers,
			[]string{name},
			nil,
		),
	}
}

func addPeerings(acc *topology.Cluster, dial *topology.Cluster) []*topology.Peering {
	peerings := []*topology.Peering{}
	for _, accPart := range acc.Partitions {
		for _, dialPart := range dial.Partitions {
			peerings = append(peerings, &topology.Peering{
				Accepting: topology.PeerCluster{
					Name:      acc.Datacenter,
					Partition: accPart.Name,
					PeerName:  LocalPeerName(dial, dialPart.Name),
				},
				Dialing: topology.PeerCluster{
					Name:      dial.Datacenter,
					Partition: dialPart.Name,
					PeerName:  LocalPeerName(acc, accPart.Name),
				},
			})
		}
	}
	return peerings
}

func injectTenancies(clu *topology.Cluster) {
	if !utils.IsEnterprise() {
		clu.Partitions = []*topology.Partition{
			{
				Name: "default",
				Namespaces: []string{
					"default",
				},
			},
		}
		return
	}

	for _, part := range []string{"default", "part1"} {
		clu.Partitions = append(clu.Partitions,
			&topology.Partition{
				Name: part,
				Namespaces: []string{
					"default",
					"ns1",
				},
			},
		)
	}
}

// Deprecated: topoutil.NewFortioServiceWithDefaults
func NewFortioServiceWithDefaults(
	cluster string,
	sid topology.ServiceID,
	mut func(s *topology.Service),
) *topology.Service {
	return topoutil.NewFortioServiceWithDefaults(cluster, sid, topology.NodeVersionV1, mut)
}

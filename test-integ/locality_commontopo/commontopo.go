// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package locality

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

const (
	consulLocalImage = "consul:local"

	topologyRegionA = "region-a"
	topologyRegionB = "region-b"
)

var (
	service1 = serviceSpec{Name: "service1", Port: 8080}
	service2 = serviceSpec{Name: "service2", Port: 9080}
	service3 = serviceSpec{Name: "service3", Port: 10080}

	// topologyServices lists every service that may appear in node Workloads.
	topologyServices = []serviceSpec{service1, service2, service3}
)

type commonTopo struct {
	Cfg    *topology.Config
	Sprawl *sprawl.Sprawl

	Spec         topologySpec
	ClustersLive map[string]*topology.Cluster
}

// Method groups for `commonTopo` to make intent explicit.

// TopologyLifecycle groups lifecycle actions (start / logging).
type TopologyLifecycle interface {
	Launch(t *testing.T)
	LogTopology(t *testing.T)
	LogManualDNSCommands(t *testing.T)
}

// TopologyAccessor groups read-only accessor methods.
type TopologyAccessor interface {
	APIClientForCluster(t *testing.T, clu *topology.Cluster) *api.Client
	NodesLive(clusterName, nodeRole, nodeType, zone string) []*topology.Node
	NodesSpecs(clusterName, nodeRole, nodeType, zone string) []nodeSpec
	allClustersSpecs() []clusterSpec
	clusterSpec(name string) clusterSpec
	clusterLive(name string) *topology.Cluster
}

// newTopologySpec is the fixed multi-DC layout used by locality DNS tests.
// Set nodeSpec.Workloads per client to the services that run there.
func newTopologySpec() topologySpec {
	return topologySpec{
		Clusters: map[string]clusterSpec{
			"dc1": {
				Name:                "dc1",
				Datacenter:          "dc1",
				Region:              topologyRegionA,
				Zones:               []string{"zone-a1", "zone-a2"},
				LocalityAwareLookup: "always",
				ServiceBlocklist:    []string{service3.Name},
				Nodes: []nodeSpec{
					{Name: "dc1-server1", Role: "server", Zone: "zone-a1"},
					{Name: "dc1-server2", Role: "server", Zone: "zone-a1"},
					{Name: "dc1-server3", Role: "server", Zone: "zone-a2"},
					{Name: "dc1-client1", Role: "client", Zone: "zone-a1"},
					{
						Name:      "dc1-client2",
						Role:      "client",
						Zone:      "zone-a1",
						Workloads: []serviceSpec{service1, service3},
					},
					{
						Name:      "dc1-client3",
						Role:      "client",
						Zone:      "zone-a1",
						Workloads: []serviceSpec{service1, service3},
					},
					{
						Name:      "dc1-client4",
						Role:      "client",
						Zone:      "zone-a1",
						Workloads: []serviceSpec{service1},
					},
					{Name: "dc1-client5", Role: "client", Zone: "zone-a2"},
					{
						Name:      "dc1-client6",
						Role:      "client",
						Zone:      "zone-a2",
						Workloads: []serviceSpec{service1, service3},
					},
					{
						Name:      "dc1-client7",
						Role:      "client",
						Zone:      "zone-a2",
						Workloads: []serviceSpec{service1, service3},
					},
				},
			},
			"dc2": {
				Name:                "dc2",
				Datacenter:          "dc2",
				Region:              topologyRegionB,
				Zones:               []string{"zone-b1", "zone-b2"},
				LocalityAwareLookup: "balanced",
				ServiceAllowlist:    []string{service1.Name, service2.Name},
				Nodes: []nodeSpec{
					{Name: "dc2-server1", Role: "server", Zone: "zone-b1"},
					{Name: "dc2-server2", Role: "server", Zone: "zone-b1"},
					{Name: "dc2-server3", Role: "server", Zone: "zone-b2"},
					{Name: "dc2-client1", Role: "client", Zone: "zone-b1"},
					{
						Name:      "dc2-client2",
						Role:      "client",
						Zone:      "zone-b1",
						Workloads: []serviceSpec{service1, service2, service3},
					},
					{
						Name:      "dc2-client3",
						Role:      "client",
						Zone:      "zone-b1",
						Workloads: []serviceSpec{service1, service2, service3},
					},
					{
						Name:      "dc2-client4",
						Role:      "client",
						Zone:      "zone-b1",
						Workloads: []serviceSpec{service1},
					},
					{Name: "dc2-client5", Role: "client", Zone: "zone-b2"},
					{
						Name:      "dc2-client6",
						Role:      "client",
						Zone:      "zone-b2",
						Workloads: []serviceSpec{service1, service2, service3},
					},
					{
						Name:      "dc2-client7",
						Role:      "client",
						Zone:      "zone-b2",
						Workloads: []serviceSpec{service1, service2, service3},
					},
				},
			},
		},
	}
}

// compile-time assertions that `commonTopo` implements the categories above.
var (
	_ TopologyLifecycle = (*commonTopo)(nil)
	_ TopologyAccessor  = (*commonTopo)(nil)
)

type topologySpec struct {
	Clusters map[string]clusterSpec
}

// serviceSpec defines a Consul service name and its Fortio listen port.
type serviceSpec struct {
	Name string
	Port int
}

type clusterSpec struct {
	Name       string
	Datacenter string
	Region     string
	// LocalityAwareLookup is dns_config.locality_aware_lookup for client agents in this cluster.
	LocalityAwareLookup string
	// ServiceAllowlist and ServiceBlocklist scope locality-aware lookup by exact service name.
	ServiceAllowlist []string
	ServiceBlocklist []string
	Zones            []string
	Nodes            []nodeSpec
}

type nodeSpec struct {
	Name      string
	Role      string
	Zone      string
	Workloads []serviceSpec
}

// NewCommonTopo builds the static deployer topology for locality-aware DNS
// integration tests (two datacenters, regions, zones, and Fortio workloads).
func NewCommonTopo(t *testing.T) *commonTopo {
	t.Helper()

	spec := newTopologySpec()
	clusters := make([]*topology.Cluster, 0, len(spec.Clusters))
	networks := make([]*topology.Network, 0, len(spec.Clusters))
	for _, cluster := range SortedByKey(spec.Clusters) {
		clusters = append(clusters, buildCluster(cluster))
		networks = append(networks, &topology.Network{Name: cluster.Datacenter})
	}

	return &commonTopo{
		Spec: spec,
		Cfg: &topology.Config{
			Images: topology.Images{
				ConsulCE: consulLocalImage,
			},
			Networks: networks,
			Clusters: clusters,
		},
	}
}

// Launch starts the Docker sprawl from ct.Cfg and populates ClustersLive with
// resolved query and service nodes per zone. It must be called at most once.
func (ct *commonTopo) Launch(t *testing.T) {
	t.Helper()
	if ct.Sprawl != nil {
		t.Fatalf("Launch must only be called once")
	}

	ct.Sprawl = sprawltest.Launch(t, ct.Cfg)

	if ct.ClustersLive == nil {
		ct.ClustersLive = make(map[string]*topology.Cluster)
	}

	for _, cluster := range SortedByKey(ct.Spec.Clusters) {
		live := ct.Sprawl.Topology().Clusters[cluster.Name]

		for _, node := range cluster.Nodes {
			liveNode := nodeByIDSafe(live, topology.NewNodeID(node.Name, ""))
			require.NotNil(t, liveNode, "missing live node %s in cluster %s", node.Name, cluster.Name)
		}

		ct.ClustersLive[cluster.Name] = live
	}

	ct.waitForPassingServices(t)
}

// waitForPassingServices blocks until the expected number of workloads are
// registered and passing before DNS locality assertions run.
func (ct *commonTopo) waitForPassingServices(t *testing.T) {
	t.Helper()

	for _, cluster := range ct.allClustersSpecs() {
		client := ct.APIClientForCluster(t, ct.clusterLive(cluster.Name))

		for _, svc := range topologyServices {
			expected := expectedWorkloadCount(cluster, svc)
			if expected == 0 {
				continue
			}

			retry.RunWith(&retry.Timer{Timeout: 2 * time.Minute, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
				entries, _, err := client.Health().Service(svc.Name, "", true, nil)
				require.NoError(r, err)
				require.Len(r, entries, expected,
					"want %d passing %s in %s, got %d", expected, svc.Name, cluster.Name, len(entries))
			})
		}
	}
}

func expectedWorkloadCount(cluster clusterSpec, svc serviceSpec) int {
	n := 0
	for _, node := range cluster.Nodes {
		for _, wrk := range node.Workloads {
			if wrk.Name == svc.Name {
				n++
			}
		}
	}
	return n
}

// expectedWorkloadsByZone returns the number of workloads for service per zone from
// the static topology spec (not the live catalog).
func expectedWorkloadsByZone(cluster clusterSpec, svc serviceSpec) map[string]int {
	counts := make(map[string]int)
	for _, node := range cluster.Nodes {
		for _, wrk := range node.Workloads {
			if wrk.Name == svc.Name {
				counts[node.Zone]++
			}
		}
	}
	return counts
}

// summarizeServiceEntries formats health catalog entries for assertion logs.
func summarizeServiceEntries(entries []*api.ServiceEntry) string {
	if len(entries) == 0 {
		return "<none>"
	}

	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		nodeName := "<nil-node>"
		dc := ""
		if entry.Node != nil {
			nodeName = entry.Node.Node
			dc = entry.Node.Datacenter
		}

		serviceID := "<nil-service>"
		if entry.Service != nil {
			serviceID = entry.Service.ID
		}

		checkStates := make([]string, 0, len(entry.Checks))
		for _, chk := range entry.Checks {
			checkStates = append(checkStates, fmt.Sprintf("%s=%s", chk.CheckID, chk.Status))
		}

		parts = append(parts, fmt.Sprintf("%s@%s service=%s checks=[%s]", nodeName, dc, serviceID, strings.Join(checkStates, ",")))
	}

	return strings.Join(parts, "; ")
}

// APIClientForCluster returns a Consul HTTP API client for the given cluster.
func (ct *commonTopo) APIClientForCluster(t *testing.T, clu *topology.Cluster) *api.Client {
	t.Helper()

	cl, err := ct.Sprawl.APIClientForCluster(clu.Name, "")
	require.NoError(t, err)
	return cl
}

// LogTopology writes deployed nodes (region, zone, ...) to the test log.
func (ct *commonTopo) LogTopology(t *testing.T) {
	t.Helper()

	for _, cluster := range SortedByKey(ct.ClustersLive) {
		t.Logf("cluster=%s datacenter=%s", cluster.Name, cluster.Datacenter)
		for _, node := range cluster.SortedNodes() {
			t.Logf(
				"    node=%s kind=%s container_name=%s region=%s zone=%s workloads=%s",
				node.Name,
				node.Kind,
				node.DockerName(),
				node.Meta["region"],
				node.Meta["zone"],
				workloadNames(node.Workloads),
			)
		}
	}
}

// LogManualDNSCommands prints docker exec + nslookup snippets for debugging from a client in each zone.
func (ct *commonTopo) LogManualDNSCommands(t *testing.T) {
	t.Helper()

	t.Logf("manual dns: the following commands reproduce DNS lookups from a client in each zone — they query the local Consul DNS agent at 127.0.0.1:8600 and print service records for %v", serviceNames(topologyServices))

	for _, cluster := range ct.allClustersSpecs() {
		for _, zone := range cluster.Zones {
			for _, node := range ct.NodesLive(cluster.Name, "client", "query", zone) {
				for _, svc := range topologyServices {
					t.Logf(
						"manual dns (%s %s %s %s): docker exec %s nslookup -type=A %s.service.consul 127.0.0.1#8600",
						cluster.Name,
						zone,
						svc.Name,
						node.DockerName(),
						node.DockerName(),
						svc.Name,
					)
				}
			}
		}
	}
}

// serviceNames returns the Consul service names from a slice of serviceSpec.
func serviceNames(services []serviceSpec) []string {
	names := make([]string, len(services))
	for i, svc := range services {
		names[i] = svc.Name
	}
	return names
}

// allClustersSpecs returns cluster declarations in deterministic name order.
func (ct *commonTopo) allClustersSpecs() []clusterSpec {
	return SortedByKey(ct.Spec.Clusters)
}

func (ct *commonTopo) clusterSpec(name string) clusterSpec {
	return ct.Spec.Clusters[name]
}

func (ct *commonTopo) clusterLive(name string) *topology.Cluster {
	return ct.ClustersLive[name]
}

func (ct *commonTopo) NodesSpecs(clusterName string, nodeRole string, nodeType string, zone string) []nodeSpec {

	Nodes := make([]nodeSpec, 0)

	for _, cluster := range ct.allClustersSpecs() {
		if cluster.Name == clusterName {
			for _, node := range cluster.Nodes {

				if nodeRole != "" && node.Role != nodeRole {
					continue
				}

				if zone != "" && node.Zone != zone {
					continue
				}

				if nodeType == "query" && len(node.Workloads) == 0 {
					Nodes = append(Nodes, node)
				} else if nodeType == "service" && len(node.Workloads) > 0 {
					Nodes = append(Nodes, node)
				} else if nodeType == "" {
					Nodes = append(Nodes, node)
				}
			}
		}
	}
	return Nodes
}

func (ct *commonTopo) NodesLive(clusterName string, nodeRole string, nodeType string, zone string) []*topology.Node {
	out := make([]*topology.Node, 0)

	live, ok := ct.ClustersLive[clusterName]
	if !ok || live == nil {
		return out
	}

	for _, ns := range ct.NodesSpecs(clusterName, nodeRole, nodeType, zone) {
		n := nodeByIDSafe(live, topology.NewNodeID(ns.Name, ""))
		if n != nil {
			out = append(out, n)
		}
	}
	return out
}

// nodeByIDSafe calls Cluster.NodeByID but recovers from the panic if the node
// isn't found, returning nil so tests can assert presence instead of panicking.
func nodeByIDSafe(c *topology.Cluster, nid topology.NodeID) (n *topology.Node) {
	defer func() {
		if r := recover(); r != nil {
			n = nil
		}
	}()
	n = c.NodeByID(nid)
	return
}

// SortedByKey returns the values of a map[string]V in order of sorted keys.
func SortedByKey[V any](m map[string]V) []V {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	vals := make([]V, 0, len(keys))
	for _, k := range keys {
		vals = append(vals, m[k])
	}
	return vals
}

// buildCluster converts a clusterSpec into a deployer topology.Cluster definition.
func buildCluster(spec clusterSpec) *topology.Cluster {
	nodes := make([]*topology.Node, 0, len(spec.Nodes))
	for _, node := range spec.Nodes {
		nodes = append(nodes, buildNode(spec, node))
	}

	return &topology.Cluster{
		Name:       spec.Name,
		Datacenter: spec.Datacenter,
		Nodes:      nodes,
	}
}

// buildNode materializes a deployer node with locality metadata, optional
// dns_config.locality_aware_lookup for clients, and workloads.
func buildNode(cluster clusterSpec, spec nodeSpec) *topology.Node {
	workloads := make([]*topology.Workload, 0, len(spec.Workloads))
	for _, svc := range spec.Workloads {
		workloads = append(workloads, serviceWorkload(cluster.Name, spec.Zone, svc))
	}

	return &topology.Node{
		Kind: nodeRole(spec.Role),
		Name: spec.Name,
		Meta: localityMeta(cluster.Region, spec.Zone),
		ExtraConfig: localityConfig(
			cluster.Region,
			spec.Zone,
			spec.Role,
			cluster.LocalityAwareLookup,
			cluster.ServiceAllowlist,
			cluster.ServiceBlocklist,
		),
		Addresses: []*topology.Address{
			{Network: cluster.Datacenter},
		},
		Workloads: workloads,
	}
}

// nodeRole maps spec role strings to deployer node kinds.
func nodeRole(role string) topology.NodeKind {
	switch role {
	case "server":
		return topology.NodeKindServer
	case "client":
		return topology.NodeKindClient
	default:
		panic(fmt.Sprintf("unsupported node role %q", role))
	}
}

// localityMeta builds Consul node meta keys for region and zone.
func localityMeta(region, zone string) map[string]string {
	return map[string]string{
		"region": region,
		"zone":   zone,
	}
}

// localityConfig returns an HCL snippet for agent locality and client DNS locality settings.
func localityConfig(
	region, zone, role, localityAwareLookup string,
	localityAwareAllowlist, localityAwareBlocklist []string,
) string {
	var b strings.Builder

	fmt.Fprintf(&b, "locality {\n  region = %q\n  zone = %q\n}\n", region, zone)
	if role == "client" && localityAwareLookup != "" {
		fmt.Fprintf(&b, "dns_config {\n  locality_aware_lookup = %q\n", localityAwareLookup)
		if len(localityAwareAllowlist) > 0 {
			fmt.Fprintf(&b, "  locality_aware_lookup_service_allowlist = [%s]\n", quoteList(localityAwareAllowlist))
		}
		if len(localityAwareBlocklist) > 0 {
			fmt.Fprintf(&b, "  locality_aware_lookup_service_blocklist = [%s]\n", quoteList(localityAwareBlocklist))
		}
		b.WriteString("}\n")
	}

	return b.String()
}

func quoteList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return strings.Join(quoted, ", ")
}

// serviceWorkload defines a Fortio workload registered under the given service.
func serviceWorkload(cluster, zone string, svc serviceSpec) *topology.Workload {
	wrk := &topology.Workload{
		ID:    topology.NewID(svc.Name, "default", "default"),
		Image: "docker.mirror.hashicorp.services/fortio/fortio",
		Env: []string{
			fmt.Sprintf("FORTIO_NAME=%s::%s::%s::%s", cluster, zone, svc.Name, generateUniqueString()),
		},
		DisableServiceMesh: true,
	}
	configureFortioPorts(wrk, svc.Port)
	return wrk
}

// generateUniqueString returns a short random suffix so each Fortio workload has a
// distinct FORTIO_NAME when multiple instances share cluster, zone, and service.
func generateUniqueString() string {
	const n = 8
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// configureFortioPorts sets HTTP/TCP/GRPC listen ports and health check for Fortio.
func configureFortioPorts(w *topology.Workload, httpPort int) {
	const (
		defaultGRPCOffset = -1
		defaultTCPOffset  = -2
	)

	w.Port = httpPort
	w.CheckTCP = fmt.Sprintf("127.0.0.1:%d", httpPort)
	w.EnvoyAdminPort = 0
	w.Command = []string{
		"server",
		"-http-port", strconv.Itoa(httpPort),
		"-grpc-port", strconv.Itoa(httpPort + defaultGRPCOffset),
		"-tcp-port", strconv.Itoa(httpPort + defaultTCPOffset),
		"-redirect-port", "-disabled",
	}
}

// workloadNames joins workload service names for logging.
func workloadNames(workloads []*topology.Workload) string {
	var names []string
	for _, wrk := range workloads {
		names = append(names, wrk.ID.Name)
	}
	return strings.Join(names, ",")
}

package topology

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-hclog"
)

const DockerPrefix = "consulcluster"

func Compile(logger hclog.Logger, raw *Config) (*Topology, error) {
	return compile(logger, raw, nil)
}

func Recompile(logger hclog.Logger, raw *Config, prev *Topology) (*Topology, error) {
	if prev == nil {
		return nil, errors.New("missing previous topology")
	}
	return compile(logger, raw, prev)
}

func compile(logger hclog.Logger, raw *Config, prev *Topology) (*Topology, error) {
	var id string
	if prev == nil {
		var err error
		id, err = newTopologyID()
		if err != nil {
			return nil, err
		}
	} else {
		id = prev.ID
	}

	images := DefaultImages().OverrideWith(raw.Images)
	if images.Consul != "" {
		return nil, fmt.Errorf("topology.images.consul cannot be set at this level")
	}

	if len(raw.Networks) == 0 {
		return nil, fmt.Errorf("topology.networks is empty")
	}

	networks := make(map[string]*Network)
	for _, net := range raw.Networks {
		if net.DockerName != "" {
			return nil, fmt.Errorf("network %q should not specify DockerName", net.Name)
		}
		if !IsValidLabel(net.Name) {
			return nil, fmt.Errorf("network name is not valid: %s", net.Name)
		}
		if _, exists := networks[net.Name]; exists {
			return nil, fmt.Errorf("cannot have two networks with the same name %q", net.Name)
		}

		switch net.Type {
		case "":
			net.Type = "lan"
		case "wan", "lan":
		default:
			return nil, fmt.Errorf("network %q has unknown type %q", net.Name, net.Type)
		}

		networks[net.Name] = net
		net.DockerName = DockerPrefix + "-" + net.Name + "-" + id
	}

	if len(raw.Clusters) == 0 {
		return nil, fmt.Errorf("topology.clusters is empty")
	}

	var (
		clusters  = make(map[string]*Cluster)
		nextIndex int // use a global index so any shared networks work properly with assignments
	)

	foundPeerNames := make(map[string]map[string]struct{})
	for _, c := range raw.Clusters {
		if c.Name == "" {
			return nil, fmt.Errorf("cluster has no name")
		}

		foundPeerNames[c.Name] = make(map[string]struct{})

		if !IsValidLabel(c.Name) {
			return nil, fmt.Errorf("cluster name is not valid: %s", c.Name)
		}

		if _, exists := clusters[c.Name]; exists {
			return nil, fmt.Errorf("cannot have two clusters with the same name %q; use unique names and override the Datacenter field if that's what you want", c.Name)
		}

		if c.Datacenter == "" {
			c.Datacenter = c.Name
		} else {
			if !IsValidLabel(c.Datacenter) {
				return nil, fmt.Errorf("datacenter name is not valid: %s", c.Datacenter)
			}
		}

		clusters[c.Name] = c
		if c.NetworkName == "" {
			c.NetworkName = c.Name
		}

		c.Images = images.OverrideWith(c.Images).ChooseConsul(c.Enterprise)

		if _, ok := networks[c.NetworkName]; !ok {
			return nil, fmt.Errorf("cluster %q uses network name %q that does not exist", c.Name, c.NetworkName)
		}

		if len(c.Nodes) == 0 {
			return nil, fmt.Errorf("cluster %q has no nodes", c.Name)
		}

		if c.TLSVolumeName != "" {
			return nil, fmt.Errorf("user cannot specify the TLSVolumeName field")
		}

		tenancies := make(map[string]map[string]struct{})
		addTenancy := func(partition, namespace string) {
			partition = PartitionOrDefault(partition)
			namespace = NamespaceOrDefault(namespace)
			m, ok := tenancies[partition]
			if !ok {
				m = make(map[string]struct{})
				tenancies[partition] = m
			}
			m[namespace] = struct{}{}
		}

		for _, ap := range c.Partitions {
			addTenancy(ap.Name, "default")
			for _, ns := range ap.Namespaces {
				addTenancy(ap.Name, ns)
			}
		}

		for _, ce := range c.InitialConfigEntries {
			addTenancy(ce.GetPartition(), ce.GetNamespace())
		}

		seenNodes := make(map[NodeID]struct{})
		for _, n := range c.Nodes {
			if n.Name == "" {
				return nil, fmt.Errorf("cluster %q node has no name", c.Name)
			}
			if !IsValidLabel(n.Name) {
				return nil, fmt.Errorf("node name is not valid: %s", n.Name)
			}

			switch n.Kind {
			case NodeKindServer, NodeKindClient, NodeKindDataplane:
			default:
				return nil, fmt.Errorf("cluster %q node %q has invalid kind: %s", c.Name, n.Name, n.Kind)
			}

			n.Partition = PartitionOrDefault(n.Partition)
			if !IsValidLabel(n.Partition) {
				return nil, fmt.Errorf("node partition is not valid: %s", n.Partition)
			}
			addTenancy(n.Partition, "default")

			if _, exists := seenNodes[n.ID()]; exists {
				return nil, fmt.Errorf("cannot have two nodes in the same cluster %q with the same name %q", c.Name, n.ID())
			}
			seenNodes[n.ID()] = struct{}{}

			if len(n.usedPorts) != 0 {
				return nil, fmt.Errorf("user cannot specify the usedPorts field")
			}
			n.usedPorts = make(map[int]int)
			exposePort := func(v int) bool {
				if _, ok := n.usedPorts[v]; ok {
					return false
				}
				n.usedPorts[v] = 0
				return true
			}

			if n.IsAgent() {
				// TODO: the ux  here is awful; we should be able to examine the topology to guess properly
				exposePort(8500)
				if n.IsServer() {
					exposePort(8503)
				} else {
					exposePort(8502)
				}
			}

			if n.Index != 0 {
				return nil, fmt.Errorf("user cannot specify the node index")
			}
			n.Index = nextIndex
			nextIndex++

			n.Images = c.Images.OverrideWith(n.Images).ChooseNode(n.Kind)

			n.Cluster = c.Name
			n.Datacenter = c.Datacenter
			n.dockerName = DockerPrefix + "-" + n.Name + "-" + id

			if len(n.Addresses) == 0 {
				n.Addresses = append(n.Addresses, &Address{Network: c.NetworkName})
			}
			var (
				numPublic int
				numLocal  int
			)
			for _, addr := range n.Addresses {
				if addr.Network == "" {
					return nil, fmt.Errorf("cluster %q node %q has invalid address", c.Name, n.Name)
				}

				if addr.Type != "" {
					return nil, fmt.Errorf("user cannot specify the address type directly")
				}

				net, ok := networks[addr.Network]
				if !ok {
					return nil, fmt.Errorf("cluster %q node %q uses network name %q that does not exist", c.Name, n.Name, addr.Network)
				}

				if net.IsPublic() {
					numPublic++
				} else if net.IsLocal() {
					numLocal++
				}
				addr.Type = net.Type

				addr.DockerNetworkName = net.DockerName
			}

			if numLocal == 0 {
				return nil, fmt.Errorf("cluster %q node %q has no local addresses", c.Name, n.Name)
			}
			if numPublic > 1 {
				return nil, fmt.Errorf("cluster %q node %q has more than one public address", c.Name, n.Name)
			}

			seenServices := make(map[ServiceID]struct{})
			for _, svc := range n.Services {
				if n.IsAgent() {
					// Default to that of the enclosing node.
					svc.ID.Partition = n.Partition
				}
				svc.ID.Normalize()

				// Denormalize
				svc.Node = n

				if !IsValidLabel(svc.ID.Partition) {
					return nil, fmt.Errorf("service partition is not valid: %s", svc.ID.Partition)
				}
				if !IsValidLabel(svc.ID.Namespace) {
					return nil, fmt.Errorf("service namespace is not valid: %s", svc.ID.Namespace)
				}
				if !IsValidLabel(svc.ID.Name) {
					return nil, fmt.Errorf("service name is not valid: %s", svc.ID.Name)
				}
				addTenancy(svc.ID.Partition, svc.ID.Namespace)

				if _, exists := seenServices[svc.ID]; exists {
					return nil, fmt.Errorf("cannot have two services on the same node %q in the same cluster %q with the same name %q", n.ID(), c.Name, svc.ID)
				}
				seenServices[svc.ID] = struct{}{}

				if !svc.DisableServiceMesh && n.IsDataplane() {
					if svc.EnvoyPublicListenerPort <= 0 {
						if _, ok := n.usedPorts[20000]; !ok {
							// For convenience the FIRST service on a node can get 20000 for free.
							svc.EnvoyPublicListenerPort = 20000
						} else {
							return nil, fmt.Errorf("envoy public listener port is required")
						}
					}
				}

				// add all of the service ports
				for _, port := range svc.ports() {
					if ok := exposePort(port); !ok {
						return nil, fmt.Errorf("port used more than once on cluster %q node %q: %d", c.Name, n.ID(), port)
					}
				}

				// TODO(rb): re-expose?
				// switch svc.Protocol {
				// case "":
				// 	svc.Protocol = "tcp"
				// 	fallthrough
				// case "tcp":
				// 	if svc.CheckHTTP != "" {
				// 		return nil, fmt.Errorf("cannot set CheckHTTP for tcp service")
				// 	}
				// case "http":
				// 	if svc.CheckTCP != "" {
				// 		return nil, fmt.Errorf("cannot set CheckTCP for tcp service")
				// 	}
				// default:
				// 	return nil, fmt.Errorf("service has invalid protocol: %s", svc.Protocol)
				// }

				for _, u := range svc.Upstreams {
					// Default to that of the enclosing service.
					if u.Peer == "" {
						if u.ID.Partition == "" {
							u.ID.Partition = svc.ID.Partition
						}
						if u.ID.Namespace == "" {
							u.ID.Namespace = svc.ID.Namespace
						}
					} else {
						if u.ID.Partition != "" {
							u.ID.Partition = "" // irrelevant here; we'll set it to the value of the OTHER side for plumbing purposes in tests
						}
						u.ID.Namespace = NamespaceOrDefault(u.ID.Namespace)
						foundPeerNames[c.Name][u.Peer] = struct{}{}
					}

					if u.ID.Name == "" {
						return nil, fmt.Errorf("upstream service name is required")
					}
					addTenancy(u.ID.Partition, u.ID.Namespace)
				}

				if err := svc.Validate(); err != nil {
					return nil, fmt.Errorf("cluster %q node %q service %q is not valid: %w", c.Name, n.Name, svc.ID.String(), err)
				}
			}
		}

		// Explode this into the explicit list based on stray references made.
		c.Partitions = nil
		for ap, nsMap := range tenancies {
			p := &Partition{
				Name: ap,
			}
			for ns := range nsMap {
				p.Namespaces = append(p.Namespaces, ns)
			}
			sort.Strings(p.Namespaces)
			c.Partitions = append(c.Partitions, p)
		}
		sort.Slice(c.Partitions, func(i, j int) bool {
			return c.Partitions[i].Name < c.Partitions[j].Name
		})

		if !c.Enterprise {
			expect := []*Partition{{Name: "default", Namespaces: []string{"default"}}}
			if !reflect.DeepEqual(c.Partitions, expect) {
				return nil, fmt.Errorf("cluster %q references non-default partitions or namespaces but is OSS", c.Name)
			}
		}
	}

	clusteredPeerings := make(map[string]map[string]*PeerCluster) // local-cluster -> local-peer -> info
	addPeerMapEntry := func(pc PeerCluster) {
		pm, ok := clusteredPeerings[pc.Name]
		if !ok {
			pm = make(map[string]*PeerCluster)
			clusteredPeerings[pc.Name] = pm
		}
		pm[pc.PeerName] = &pc
	}
	for _, p := range raw.Peerings {
		dialingCluster, ok := clusters[p.Dialing.Name]
		if !ok {
			return nil, fmt.Errorf("peering references a dialing cluster that does not exist: %s", p.Dialing.Name)
		}
		acceptingCluster, ok := clusters[p.Accepting.Name]
		if !ok {
			return nil, fmt.Errorf("peering references an accepting cluster that does not exist: %s", p.Accepting.Name)
		}
		if p.Dialing.Name == p.Accepting.Name {
			return nil, fmt.Errorf("self peerings are not allowed: %s", p.Dialing.Name)
		}

		p.Dialing.Partition = PartitionOrDefault(p.Dialing.Partition)
		p.Accepting.Partition = PartitionOrDefault(p.Accepting.Partition)

		if dialingCluster.Enterprise {
			if !dialingCluster.hasPartition(p.Dialing.Partition) {
				return nil, fmt.Errorf("dialing side of peering cannot reference a partition that does not exist: %s", p.Dialing.Partition)
			}
		} else {
			if p.Dialing.Partition != "default" {
				return nil, fmt.Errorf("dialing side of peering cannot reference a partition when OSS")
			}
		}
		if acceptingCluster.Enterprise {
			if !acceptingCluster.hasPartition(p.Accepting.Partition) {
				return nil, fmt.Errorf("accepting side of peering cannot reference a partition that does not exist: %s", p.Accepting.Partition)
			}
		} else {
			if p.Accepting.Partition != "default" {
				return nil, fmt.Errorf("accepting side of peering cannot reference a partition when OSS")
			}
		}

		if p.Dialing.PeerName == "" {
			p.Dialing.PeerName = "peer-" + p.Accepting.Name + "-" + p.Accepting.Partition
		}
		if p.Accepting.PeerName == "" {
			p.Accepting.PeerName = "peer-" + p.Dialing.Name + "-" + p.Dialing.Partition
		}

		{ // Ensure the link fields do not have recursive links.
			p.Dialing.Link = nil
			p.Accepting.Link = nil

			// Copy the un-linked data before setting the link
			pa := p.Accepting
			pd := p.Dialing

			p.Accepting.Link = &pd
			p.Dialing.Link = &pa
		}

		addPeerMapEntry(p.Accepting)
		addPeerMapEntry(p.Dialing)

		delete(foundPeerNames[p.Accepting.Name], p.Accepting.PeerName)
		delete(foundPeerNames[p.Dialing.Name], p.Dialing.PeerName)
	}

	for cluster, peers := range foundPeerNames {
		if len(peers) > 0 {
			var pretty []string
			for name := range peers {
				pretty = append(pretty, name)
			}
			sort.Strings(pretty)
			return nil, fmt.Errorf("cluster[%s] found topology references to peerings that do not exist: %v", cluster, pretty)
		}
	}

	// after we decoded the peering stuff, we can fill in some computed data in the upstreams
	for _, c := range clusters {
		c.Peerings = clusteredPeerings[c.Name]
		for _, n := range c.Nodes {
			for _, svc := range n.Services {
				for _, u := range svc.Upstreams {
					if u.Peer == "" {
						u.Cluster = c.Name
						u.Peering = nil
						continue
					}
					remotePeer, ok := c.Peerings[u.Peer]
					if !ok {
						return nil, fmt.Errorf("not possible")
					}
					u.Cluster = remotePeer.Link.Name
					u.Peering = remotePeer.Link
					// this helps in generating fortio assertions; otherwise field is ignored
					u.ID.Partition = remotePeer.Link.Partition
				}
			}
		}
	}

	t := &Topology{
		ID:       id,
		Networks: networks,
		Clusters: clusters,
		Images:   images,
		Peerings: raw.Peerings,
	}

	if prev != nil {
		// networks cannot change
		if !sameKeys(prev.Networks, t.Networks) {
			return nil, fmt.Errorf("cannot create or destroy networks")
		}

		for _, newNetwork := range t.Networks {
			oldNetwork := prev.Networks[newNetwork.Name]

			// Carryover
			newNetwork.inheritFromExisting(oldNetwork)

			if err := isSame(oldNetwork, newNetwork); err != nil {
				return nil, fmt.Errorf("networks cannot change: %w", err)
			}

		}

		// cannot add or remove an entire cluster
		if !sameKeys(prev.Clusters, t.Clusters) {
			return nil, fmt.Errorf("cannot create or destroy clusters")
		}

		for _, newCluster := range t.Clusters {
			oldCluster := prev.Clusters[newCluster.Name]

			// Carryover
			newCluster.inheritFromExisting(oldCluster)

			if newCluster.Name != oldCluster.Name ||
				newCluster.NetworkName != oldCluster.NetworkName ||
				newCluster.Datacenter != oldCluster.Datacenter ||
				newCluster.Enterprise != oldCluster.Enterprise {
				return nil, fmt.Errorf("cannot edit some cluster fields for %q", newCluster.Name)
			}

			// WARN on presence of some things.
			if len(newCluster.InitialConfigEntries) > 0 {
				logger.Warn("initial config entries were provided, but are skipped on recompile")
			}

			// Check NODES
			if err := inheritAndValidateNodes(oldCluster.Nodes, newCluster.Nodes); err != nil {
				return nil, fmt.Errorf("some immutable aspects of nodes were changed in cluster %q: %w", newCluster.Name, err)
			}
		}
	}

	return t, nil
}

const permutedWarning = "use the disabled node kind if you want to ignore a node"

func inheritAndValidateNodes(
	prev, curr []*Node,
) error {
	nodeMap := mapifyNodes(curr)

	for prevIdx, node := range prev {
		currNode, ok := nodeMap[node.ID()]
		if !ok {
			return fmt.Errorf("node %q has vanished; "+permutedWarning, node.ID())
		}
		// Ensure it hasn't been permuted.
		if currNode.Pos != prevIdx {
			return fmt.Errorf(
				"node %q has been shuffled %d -> %d; "+permutedWarning,
				node.ID(),
				prevIdx,
				currNode.Pos,
			)
		}

		if currNode.Node.Kind != node.Kind ||
			currNode.Node.Partition != node.Partition ||
			currNode.Node.Name != node.Name ||
			currNode.Node.Index != node.Index ||
			len(currNode.Node.Addresses) != len(node.Addresses) ||
			!sameKeys(currNode.Node.usedPorts, node.usedPorts) {
			return fmt.Errorf("cannot edit some node fields for %q", node.ID())
		}

		currNode.Node.inheritFromExisting(node)

		for i := 0; i < len(currNode.Node.Addresses); i++ {
			prevAddr := node.Addresses[i]
			currAddr := currNode.Node.Addresses[i]

			if prevAddr.Network != currAddr.Network {
				return fmt.Errorf("addresses were shuffled for node %q", node.ID())
			}

			if prevAddr.Type != currAddr.Type {
				return fmt.Errorf("cannot edit some address fields for %q", node.ID())
			}

			currAddr.inheritFromExisting(prevAddr)
		}

		svcMap := mapifyServices(currNode.Node.Services)

		for _, svc := range node.Services {
			currSvc, ok := svcMap[svc.ID]
			if !ok {
				continue // service has vanished, this is ok
			}
			// don't care about index permutation

			if currSvc.ID != svc.ID ||
				currSvc.Port != svc.Port ||
				currSvc.EnvoyAdminPort != svc.EnvoyAdminPort ||
				currSvc.EnvoyPublicListenerPort != svc.EnvoyPublicListenerPort ||
				isSame(currSvc.Command, svc.Command) != nil ||
				isSame(currSvc.Env, svc.Env) != nil {
				return fmt.Errorf("cannot edit some address fields for %q", svc.ID)
			}

			currSvc.inheritFromExisting(svc)
		}
	}
	return nil
}

func newTopologyID() (string, error) {
	const n = 16
	id := make([]byte, n)
	if _, err := crand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id)[:n], nil
}

// matches valid DNS labels according to RFC 1123, should be at most 63
// characters according to the RFC
var validLabel = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

// IsValidLabel returns true if the string given is a valid DNS label (RFC 1123).
// Note: the only difference between RFC 1035 and RFC 1123 labels is that in
// RFC 1123 labels can begin with a number.
func IsValidLabel(name string) bool {
	return validLabel.MatchString(name)
}

// ValidateLabel is similar to IsValidLabel except it returns an error
// instead of false when name is not a valid DNS label. The error will contain
// reference to what constitutes a valid DNS label.
func ValidateLabel(name string) error {
	if !IsValidLabel(name) {
		return errors.New("a valid DNS label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character")
	}
	return nil
}

func isSame(x, y any) error {
	diff := cmp.Diff(x, y)
	if diff != "" {
		return fmt.Errorf("values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
	return nil
}

func sameKeys[K comparable, V any](x, y map[K]V) bool {
	if len(x) != len(y) {
		return false
	}

	for kx := range x {
		if _, ok := y[kx]; !ok {
			return false
		}
	}
	return true
}

func mapifyNodes(nodes []*Node) map[NodeID]nodeWithPosition {
	m := make(map[NodeID]nodeWithPosition)
	for i, node := range nodes {
		m[node.ID()] = nodeWithPosition{
			Pos:  i,
			Node: node,
		}
	}
	return m
}

type nodeWithPosition struct {
	Pos  int
	Node *Node
}

func mapifyServices(services []*Service) map[ServiceID]*Service {
	m := make(map[ServiceID]*Service)
	for _, svc := range services {
		m[svc.ID] = svc
	}
	return m
}

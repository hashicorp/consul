// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/exp/maps"

	"github.com/hashicorp/consul/testing/deployer/util"
)

const DockerPrefix = "cslc" // ConSuLCluster

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

		if len(c.Services) == 0 { // always initialize this regardless of v2-ness, because we might late-enable it below
			c.Services = make(map[ID]*pbcatalog.Service)
		}

		var implicitV2Services bool
		if len(c.Services) > 0 {
			c.EnableV2 = true
			for name, svc := range c.Services {
				if svc.Workloads != nil {
					return nil, fmt.Errorf("the workloads field for v2 service %q is not user settable", name)
				}
			}
		} else {
			implicitV2Services = true
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

		if len(c.InitialResources) > 0 {
			c.EnableV2 = true
		}
		for _, res := range c.InitialResources {
			if res.Id.Tenancy == nil {
				res.Id.Tenancy = &pbresource.Tenancy{}
			}
			// TODO(peering/v2) prevent non-local peer resources
			res.Id.Tenancy.Partition = PartitionOrDefault(res.Id.Tenancy.Partition)
			if !util.IsTypePartitionScoped(res.Id.Type) {
				res.Id.Tenancy.Namespace = NamespaceOrDefault(res.Id.Tenancy.Namespace)
			}

			switch {
			case util.EqualType(pbauth.ComputedTrafficPermissionsType, res.Id.GetType()),
				util.EqualType(pbauth.WorkloadIdentityType, res.Id.GetType()):
				fallthrough
			case util.EqualType(pbmesh.ComputedRoutesType, res.Id.GetType()),
				util.EqualType(pbmesh.ProxyStateTemplateType, res.Id.GetType()):
				fallthrough
			case util.EqualType(pbcatalog.HealthChecksType, res.Id.GetType()),
				util.EqualType(pbcatalog.HealthStatusType, res.Id.GetType()),
				util.EqualType(pbcatalog.NodeType, res.Id.GetType()),
				util.EqualType(pbcatalog.ServiceEndpointsType, res.Id.GetType()),
				util.EqualType(pbcatalog.WorkloadType, res.Id.GetType()):
				return nil, fmt.Errorf("you should not create a resource of type %q this way", util.TypeToString(res.Id.Type))
			}

			addTenancy(res.Id.Tenancy.Partition, res.Id.Tenancy.Namespace)
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

			if n.Version == NodeVersionUnknown {
				n.Version = NodeVersionV1
			}
			switch n.Version {
			case NodeVersionV1:
			case NodeVersionV2:
				if n.Kind == NodeKindClient {
					return nil, fmt.Errorf("v2 does not support client agents at this time")
				}
				c.EnableV2 = true
			default:
				return nil, fmt.Errorf("cluster %q node %q has invalid version: %s", c.Name, n.Name, n.Version)
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

			n.Images = c.Images.OverrideWith(n.Images.ChooseConsul(c.Enterprise)).ChooseNode(n.Kind)

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

			if len(n.Services) > 0 {
				logger.Warn("please use Node.Workloads instead of Node.Services")
				n.Workloads = append(n.Workloads, n.Services...)
				n.Services = nil
			}

			if n.IsDataplane() && len(n.Workloads) > 1 {
				// Our use of consul-dataplane here is supposed to mimic that
				// of consul-k8s, which ultimately has one IP per Service, so
				// we introduce the same limitation here.
				return nil, fmt.Errorf("cluster %q node %q uses dataplane, but has more than one service", c.Name, n.Name)
			}

			seenServices := make(map[ID]struct{})
			for _, wrk := range n.Workloads {
				if n.IsAgent() {
					// Default to that of the enclosing node.
					wrk.ID.Partition = n.Partition
				}
				wrk.ID.Normalize()

				// Denormalize
				wrk.Node = n
				wrk.NodeVersion = n.Version
				if n.IsV2() {
					wrk.Workload = wrk.ID.Name + "-" + n.Name
				}

				if !IsValidLabel(wrk.ID.Partition) {
					return nil, fmt.Errorf("service partition is not valid: %s", wrk.ID.Partition)
				}
				if !IsValidLabel(wrk.ID.Namespace) {
					return nil, fmt.Errorf("service namespace is not valid: %s", wrk.ID.Namespace)
				}
				if !IsValidLabel(wrk.ID.Name) {
					return nil, fmt.Errorf("service name is not valid: %s", wrk.ID.Name)
				}
				if wrk.ID.Partition != n.Partition {
					return nil, fmt.Errorf("service %s on node %s has mismatched partitions: %s != %s",
						wrk.ID.Name, n.Name, wrk.ID.Partition, n.Partition)
				}
				addTenancy(wrk.ID.Partition, wrk.ID.Namespace)

				if _, exists := seenServices[wrk.ID]; exists {
					return nil, fmt.Errorf("cannot have two services on the same node %q in the same cluster %q with the same name %q", n.ID(), c.Name, wrk.ID)
				}
				seenServices[wrk.ID] = struct{}{}

				if !wrk.DisableServiceMesh && n.IsDataplane() {
					if wrk.EnvoyPublicListenerPort <= 0 {
						if _, ok := n.usedPorts[20000]; !ok {
							// For convenience the FIRST service on a node can get 20000 for free.
							wrk.EnvoyPublicListenerPort = 20000
						} else {
							return nil, fmt.Errorf("envoy public listener port is required")
						}
					}
				}

				// add all of the service ports
				for _, port := range wrk.ports() {
					if ok := exposePort(port); !ok {
						return nil, fmt.Errorf("port used more than once on cluster %q node %q: %d", c.Name, n.ID(), port)
					}
				}

				// TODO(rb): re-expose?
				// switch wrk.Protocol {
				// case "":
				// 	wrk.Protocol = "tcp"
				// 	fallthrough
				// case "tcp":
				// 	if wrk.CheckHTTP != "" {
				// 		return nil, fmt.Errorf("cannot set CheckHTTP for tcp service")
				// 	}
				// case "http":
				// 	if wrk.CheckTCP != "" {
				// 		return nil, fmt.Errorf("cannot set CheckTCP for tcp service")
				// 	}
				// default:
				// 	return nil, fmt.Errorf("service has invalid protocol: %s", wrk.Protocol)
				// }

				defaultDestination := func(dest *Destination) error {
					// Default to that of the enclosing service.
					if dest.Peer == "" {
						if dest.ID.Partition == "" {
							dest.ID.Partition = wrk.ID.Partition
						}
						if dest.ID.Namespace == "" {
							dest.ID.Namespace = wrk.ID.Namespace
						}
					} else {
						if dest.ID.Partition != "" {
							dest.ID.Partition = "" // irrelevant here; we'll set it to the value of the OTHER side for plumbing purposes in tests
						}
						dest.ID.Namespace = NamespaceOrDefault(dest.ID.Namespace)
						foundPeerNames[c.Name][dest.Peer] = struct{}{}
					}

					addTenancy(dest.ID.Partition, dest.ID.Namespace)

					if dest.Implied {
						if dest.PortName == "" {
							return fmt.Errorf("implicit destinations must use port names in v2")
						}
					} else {
						if dest.LocalAddress == "" {
							// v1 defaults to 127.0.0.1 but v2 does not. Safe to do this generally though.
							dest.LocalAddress = "127.0.0.1"
						}
						if dest.PortName != "" && n.IsV1() {
							return fmt.Errorf("explicit destinations cannot use port names in v1")
						}
						if dest.PortName == "" && n.IsV2() {
							// Assume this is a v1->v2 conversion and name it.
							dest.PortName = "legacy"
						}
					}

					return nil
				}

				for _, dest := range wrk.Destinations {
					if err := defaultDestination(dest); err != nil {
						return nil, err
					}
				}

				if n.IsV2() {
					for _, dest := range wrk.ImpliedDestinations {
						dest.Implied = true
						if err := defaultDestination(dest); err != nil {
							return nil, err
						}
					}
				} else {
					if len(wrk.ImpliedDestinations) > 0 {
						return nil, fmt.Errorf("v1 does not support implied destinations yet")
					}
				}

				if err := wrk.Validate(); err != nil {
					return nil, fmt.Errorf("cluster %q node %q service %q is not valid: %w", c.Name, n.Name, wrk.ID.String(), err)
				}

				if wrk.EnableTransparentProxy && !n.IsDataplane() {
					return nil, fmt.Errorf("cannot enable tproxy on a non-dataplane node")
				}

				if n.IsV2() {
					if implicitV2Services {
						wrk.V2Services = []string{wrk.ID.Name}

						var svcPorts []*pbcatalog.ServicePort
						for name, cfg := range wrk.Ports {
							svcPorts = append(svcPorts, &pbcatalog.ServicePort{
								TargetPort: name,
								Protocol:   cfg.ActualProtocol,
							})
						}

						v2svc := &pbcatalog.Service{
							Workloads: &pbcatalog.WorkloadSelector{},
							Ports:     svcPorts,
						}

						prev, ok := c.Services[wrk.ID]
						if !ok {
							c.Services[wrk.ID] = v2svc
							prev = v2svc
						}
						if prev.Workloads == nil {
							prev.Workloads = &pbcatalog.WorkloadSelector{}
						}
						prev.Workloads.Names = append(prev.Workloads.Names, wrk.Workload)

					} else {
						for _, name := range wrk.V2Services {
							v2ID := NewServiceID(name, wrk.ID.Namespace, wrk.ID.Partition)

							v2svc, ok := c.Services[v2ID]
							if !ok {
								return nil, fmt.Errorf("cluster %q node %q service %q has a v2 service reference that does not exist %q",
									c.Name, n.Name, wrk.ID.String(), name)
							}
							if v2svc.Workloads == nil {
								v2svc.Workloads = &pbcatalog.WorkloadSelector{}
							}
							v2svc.Workloads.Names = append(v2svc.Workloads.Names, wrk.Workload)
						}
					}

					if wrk.WorkloadIdentity == "" {
						wrk.WorkloadIdentity = wrk.ID.Name
					}
				} else {
					if len(wrk.V2Services) > 0 {
						return nil, fmt.Errorf("cannot specify v2 services for v1")
					}
					if wrk.WorkloadIdentity != "" {
						return nil, fmt.Errorf("cannot specify workload identities for v1")
					}
				}
			}
		}

		if err := assignVirtualIPs(c); err != nil {
			return nil, err
		}

		if c.EnableV2 {
			// Populate the VirtualPort field on all destinations.
			for _, n := range c.Nodes {
				for _, wrk := range n.Workloads {
					for _, dest := range wrk.ImpliedDestinations {
						res, ok := c.Services[dest.ID]
						if ok {
							for _, sp := range res.Ports {
								if sp.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
									continue
								}
								if sp.MatchesPortId(dest.PortName) {
									dest.VirtualPort = sp.VirtualPort
								}
							}
						}
					}
					for _, dest := range wrk.Destinations {
						res, ok := c.Services[dest.ID]
						if ok {
							for _, sp := range res.Ports {
								if sp.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
									continue
								}
								if sp.MatchesPortId(dest.PortName) {
									dest.VirtualPort = sp.VirtualPort
								}
							}
						}
					}
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
				return nil, fmt.Errorf("cluster %q references non-default partitions or namespaces but is CE", c.Name)
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
				return nil, fmt.Errorf("dialing side of peering cannot reference a partition when CE")
			}
		}
		if acceptingCluster.Enterprise {
			if !acceptingCluster.hasPartition(p.Accepting.Partition) {
				return nil, fmt.Errorf("accepting side of peering cannot reference a partition that does not exist: %s", p.Accepting.Partition)
			}
		} else {
			if p.Accepting.Partition != "default" {
				return nil, fmt.Errorf("accepting side of peering cannot reference a partition when CE")
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

	// after we decoded the peering stuff, we can fill in some computed data in the destinations
	for _, c := range clusters {
		c.Peerings = clusteredPeerings[c.Name]
		for _, n := range c.Nodes {
			for _, wrk := range n.Workloads {
				for _, dest := range wrk.Destinations {
					if dest.Peer == "" {
						dest.Cluster = c.Name
						dest.Peering = nil
						continue
					}
					remotePeer, ok := c.Peerings[dest.Peer]
					if !ok {
						return nil, fmt.Errorf("not possible")
					}
					dest.Cluster = remotePeer.Link.Name
					dest.Peering = remotePeer.Link
					// this helps in generating fortio assertions; otherwise field is ignored
					dest.ID.Partition = remotePeer.Link.Partition
				}
				for _, dest := range wrk.ImpliedDestinations {
					if dest.Peer == "" {
						dest.Cluster = c.Name
						dest.Peering = nil
						continue
					}
					remotePeer, ok := c.Peerings[dest.Peer]
					if !ok {
						return nil, fmt.Errorf("not possible")
					}
					dest.Cluster = remotePeer.Link.Name
					dest.Peering = remotePeer.Link
					// this helps in generating fortio assertions; otherwise field is ignored
					dest.ID.Partition = remotePeer.Link.Partition
				}
			}
		}
	}

	t := &Topology{
		ID:           id,
		Networks:     networks,
		Clusters:     clusters,
		Images:       images,
		Peerings:     raw.Peerings,
		NetworkAreas: raw.NetworkAreas,
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
			if len(newCluster.InitialResources) > 0 {
				logger.Warn("initial resources were provided, but are skipped on recompile")
			}

			// Check NODES
			if err := inheritAndValidateNodes(oldCluster.Nodes, newCluster.Nodes); err != nil {
				return nil, fmt.Errorf("some immutable aspects of nodes were changed in cluster %q: %w", newCluster.Name, err)
			}
		}
	}

	return t, nil
}

func assignVirtualIPs(c *Cluster) error {
	lastVIPIndex := 1
	for _, svcData := range c.Services {
		lastVIPIndex++
		if lastVIPIndex > 250 {
			return fmt.Errorf("too many ips using this approach to VIPs")
		}
		svcData.VirtualIps = []string{
			fmt.Sprintf("10.244.0.%d", lastVIPIndex),
		}

		// populate virtual ports where we forgot them
		var (
			usedPorts = make(map[uint32]struct{})
			next      = uint32(8080)
		)
		for _, sp := range svcData.Ports {
			if sp.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue
			}
			if sp.VirtualPort > 0 {
				usedPorts[sp.VirtualPort] = struct{}{}
			}
		}
		for _, sp := range svcData.Ports {
			if sp.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue
			}
			if sp.VirtualPort > 0 {
				continue
			}
		RETRY:
			attempt := next
			next++
			_, used := usedPorts[attempt]
			if used {
				goto RETRY
			}
			usedPorts[attempt] = struct{}{}
			sp.VirtualPort = attempt
		}
	}
	return nil
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
			currNode.Node.Version != node.Version ||
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

		wrkMap := mapifyWorkloads(currNode.Node.Workloads)

		for _, wrk := range node.Workloads {
			currWrk, ok := wrkMap[wrk.ID]
			if !ok {
				continue // service has vanished, this is ok
			}
			// don't care about index permutation

			if currWrk.ID != wrk.ID ||
				currWrk.Port != wrk.Port ||
				!maps.Equal(currWrk.Ports, wrk.Ports) ||
				currWrk.EnvoyAdminPort != wrk.EnvoyAdminPort ||
				currWrk.EnvoyPublicListenerPort != wrk.EnvoyPublicListenerPort ||
				isSame(currWrk.Command, wrk.Command) != nil ||
				isSame(currWrk.Env, wrk.Env) != nil {
				return fmt.Errorf("cannot edit some address fields for %q", wrk.ID)
			}

			currWrk.inheritFromExisting(wrk)
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

func mapifyWorkloads(workloads []*Service) map[ID]*Service {
	m := make(map[ID]*Service)
	for _, wrk := range workloads {
		m[wrk.ID] = wrk
	}
	return m
}

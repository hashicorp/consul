// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/consul/api"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Topology struct {
	ID string

	// Images controls which specific docker images are used when running this
	// node. Non-empty fields here override non-empty fields inherited from the
	// general default values from DefaultImages().
	Images Images

	// Networks is the list of networks to create for this set of clusters.
	Networks map[string]*Network

	// Clusters defines the list of Consul clusters that should be created, and
	// their associated workloads.
	Clusters map[string]*Cluster

	// Peerings defines the list of pairwise peerings that should be established
	// between clusters.
	Peerings []*Peering `json:",omitempty"`
}

func (t *Topology) DigestExposedProxyPort(netName string, proxyPort int) (bool, error) {
	net, ok := t.Networks[netName]
	if !ok {
		return false, fmt.Errorf("found output network that does not exist: %s", netName)
	}
	if net.ProxyPort == proxyPort {
		return false, nil
	}

	net.ProxyPort = proxyPort

	// Denormalize for UX.
	for _, cluster := range t.Clusters {
		for _, node := range cluster.Nodes {
			for _, addr := range node.Addresses {
				if addr.Network == netName {
					addr.ProxyPort = proxyPort
				}
			}
		}
	}

	return true, nil
}

func (t *Topology) SortedNetworks() []*Network {
	var out []*Network
	for _, n := range t.Networks {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (t *Topology) SortedClusters() []*Cluster {
	var out []*Cluster
	for _, c := range t.Clusters {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

type Config struct {
	// Images controls which specific docker images are used when running this
	// node. Non-empty fields here override non-empty fields inherited from the
	// general default values from DefaultImages().
	Images Images

	// Networks is the list of networks to create for this set of clusters.
	Networks []*Network

	// Clusters defines the list of Consul clusters that should be created, and
	// their associated workloads.
	Clusters []*Cluster

	// Peerings defines the list of pairwise peerings that should be established
	// between clusters.
	Peerings []*Peering
}

func (c *Config) Cluster(name string) *Cluster {
	for _, cluster := range c.Clusters {
		if cluster.Name == name {
			return cluster
		}
	}
	return nil
}

// DisableNode is a no-op if the node is already disabled.
func (c *Config) DisableNode(clusterName string, nid NodeID) (bool, error) {
	cluster := c.Cluster(clusterName)
	if cluster == nil {
		return false, fmt.Errorf("no such cluster: %q", clusterName)
	}

	for _, n := range cluster.Nodes {
		if n.ID() == nid {
			if n.Disabled {
				return false, nil
			}
			n.Disabled = true
			return true, nil
		}
	}

	return false, fmt.Errorf("expected to find nodeID %q in cluster %q", nid.String(), clusterName)
}

// EnableNode is a no-op if the node is already enabled.
func (c *Config) EnableNode(clusterName string, nid NodeID) (bool, error) {
	cluster := c.Cluster(clusterName)
	if cluster == nil {
		return false, fmt.Errorf("no such cluster: %q", clusterName)
	}

	for _, n := range cluster.Nodes {
		if n.ID() == nid {
			if !n.Disabled {
				return false, nil
			}
			n.Disabled = false
			return true, nil
		}
	}
	return false, fmt.Errorf("expected to find nodeID %q in cluster %q", nid.String(), clusterName)
}

type Network struct {
	Type string // lan/wan ; empty means lan
	Name string // logical name

	// computed at topology compile
	DockerName string
	// generated during network-and-tls
	Subnet string
	IPPool []string `json:"-"`
	// generated during network-and-tls
	ProxyAddress string `json:",omitempty"`
	DNSAddress   string `json:",omitempty"`
	// filled in from terraform outputs after network-and-tls
	ProxyPort int `json:",omitempty"`
}

func (n *Network) IsLocal() bool {
	return n.Type == "" || n.Type == "lan"
}

func (n *Network) IsPublic() bool {
	return n.Type == "wan"
}

func (n *Network) inheritFromExisting(existing *Network) {
	n.Subnet = existing.Subnet
	n.IPPool = existing.IPPool
	n.ProxyAddress = existing.ProxyAddress
	n.DNSAddress = existing.DNSAddress
	n.ProxyPort = existing.ProxyPort
}

func (n *Network) IPByIndex(index int) string {
	if index >= len(n.IPPool) {
		panic(fmt.Sprintf(
			"not enough ips on this network to assign index %d: %d",
			len(n.IPPool), index,
		))
	}
	return n.IPPool[index]
}

func (n *Network) SetSubnet(subnet string) (bool, error) {
	if n.Subnet == subnet {
		return false, nil
	}

	p, err := netip.ParsePrefix(subnet)
	if err != nil {
		return false, err
	}
	if !p.IsValid() {
		return false, errors.New("not valid")
	}
	p = p.Masked()

	var ipPool []string

	addr := p.Addr()
	for {
		if !p.Contains(addr) {
			break
		}
		ipPool = append(ipPool, addr.String())
		addr = addr.Next()
	}

	ipPool = ipPool[2:] // skip the x.x.x.{0,1}

	n.Subnet = subnet
	n.IPPool = ipPool
	return true, nil
}

// Cluster represents a single standalone install of Consul. This is the unit
// of what is peered when using cluster peering. Older consul installs would
// call this a datacenter.
type Cluster struct {
	Name        string
	NetworkName string // empty assumes same as Name

	// Images controls which specific docker images are used when running this
	// cluster. Non-empty fields here override non-empty fields inherited from
	// the enclosing Topology.
	Images Images

	// Enterprise marks this cluster as desiring to run Consul Enterprise
	// components.
	Enterprise bool `json:",omitempty"`

	// Services is a forward declaration of V2 services. This goes in hand with
	// the V2Services field on the Service (instance) struct.
	//
	// Use of this is optional. If you elect not to use it, then v2 Services
	// definitions are inferred from the list of service instances defined on
	// the nodes in this cluster.
	Services map[ServiceID]*pbcatalog.Service `json:"omitempty"`

	// Nodes is the definition of the nodes (agent-less and agent-ful).
	Nodes []*Node

	// Partitions is a list of tenancy configurations that should be created
	// after the servers come up but before the clients and the rest of the
	// topology starts.
	//
	// Enterprise Only.
	Partitions []*Partition `json:",omitempty"`

	// Datacenter defaults to "Name" if left unspecified. It lets you possibly
	// create multiple peer clusters with identical datacenter names.
	Datacenter string

	// InitialConfigEntries is a convenience mechanism to have some config
	// entries created after the servers start up but before the rest of the
	// topology comes up.
	InitialConfigEntries []api.ConfigEntry `json:",omitempty"`

	// InitialResources is a convenience mechanism to have some resources
	// created after the servers start up but before the rest of the topology
	// comes up.
	InitialResources []*pbresource.Resource `json:",omitempty"`

	// TLSVolumeName is the docker volume name containing the various certs
	// generated by 'consul tls cert create'
	//
	// This is generated during the networking phase and is not user specified.
	TLSVolumeName string `json:",omitempty"`

	// Peerings is a map of peering names to information about that peering in this cluster
	//
	// Denormalized during compile.
	Peerings map[string]*PeerCluster `json:",omitempty"`

	// EnableV2 activates V2 on the servers. If any node in the cluster needs
	// V2 this will be turned on automatically.
	EnableV2 bool `json:",omitempty"`
}

func (c *Cluster) inheritFromExisting(existing *Cluster) {
	c.TLSVolumeName = existing.TLSVolumeName
}

type Partition struct {
	Name       string
	Namespaces []string
}

func (c *Cluster) hasPartition(p string) bool {
	for _, partition := range c.Partitions {
		if partition.Name == p {
			return true
		}
	}
	return false
}

func (c *Cluster) PartitionQueryOptionsList() []*api.QueryOptions {
	if !c.Enterprise {
		return []*api.QueryOptions{{}}
	}

	var out []*api.QueryOptions
	for _, p := range c.Partitions {
		out = append(out, &api.QueryOptions{Partition: p.Name})
	}
	return out
}

func (c *Cluster) ServerNodes() []*Node {
	var out []*Node
	for _, node := range c.SortedNodes() {
		if node.Kind != NodeKindServer || node.Disabled {
			continue
		}
		out = append(out, node)
	}
	return out
}

func (c *Cluster) ServerByAddr(addr string) *Node {
	expect, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}

	for _, node := range c.Nodes {
		if node.Kind != NodeKindServer || node.Disabled {
			continue
		}
		if node.LocalAddress() == expect {
			return node
		}
	}

	return nil
}

func (c *Cluster) FirstServer() *Node {
	for _, node := range c.Nodes {
		// TODO: not sure why we check that it has 8500 exposed?
		if node.IsServer() && !node.Disabled && node.ExposedPort(8500) > 0 {
			return node
		}
	}
	return nil
}

func (c *Cluster) FirstClient() *Node {
	for _, node := range c.Nodes {
		if node.Kind != NodeKindClient || node.Disabled {
			continue
		}
		return node
	}
	return nil
}

func (c *Cluster) ActiveNodes() []*Node {
	var out []*Node
	for _, node := range c.Nodes {
		if !node.Disabled {
			out = append(out, node)
		}
	}
	return out
}

func (c *Cluster) SortedNodes() []*Node {
	var out []*Node
	out = append(out, c.Nodes...)

	kindOrder := map[NodeKind]int{
		NodeKindServer:    1,
		NodeKindClient:    2,
		NodeKindDataplane: 2,
	}
	sort.Slice(out, func(i, j int) bool {
		ni, nj := out[i], out[j]

		// servers before clients/dataplanes
		ki, kj := kindOrder[ni.Kind], kindOrder[nj.Kind]
		if ki < kj {
			return true
		} else if ki > kj {
			return false
		}

		// lex sort by partition
		if ni.Partition < nj.Partition {
			return true
		} else if ni.Partition > nj.Partition {
			return false
		}

		// lex sort by name
		return ni.Name < nj.Name
	})
	return out
}

func (c *Cluster) FindService(id NodeServiceID) *Service {
	id.Normalize()

	nid := id.NodeID()
	sid := id.ServiceID()
	return c.ServiceByID(nid, sid)
}

func (c *Cluster) ServiceByID(nid NodeID, sid ServiceID) *Service {
	return c.NodeByID(nid).ServiceByID(sid)
}

func (c *Cluster) ServicesByID(sid ServiceID) []*Service {
	sid.Normalize()

	var out []*Service
	for _, n := range c.Nodes {
		for _, svc := range n.Services {
			if svc.ID == sid {
				out = append(out, svc)
			}
		}
	}
	return out
}

func (c *Cluster) NodeByID(nid NodeID) *Node {
	nid.Normalize()
	for _, n := range c.Nodes {
		if n.ID() == nid {
			return n
		}
	}
	panic("node not found: " + nid.String())
}

type Address struct {
	Network string

	// denormalized at topology compile
	Type string
	// denormalized at topology compile
	DockerNetworkName string
	// generated after network-and-tls
	IPAddress string
	// denormalized from terraform outputs stored in the Network
	ProxyPort int `json:",omitempty"`
}

func (a *Address) inheritFromExisting(existing *Address) {
	a.IPAddress = existing.IPAddress
	a.ProxyPort = existing.ProxyPort
}

func (a Address) IsLocal() bool {
	return a.Type == "" || a.Type == "lan"
}

func (a Address) IsPublic() bool {
	return a.Type == "wan"
}

type NodeKind string

const (
	NodeKindUnknown   NodeKind = ""
	NodeKindServer    NodeKind = "server"
	NodeKindClient    NodeKind = "client"
	NodeKindDataplane NodeKind = "dataplane"
)

type NodeVersion string

const (
	NodeVersionUnknown NodeVersion = ""
	NodeVersionV1      NodeVersion = "v1"
	NodeVersionV2      NodeVersion = "v2"
)

// TODO: rename pod
type Node struct {
	Kind      NodeKind
	Version   NodeVersion
	Partition string // will be not empty
	Name      string // logical name

	// Images controls which specific docker images are used when running this
	// node. Non-empty fields here override non-empty fields inherited from
	// the enclosing Cluster.
	Images Images

	Disabled bool `json:",omitempty"`

	Addresses []*Address
	Services  []*Service

	// denormalized at topology compile
	Cluster    string
	Datacenter string

	// computed at topology compile
	Index int

	// generated during network-and-tls
	TLSCertPrefix string `json:",omitempty"`

	// dockerName is computed at topology compile
	dockerName string

	// usedPorts has keys that are computed at topology compile (internal
	// ports) and values initialized to zero until terraform creates the pods
	// and extracts the exposed port values from output variables.
	usedPorts map[int]int // keys are from compile / values are from terraform output vars
}

func (n *Node) DockerName() string {
	return n.dockerName
}

func (n *Node) ExposedPort(internalPort int) int {
	if internalPort == 0 {
		return 0
	}
	return n.usedPorts[internalPort]
}

func (n *Node) SortedPorts() []int {
	var out []int
	for internalPort := range n.usedPorts {
		out = append(out, internalPort)
	}
	sort.Ints(out)
	return out
}

func (n *Node) inheritFromExisting(existing *Node) {
	n.TLSCertPrefix = existing.TLSCertPrefix

	merged := existing.usedPorts
	for k, vNew := range n.usedPorts {
		if _, present := merged[k]; !present {
			merged[k] = vNew
		}
	}
	n.usedPorts = merged
}

func (n *Node) String() string {
	return n.ID().String()
}

func (n *Node) ID() NodeID {
	return NewNodeID(n.Name, n.Partition)
}

func (n *Node) CatalogID() NodeID {
	return NewNodeID(n.PodName(), n.Partition)
}

func (n *Node) PodName() string {
	return n.dockerName + "-pod"
}

func (n *Node) AddressByNetwork(name string) *Address {
	for _, a := range n.Addresses {
		if a.Network == name {
			return a
		}
	}
	return nil
}

func (n *Node) LocalAddress() string {
	for _, a := range n.Addresses {
		if a.IsLocal() {
			if a.IPAddress == "" {
				panic("node has no assigned local address: " + n.Name)
			}
			return a.IPAddress
		}
	}
	panic("node has no local network")
}

func (n *Node) HasPublicAddress() bool {
	for _, a := range n.Addresses {
		if a.IsPublic() {
			return true
		}
	}
	return false
}

func (n *Node) LocalProxyPort() int {
	for _, a := range n.Addresses {
		if a.IsLocal() {
			if a.ProxyPort > 0 {
				return a.ProxyPort
			}
			panic("node has no assigned local address: " + n.Name)
		}
	}
	panic("node has no local network")
}

func (n *Node) PublicAddress() string {
	for _, a := range n.Addresses {
		if a.IsPublic() {
			if a.IPAddress == "" {
				panic("node has no assigned public address")
			}
			return a.IPAddress
		}
	}
	panic("node has no public network")
}

func (n *Node) PublicProxyPort() int {
	for _, a := range n.Addresses {
		if a.IsPublic() {
			if a.ProxyPort > 0 {
				return a.ProxyPort
			}
			panic("node has no assigned public address")
		}
	}
	panic("node has no public network")
}

func (n *Node) IsV2() bool {
	return n.Version == NodeVersionV2
}

func (n *Node) IsV1() bool {
	return !n.IsV2()
}

func (n *Node) IsServer() bool {
	return n.Kind == NodeKindServer
}

func (n *Node) IsAgent() bool {
	return n.Kind == NodeKindServer || n.Kind == NodeKindClient
}

func (n *Node) RunsWorkloads() bool {
	return n.IsAgent() || n.IsDataplane()
}

func (n *Node) IsDataplane() bool {
	return n.Kind == NodeKindDataplane
}

func (n *Node) SortedServices() []*Service {
	var out []*Service
	out = append(out, n.Services...)
	sort.Slice(out, func(i, j int) bool {
		mi := out[i].IsMeshGateway
		mj := out[j].IsMeshGateway
		if mi && !mi {
			return false
		} else if !mi && mj {
			return true
		}
		return out[i].ID.Less(out[j].ID)
	})
	return out
}

// DigestExposedPorts returns true if it was changed.
func (n *Node) DigestExposedPorts(ports map[int]int) bool {
	if reflect.DeepEqual(n.usedPorts, ports) {
		return false
	}
	for internalPort := range n.usedPorts {
		if v, ok := ports[internalPort]; ok {
			n.usedPorts[internalPort] = v
		} else {
			panic(fmt.Sprintf(
				"cluster %q node %q port %d not found in exposed list",
				n.Cluster,
				n.ID(),
				internalPort,
			))
		}
	}
	for _, svc := range n.Services {
		svc.DigestExposedPorts(ports)
	}

	return true
}

func (n *Node) ServiceByID(sid ServiceID) *Service {
	sid.Normalize()
	for _, svc := range n.Services {
		if svc.ID == sid {
			return svc
		}
	}
	panic("service not found: " + sid.String())
}

type ServiceAndNode struct {
	Service *Service
	Node    *Node
}

// Protocol is a convenience function to use when authoring topology configs.
func Protocol(s string) (pbcatalog.Protocol, bool) {
	switch strings.ToLower(s) {
	case "tcp":
		return pbcatalog.Protocol_PROTOCOL_TCP, true
	case "http":
		return pbcatalog.Protocol_PROTOCOL_HTTP, true
	case "http2":
		return pbcatalog.Protocol_PROTOCOL_HTTP2, true
	case "grpc":
		return pbcatalog.Protocol_PROTOCOL_GRPC, true
	case "mesh":
		return pbcatalog.Protocol_PROTOCOL_MESH, true
	default:
		return pbcatalog.Protocol_PROTOCOL_UNSPECIFIED, false
	}
}

type Port struct {
	Number   int
	Protocol string `json:",omitempty"`

	// denormalized at topology compile
	ActualProtocol pbcatalog.Protocol `json:",omitempty"`
}

// TODO(rb): really this should now be called "workload" or "instance"
type Service struct {
	ID    ServiceID
	Image string

	// Port is the v1 single-port of this service.
	Port int `json:",omitempty"`

	// Ports is the v2 multi-port list for this service.
	//
	// This only applies for multi-port (v2).
	Ports map[string]*Port `json:",omitempty"`

	// ExposedPort is the exposed docker port corresponding to 'Port'.
	ExposedPort int `json:",omitempty"`

	// ExposedPorts are the exposed docker ports corresponding to 'Ports'.
	//
	// This only applies for multi-port (v2).
	ExposedPorts map[string]int `json:",omitempty"`

	// V2Services contains service names (which are merged with the tenancy
	// info from ID) to resolve services in the Services slice in the Cluster
	// definition.
	//
	// If omitted it is inferred that the ID.Name field is the singular service
	// for this workload.
	//
	// This only applies for multi-port (v2).
	V2Services []string `json:",omitempty"`

	// WorkloadIdentities contains named WorkloadIdentities to assign to this
	// workload.
	//
	// If omitted it is inferred that the ID.Name field is the singular
	// identity for this workload.
	//
	// This only applies for multi-port (v2).
	WorkloadIdentities []string `json:",omitempty"`

	Disabled bool `json:",omitempty"` // TODO

	// TODO: expose extra port here?

	Meta map[string]string `json:",omitempty"`

	// TODO(rb): re-expose this perhaps? Protocol  string `json:",omitempty"` // tcp|http (empty == tcp)
	CheckHTTP string `json:",omitempty"` // url; will do a GET
	CheckTCP  string `json:",omitempty"` // addr; will do a socket open/close

	EnvoyAdminPort          int
	ExposedEnvoyAdminPort   int `json:",omitempty"`
	EnvoyPublicListenerPort int `json:",omitempty"` // agentless

	Command []string `json:",omitempty"` // optional
	Env     []string `json:",omitempty"` // optional

	EnableTransparentProxy bool        `json:",omitempty"`
	DisableServiceMesh     bool        `json:",omitempty"`
	IsMeshGateway          bool        `json:",omitempty"`
	Upstreams              []*Upstream `json:",omitempty"`
	ImpliedUpstreams       []*Upstream `json:",omitempty"`

	// denormalized at topology compile
	Node        *Node       `json:"-"`
	NodeVersion NodeVersion `json:"-"`
	Workload    string      `json:"-"`
}

func (s *Service) PortOrDefault(name string) int {
	if len(s.Ports) > 0 {
		return s.Ports[name].Number
	}
	return s.Port
}

func (s *Service) IsV2() bool {
	return s.NodeVersion == NodeVersionV2
}

func (s *Service) IsV1() bool {
	return !s.IsV2()
}

func (s *Service) inheritFromExisting(existing *Service) {
	s.ExposedPort = existing.ExposedPort
	s.ExposedPorts = existing.ExposedPorts
	s.ExposedEnvoyAdminPort = existing.ExposedEnvoyAdminPort
}

func (s *Service) ports() []int {
	var out []int
	if len(s.Ports) > 0 {
		seen := make(map[int]struct{})
		for _, port := range s.Ports {
			if _, ok := seen[port.Number]; !ok {
				// It's totally fine to expose the same port twice in a workload.
				seen[port.Number] = struct{}{}
				out = append(out, port.Number)
			}
		}
	} else if s.Port > 0 {
		out = append(out, s.Port)
	}
	if s.EnvoyAdminPort > 0 {
		out = append(out, s.EnvoyAdminPort)
	}
	if s.EnvoyPublicListenerPort > 0 {
		out = append(out, s.EnvoyPublicListenerPort)
	}
	for _, u := range s.Upstreams {
		if u.LocalPort > 0 {
			out = append(out, u.LocalPort)
		}
	}
	return out
}

func (s *Service) HasCheck() bool {
	return s.CheckTCP != "" || s.CheckHTTP != ""
}

func (s *Service) DigestExposedPorts(ports map[int]int) {
	s.ExposedPort = ports[s.Port]
	for portName, port := range s.Ports {
		s.ExposedPorts[portName] = ports[port.Number]
	}
	if s.EnvoyAdminPort > 0 {
		s.ExposedEnvoyAdminPort = ports[s.EnvoyAdminPort]
	} else {
		s.ExposedEnvoyAdminPort = 0
	}
}

func (s *Service) Validate() error {
	if s.ID.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if s.Image == "" && !s.IsMeshGateway {
		return fmt.Errorf("service image is required")
	}
	if s.IsV2() {
		if len(s.Ports) > 0 && s.Port > 0 {
			return fmt.Errorf("cannot specify both singleport and multiport on service in v2")
		}
		if s.Port > 0 {
			s.Ports = map[string]*Port{
				"legacy": {
					Number:   s.Port,
					Protocol: "tcp",
				},
			}
			s.Port = 0
		}

		if !s.DisableServiceMesh && s.EnvoyPublicListenerPort > 0 {
			s.Ports["mesh"] = &Port{
				Number:   s.EnvoyPublicListenerPort,
				Protocol: "mesh",
			}
		}

		for name, port := range s.Ports {
			if port == nil {
				return fmt.Errorf("cannot be nil")
			}
			if port.Number <= 0 {
				return fmt.Errorf("service has invalid port number %q", name)
			}
			if port.ActualProtocol != pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
				return fmt.Errorf("user cannot specify ActualProtocol field")
			}

			proto, valid := Protocol(port.Protocol)
			if !valid {
				return fmt.Errorf("service has invalid port protocol %q", port.Protocol)
			}
			port.ActualProtocol = proto
		}
	} else {
		if len(s.Ports) > 0 {
			return fmt.Errorf("cannot specify mulitport on service in v1")
		}
		if s.Port <= 0 {
			return fmt.Errorf("service has invalid port")
		}
		if s.EnableTransparentProxy {
			return fmt.Errorf("tproxy does not work with v1 yet")
		}
	}
	if s.DisableServiceMesh && s.IsMeshGateway {
		return fmt.Errorf("cannot disable service mesh and still run a mesh gateway")
	}
	if s.DisableServiceMesh && len(s.Upstreams) > 0 {
		return fmt.Errorf("cannot disable service mesh and configure upstreams")
	}
	if s.DisableServiceMesh && len(s.ImpliedUpstreams) > 0 {
		return fmt.Errorf("cannot disable service mesh and configure implied upstreams")
	}
	if s.DisableServiceMesh && s.EnableTransparentProxy {
		return fmt.Errorf("cannot disable service mesh and activate tproxy")
	}

	if s.DisableServiceMesh {
		if s.EnvoyAdminPort != 0 {
			return fmt.Errorf("cannot use envoy admin port without a service mesh")
		}
	} else {
		if s.EnvoyAdminPort <= 0 {
			return fmt.Errorf("envoy admin port is required")
		}
	}

	for _, u := range s.Upstreams {
		if u.ID.Name == "" {
			return fmt.Errorf("upstream service name is required")
		}
		if u.LocalPort <= 0 {
			return fmt.Errorf("upstream local port is required")
		}

		if u.LocalAddress != "" {
			ip := net.ParseIP(u.LocalAddress)
			if ip == nil {
				return fmt.Errorf("upstream local address is invalid: %s", u.LocalAddress)
			}
		}
		if u.Implied {
			return fmt.Errorf("implied field cannot be set")
		}
	}
	for _, u := range s.ImpliedUpstreams {
		if u.ID.Name == "" {
			return fmt.Errorf("implied upstream service name is required")
		}
		if u.LocalPort > 0 {
			return fmt.Errorf("implied upstream local port cannot be set")
		}
		if u.LocalAddress != "" {
			return fmt.Errorf("implied upstream local address cannot be set")
		}
	}

	return nil
}

type Upstream struct {
	ID           ServiceID
	LocalAddress string `json:",omitempty"` // defaults to 127.0.0.1
	LocalPort    int
	Peer         string `json:",omitempty"`

	// PortName is the named port of this Upstream to route traffic to.
	//
	// This only applies for multi-port (v2).
	PortName string `json:",omitempty"`
	// TODO: what about mesh gateway mode overrides?

	// computed at topology compile
	Cluster     string       `json:",omitempty"`
	Peering     *PeerCluster `json:",omitempty"` // this will have Link!=nil
	Implied     bool         `json:",omitempty"`
	VirtualPort uint32       `json:",omitempty"`
}

type Peering struct {
	Dialing   PeerCluster
	Accepting PeerCluster
}

type PeerCluster struct {
	Name      string
	Partition string
	PeerName  string // name to call it on this side; defaults if not specified

	// computed at topology compile (pointer so it can be empty in json)
	Link *PeerCluster `json:",omitempty"`
}

func (c PeerCluster) String() string {
	return c.Name + ":" + c.Partition
}

func (p *Peering) String() string {
	return "(" + p.Dialing.String() + ")->(" + p.Accepting.String() + ")"
}

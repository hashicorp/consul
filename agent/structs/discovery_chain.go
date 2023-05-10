// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib"
)

// CompiledDiscoveryChain is the result from taking a set of related config
// entries for a single service's discovery chain and restructuring them into a
// form that is more usable for actual service discovery.
type CompiledDiscoveryChain struct {
	ServiceName string
	Namespace   string // the namespace that the chain was compiled within
	Partition   string // the partition that the chain was compiled within
	Datacenter  string // the datacenter that the chain was compiled within

	// CustomizationHash is a unique hash of any data that affects the
	// compilation of the discovery chain other than config entries or the
	// name/namespace/datacenter evaluation criteria.
	//
	// If set, this value should be used to prefix/suffix any generated load
	// balancer data plane objects to avoid sharing customized and
	// non-customized versions.
	CustomizationHash string `json:",omitempty"`

	// Default indicates if this discovery chain is based on no
	// service-resolver, service-splitter, or service-router config entries.
	Default bool `json:",omitempty"`

	// Protocol is the overall protocol shared by everything in the chain.
	Protocol string `json:",omitempty"`

	// ServiceMeta is the metadata from the underlying service-defaults config
	// entry for the service named ServiceName.
	ServiceMeta map[string]string `json:",omitempty"`

	// EnvoyExtensions has a list of configurations for an extension that patches Envoy resources.
	EnvoyExtensions []EnvoyExtension `json:",omitempty"`

	// StartNode is the first key into the Nodes map that should be followed
	// when walking the discovery chain.
	StartNode string `json:",omitempty"`

	// Nodes contains all nodes available for traversal in the chain keyed by a
	// unique name.  You can walk this by starting with StartNode.
	//
	// NOTE: The names should be treated as opaque values and are only
	// guaranteed to be consistent within a single compilation.
	Nodes map[string]*DiscoveryGraphNode `json:",omitempty"`

	// Targets is a list of all targets used in this chain.
	Targets map[string]*DiscoveryTarget `json:",omitempty"`

	// VirtualIPs is a list of virtual IPs associated with the service.
	AutoVirtualIPs   []string
	ManualVirtualIPs []string
}

// ID returns an ID that encodes the service, namespace, partition, and datacenter.
// This ID allows us to compare a discovery chain target to the chain upstream itself.
func (c *CompiledDiscoveryChain) ID() string {
	return ChainID(DiscoveryTargetOpts{
		Service:    c.ServiceName,
		Namespace:  c.Namespace,
		Partition:  c.Partition,
		Datacenter: c.Datacenter,
	})
}

func (c *CompiledDiscoveryChain) CompoundServiceName() ServiceName {
	entMeta := acl.NewEnterpriseMetaWithPartition(c.Partition, c.Namespace)
	return NewServiceName(c.ServiceName, &entMeta)
}

const (
	DiscoveryGraphNodeTypeRouter   = "router"
	DiscoveryGraphNodeTypeSplitter = "splitter"
	DiscoveryGraphNodeTypeResolver = "resolver"
)

// DiscoveryGraphNode is a single node in the compiled discovery chain.
type DiscoveryGraphNode struct {
	Type string
	Name string // this is NOT necessarily a service

	// fields for Type==router
	Routes []*DiscoveryRoute `json:",omitempty"`

	// fields for Type==splitter
	Splits []*DiscoverySplit `json:",omitempty"`

	// fields for Type==resolver
	Resolver *DiscoveryResolver `json:",omitempty"`

	// shared by Type==resolver || Type==splitter
	LoadBalancer *LoadBalancer `json:",omitempty"`
}

func (s *DiscoveryGraphNode) IsRouter() bool {
	return s.Type == DiscoveryGraphNodeTypeRouter
}

func (s *DiscoveryGraphNode) IsSplitter() bool {
	return s.Type == DiscoveryGraphNodeTypeSplitter
}

func (s *DiscoveryGraphNode) IsResolver() bool {
	return s.Type == DiscoveryGraphNodeTypeResolver
}

func (s *DiscoveryGraphNode) MapKey() string {
	return fmt.Sprintf("%s:%s", s.Type, s.Name)
}

// compiled form of ServiceResolverConfigEntry
type DiscoveryResolver struct {
	Default              bool                           `json:",omitempty"`
	ConnectTimeout       time.Duration                  `json:",omitempty"`
	RequestTimeout       time.Duration                  `json:",omitempty"`
	Target               string                         `json:",omitempty"`
	Failover             *DiscoveryFailover             `json:",omitempty"`
	PrioritizeByLocality *DiscoveryPrioritizeByLocality `json:",omitempty"`
}

func (r *DiscoveryResolver) MarshalJSON() ([]byte, error) {
	type Alias DiscoveryResolver
	exported := &struct {
		ConnectTimeout string `json:",omitempty"`
		*Alias
	}{
		ConnectTimeout: r.ConnectTimeout.String(),
		Alias:          (*Alias)(r),
	}
	if r.ConnectTimeout == 0 {
		exported.ConnectTimeout = ""
	}

	return json.Marshal(exported)
}

func (r *DiscoveryResolver) UnmarshalJSON(data []byte) error {
	type Alias DiscoveryResolver
	aux := &struct {
		ConnectTimeout string
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.ConnectTimeout != "" {
		if r.ConnectTimeout, err = time.ParseDuration(aux.ConnectTimeout); err != nil {
			return err
		}
	}
	return nil
}

// compiled form of ServiceRoute
type DiscoveryRoute struct {
	Definition *ServiceRoute `json:",omitempty"`
	NextNode   string        `json:",omitempty"`
}

// compiled form of ServiceSplit
type DiscoverySplit struct {
	Definition *ServiceSplit `json:",omitempty"`
	// Weight is not necessarily a duplicate of Definition.Weight since when
	// multiple splits are compiled down to a single set of splits the effective
	// weight of a split leg might not be the same as in the original definition.
	// Proxies should use this compiled weight. The Definition is provided above
	// for any other significant configuration that the proxy might need to apply
	// to that leg of the split.
	Weight   float32 `json:",omitempty"`
	NextNode string  `json:",omitempty"`
}

// compiled form of ServiceResolverFailover
type DiscoveryFailover struct {
	Targets []string                       `json:",omitempty"`
	Policy  *ServiceResolverFailoverPolicy `json:",omitempty"`
	Regions []string                       `json:",omitempty"`
}

// compiled form of ServiceResolverPrioritizeByLocality
type DiscoveryPrioritizeByLocality struct {
	Mode string `json:",omitempty"`
}

func (pbl *ServiceResolverPrioritizeByLocality) ToDiscovery() *DiscoveryPrioritizeByLocality {
	if pbl == nil {
		return nil
	}
	return &DiscoveryPrioritizeByLocality{
		Mode: pbl.Mode,
	}
}

// DiscoveryTarget represents all of the inputs necessary to use a resolver
// config entry to execute a catalog query to generate a list of service
// instances during discovery.
type DiscoveryTarget struct {
	// ID is a unique identifier for referring to this target in a compiled
	// chain. It should be treated as a per-compile opaque string.
	ID string `json:",omitempty"`

	Service       string    `json:",omitempty"`
	ServiceSubset string    `json:",omitempty"`
	Namespace     string    `json:",omitempty"`
	Partition     string    `json:",omitempty"`
	Datacenter    string    `json:",omitempty"`
	Peer          string    `json:",omitempty"`
	Locality      *Locality `json:",omitempty"`

	MeshGateway      MeshGatewayConfig      `json:",omitempty"`
	Subset           ServiceResolverSubset  `json:",omitempty"`
	TransparentProxy TransparentProxyConfig `json:",omitempty"`

	ConnectTimeout time.Duration `json:",omitempty"`

	// External is true if this target is outside of this consul cluster.
	External bool `json:",omitempty"`

	// SNI is the sni field to use when connecting to this set of endpoints
	// over TLS.
	SNI string `json:",omitempty"`

	// Name is the unique name for this target for use when generating load
	// balancer objects.  This has a structure similar to SNI, but will not be
	// affected by SNI customizations.
	Name string `json:",omitempty"`
}

func (t *DiscoveryTarget) MarshalJSON() ([]byte, error) {
	type Alias DiscoveryTarget
	exported := struct {
		ConnectTimeout string `json:",omitempty"`
		*Alias
	}{
		ConnectTimeout: t.ConnectTimeout.String(),
		Alias:          (*Alias)(t),
	}
	if t.ConnectTimeout == 0 {
		exported.ConnectTimeout = ""
	}

	return json.Marshal(exported)
}

func (t *DiscoveryTarget) UnmarshalJSON(data []byte) error {
	type Alias DiscoveryTarget
	aux := &struct {
		ConnectTimeout string
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	var err error
	if aux.ConnectTimeout != "" {
		if t.ConnectTimeout, err = time.ParseDuration(aux.ConnectTimeout); err != nil {
			return err
		}
	}
	return nil
}

type DiscoveryTargetOpts struct {
	Service       string
	ServiceSubset string
	Namespace     string
	Partition     string
	Datacenter    string
	Peer          string
}

func MergeDiscoveryTargetOpts(opts ...DiscoveryTargetOpts) DiscoveryTargetOpts {
	var final DiscoveryTargetOpts
	for _, o := range opts {
		if o.Service != "" {
			final.Service = o.Service
		}

		if o.ServiceSubset != "" {
			final.ServiceSubset = o.ServiceSubset
		}

		// default should override the existing value
		if o.Namespace != "" {
			final.Namespace = o.Namespace
		}

		// default should override the existing value
		if o.Partition != "" {
			final.Partition = o.Partition
		}

		if o.Datacenter != "" {
			final.Datacenter = o.Datacenter
		}

		if o.Peer != "" {
			final.Peer = o.Peer
		}
	}

	return final
}

func NewDiscoveryTarget(opts DiscoveryTargetOpts) *DiscoveryTarget {
	t := &DiscoveryTarget{
		Service:       opts.Service,
		ServiceSubset: opts.ServiceSubset,
		Namespace:     opts.Namespace,
		Partition:     opts.Partition,
		Datacenter:    opts.Datacenter,
		Peer:          opts.Peer,
	}
	t.setID()
	return t
}

func (t *DiscoveryTarget) ToDiscoveryTargetOpts() DiscoveryTargetOpts {
	return DiscoveryTargetOpts{
		Service:       t.Service,
		ServiceSubset: t.ServiceSubset,
		Namespace:     t.Namespace,
		Partition:     t.Partition,
		Datacenter:    t.Datacenter,
		Peer:          t.Peer,
	}
}

func ChainID(opts DiscoveryTargetOpts) string {
	// NOTE: this format is similar to the SNI syntax for simplicity
	if opts.Peer != "" {
		return fmt.Sprintf("%s.%s.%s.external.%s", opts.Service, opts.Namespace, opts.Partition, opts.Peer)
	}
	if opts.ServiceSubset == "" {
		return fmt.Sprintf("%s.%s.%s.%s", opts.Service, opts.Namespace, opts.Partition, opts.Datacenter)
	}
	return fmt.Sprintf("%s.%s.%s.%s.%s", opts.ServiceSubset, opts.Service, opts.Namespace, opts.Partition, opts.Datacenter)
}

func (t *DiscoveryTarget) setID() {
	t.ID = ChainID(t.ToDiscoveryTargetOpts())
}

func (t *DiscoveryTarget) String() string {
	return t.ID
}

func (t *DiscoveryTarget) ServiceID() ServiceID {
	return NewServiceID(t.Service, t.GetEnterpriseMetadata())
}

func (t *DiscoveryTarget) ServiceName() ServiceName {
	return NewServiceName(t.Service, t.GetEnterpriseMetadata())
}

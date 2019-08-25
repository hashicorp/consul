package structs

import (
	"encoding/json"
	"fmt"
	"time"
)

// CompiledDiscoveryChain is the result from taking a set of related config
// entries for a single service's discovery chain and restructuring them into a
// form that is more usable for actual service discovery.
type CompiledDiscoveryChain struct {
	ServiceName string
	Namespace   string // the namespace that the chain was compiled within
	Datacenter  string // the datacenter that the chain was compiled within

	// CustomizationHash is a unique hash of any data that affects the
	// compilation of the discovery chain other than config entries or the
	// name/namespace/datacenter evaluation criteria.
	//
	// If set, this value should be used to prefix/suffix any generated load
	// balancer data plane objects to avoid sharing customized and
	// non-customized versions.
	CustomizationHash string `json:",omitempty"`

	// Protocol is the overall protocol shared by everything in the chain.
	Protocol string `json:",omitempty"`

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
}

func (c *CompiledDiscoveryChain) WillFailoverThroughMeshGateway(node *DiscoveryGraphNode) bool {
	if node.Type != DiscoveryGraphNodeTypeResolver {
		return false
	}
	failover := node.Resolver.Failover

	if failover != nil && len(failover.Targets) > 0 {
		for _, failTargetID := range failover.Targets {
			failTarget := c.Targets[failTargetID]
			switch failTarget.MeshGateway.Mode {
			case MeshGatewayModeLocal, MeshGatewayModeRemote:
				return true
			}
		}
	}
	return false
}

// IsDefault returns true if the compiled chain represents no routing, no
// splitting, and only the default resolution.  We have to be careful here to
// avoid returning "yep this is default" when the only resolver action being
// applied is redirection to another resolver that is default, so we double
// check the resolver matches the requested resolver.
func (c *CompiledDiscoveryChain) IsDefault() bool {
	if c.StartNode == "" || len(c.Nodes) == 0 {
		return true
	}

	node := c.Nodes[c.StartNode]
	if node == nil {
		panic("not possible: missing node named '" + c.StartNode + "' in chain '" + c.ServiceName + "'")
	}

	if node.Type != DiscoveryGraphNodeTypeResolver {
		return false
	}
	if !node.Resolver.Default {
		return false
	}

	target := c.Targets[node.Resolver.Target]

	return target.Service == c.ServiceName
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
	Default        bool               `json:",omitempty"`
	ConnectTimeout time.Duration      `json:",omitempty"`
	Target         string             `json:",omitempty"`
	Failover       *DiscoveryFailover `json:",omitempty"`
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
	if err := json.Unmarshal(data, &aux); err != nil {
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
	Weight   float32 `json:",omitempty"`
	NextNode string  `json:",omitempty"`
}

// compiled form of ServiceResolverFailover
type DiscoveryFailover struct {
	Targets []string `json:",omitempty"`
}

// DiscoveryTarget represents all of the inputs necessary to use a resolver
// config entry to execute a catalog query to generate a list of service
// instances during discovery.
type DiscoveryTarget struct {
	// ID is a unique identifier for referring to this target in a compiled
	// chain. It should be treated as a per-compile opaque string.
	ID string `json:",omitempty"`

	Service       string `json:",omitempty"`
	ServiceSubset string `json:",omitempty"`
	Namespace     string `json:",omitempty"`
	Datacenter    string `json:",omitempty"`

	MeshGateway MeshGatewayConfig     `json:",omitempty"`
	Subset      ServiceResolverSubset `json:",omitempty"`

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

func NewDiscoveryTarget(service, serviceSubset, namespace, datacenter string) *DiscoveryTarget {
	t := &DiscoveryTarget{
		Service:       service,
		ServiceSubset: serviceSubset,
		Namespace:     namespace,
		Datacenter:    datacenter,
	}
	t.setID()
	return t
}

func (t *DiscoveryTarget) setID() {
	// NOTE: this format is similar to the SNI syntax for simplicity
	if t.ServiceSubset == "" {
		t.ID = fmt.Sprintf("%s.%s.%s", t.Service, t.Namespace, t.Datacenter)
	} else {
		t.ID = fmt.Sprintf("%s.%s.%s.%s", t.ServiceSubset, t.Service, t.Namespace, t.Datacenter)
	}
}

func (t *DiscoveryTarget) String() string {
	return t.ID
}

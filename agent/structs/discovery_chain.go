package structs

import (
	"bytes"
	"encoding"
	"fmt"
	"net/url"
	"strings"
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
	CustomizationHash string

	// Protocol is the overall protocol shared by everything in the chain.
	Protocol string

	// StartNode is the first key into the Nodes map that should be followed
	// when walking the discovery chain.
	StartNode string `json:",omitempty"`

	// Nodes contains all nodes available for traversal in the chain keyed by a
	// unique name.  You can walk this by starting with StartNode.
	//
	// NOTE: The names should be treated as opaque values and are only
	// guaranteed to be consistent within a single compilation.
	Nodes map[string]*DiscoveryGraphNode `json:",omitempty"`

	// Targets is a list of all targets and configuration related just to targets.
	Targets map[DiscoveryTarget]DiscoveryTargetConfig `json:",omitempty"`
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

	// TODO(rb): include CustomizationHash here?
	return node.Type == DiscoveryGraphNodeTypeResolver &&
		node.Resolver.Default &&
		node.Resolver.Target.Service == c.ServiceName
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

func (s *DiscoveryGraphNode) ServiceName() string {
	if s.Type == DiscoveryGraphNodeTypeResolver {
		return s.Resolver.Target.Service
	}
	return s.Name
}

func (s *DiscoveryGraphNode) MapKey() string {
	return fmt.Sprintf("%s:%s", s.Type, s.Name)
}

// compiled form of ServiceResolverConfigEntry
type DiscoveryResolver struct {
	Definition     *ServiceResolverConfigEntry `json:",omitempty"`
	Default        bool                        `json:",omitempty"`
	ConnectTimeout time.Duration               `json:",omitempty"`
	Target         DiscoveryTarget             `json:",omitempty"`
	Failover       *DiscoveryFailover          `json:",omitempty"`
}

type DiscoveryTargetConfig struct {
	MeshGateway MeshGatewayConfig     `json:",omitempty"`
	Subset      ServiceResolverSubset `json:",omitempty"`
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
	Definition *ServiceResolverFailover `json:",omitempty"`
	Targets    []DiscoveryTarget        `json:",omitempty"`
}

// DiscoveryTarget represents all of the inputs necessary to use a resolver
// config entry to execute a catalog query to generate a list of service
// instances during discovery.
//
// This is a value type so it can be used as a map key.
type DiscoveryTarget struct {
	Service       string `json:",omitempty"`
	ServiceSubset string `json:",omitempty"`
	Namespace     string `json:",omitempty"`
	Datacenter    string `json:",omitempty"`
}

func (t DiscoveryTarget) IsEmpty() bool {
	return t.Service == "" && t.ServiceSubset == "" && t.Namespace == "" && t.Datacenter == ""
}

// CopyAndModify will duplicate the target and selectively modify it given the
// requested inputs.
func (t DiscoveryTarget) CopyAndModify(
	service,
	serviceSubset,
	namespace,
	datacenter string,
) DiscoveryTarget {
	t2 := t // copy
	if service != "" && service != t2.Service {
		t2.Service = service
		// Reset the chosen subset if we reference a service other than our own.
		t2.ServiceSubset = ""
	}
	if serviceSubset != "" && serviceSubset != t2.ServiceSubset {
		t2.ServiceSubset = serviceSubset
	}
	if namespace != "" && namespace != t2.Namespace {
		t2.Namespace = namespace
	}
	if datacenter != "" && datacenter != t2.Datacenter {
		t2.Datacenter = datacenter
	}
	return t2
}

var _ encoding.TextMarshaler = DiscoveryTarget{}
var _ encoding.TextUnmarshaler = (*DiscoveryTarget)(nil)

// MarshalText implements encoding.TextMarshaler.
//
// This should also not include any colons for embedding that happens
// elsewhere.
//
// This should NOT return any errors.
func (t DiscoveryTarget) MarshalText() (text []byte, err error) {
	return []byte(t.Identifier()), nil
}

func (t DiscoveryTarget) Identifier() string {
	var buf bytes.Buffer
	buf.WriteString(url.QueryEscape(t.Service))
	buf.WriteRune(',')
	buf.WriteString(url.QueryEscape(t.ServiceSubset)) // TODO(rb): move this first so the scoping flows from small->large?
	buf.WriteRune(',')
	if t.Namespace != "default" {
		buf.WriteString(url.QueryEscape(t.Namespace))
	}
	buf.WriteRune(',')
	buf.WriteString(url.QueryEscape(t.Datacenter))
	return buf.String()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *DiscoveryTarget) UnmarshalText(text []byte) error {
	parts := bytes.Split(text, []byte(","))
	bad := false
	if len(parts) != 4 {
		return fmt.Errorf("invalid target: %q", string(text))
	}

	var err error
	t.Service, err = url.QueryUnescape(string(parts[0]))
	if err != nil {
		bad = true
	}
	t.ServiceSubset, err = url.QueryUnescape(string(parts[1]))
	if err != nil {
		bad = true
	}
	t.Namespace, err = url.QueryUnescape(string(parts[2]))
	if err != nil {
		bad = true
	}
	t.Datacenter, err = url.QueryUnescape(string(parts[3]))
	if err != nil {
		bad = true
	}

	if bad {
		return fmt.Errorf("invalid target: %q", string(text))
	}

	if t.Namespace == "" {
		t.Namespace = "default"
	}
	return nil
}

func (t DiscoveryTarget) String() string {
	var b strings.Builder

	if t.ServiceSubset != "" {
		b.WriteString(t.ServiceSubset)
	} else {
		b.WriteString("<default>")
	}
	b.WriteRune('.')

	b.WriteString(t.Service)
	b.WriteRune('.')

	if t.Namespace != "" {
		b.WriteString(t.Namespace)
	} else {
		b.WriteString("<default>")
	}
	b.WriteRune('.')

	b.WriteString(t.Datacenter)

	return b.String()
}

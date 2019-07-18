package structs

import (
	"bytes"
	"encoding"
	"fmt"
	"net/url"
	"sort"
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

	Protocol string // overall protocol shared by everything in the chain

	// Node is the top node in the chain.
	//
	// If this is a router or splitter then in envoy this renders as an http
	// route object.
	//
	// If this is a group resolver then in envoy this renders as a default
	// wildcard http route object.
	Node *DiscoveryGraphNode `json:",omitempty"`

	// GroupResolverNodes respresents all unique service instance groups that
	// need to be represented. For envoy these render as Clusters.
	//
	// Omitted from JSON because these already show up under the Node field.
	GroupResolverNodes map[DiscoveryTarget]*DiscoveryGraphNode `json:"-"`

	// TODO(rb): not sure if these two fields are actually necessary but I'll know when I get into xDS
	Resolvers map[string]*ServiceResolverConfigEntry `json:",omitempty"`
	Targets   []DiscoveryTarget                      `json:",omitempty"`
}

// IsDefault returns true if the compiled chain represents no routing, no
// splitting, and only the default resolution.  We have to be careful here to
// avoid returning "yep this is default" when the only resolver action being
// applied is redirection to another resolver that is default, so we double
// check the resolver matches the requested resolver.
func (c *CompiledDiscoveryChain) IsDefault() bool {
	if c.Node == nil {
		return true
	}
	return c.Node.Name == c.ServiceName &&
		c.Node.Type == DiscoveryGraphNodeTypeGroupResolver &&
		c.Node.GroupResolver.Default
}

// SubsetDefinitionForTarget is a convenience function to fetch the subset
// definition for the service subset defined by the provided target. If the
// subset is not defined an empty definition is returned.
func (c *CompiledDiscoveryChain) SubsetDefinitionForTarget(t DiscoveryTarget) ServiceResolverSubset {
	if t.ServiceSubset == "" {
		return ServiceResolverSubset{}
	}

	resolver, ok := c.Resolvers[t.Service]
	if !ok {
		return ServiceResolverSubset{}
	}

	return resolver.Subsets[t.ServiceSubset]
}

const (
	DiscoveryGraphNodeTypeRouter        = "router"
	DiscoveryGraphNodeTypeSplitter      = "splitter"
	DiscoveryGraphNodeTypeGroupResolver = "group-resolver"
)

// DiscoveryGraphNode is a single node of the compiled discovery chain.
type DiscoveryGraphNode struct {
	Type string
	Name string // default chain/service name at this spot

	// fields for Type==router
	Routes []*DiscoveryRoute `json:",omitempty"`

	// fields for Type==splitter
	Splits []*DiscoverySplit `json:",omitempty"`

	// fields for Type==group-resolver
	GroupResolver *DiscoveryGroupResolver `json:",omitempty"`
}

// compiled form of ServiceResolverConfigEntry but customized per non-failover target
type DiscoveryGroupResolver struct {
	Definition     *ServiceResolverConfigEntry `json:",omitempty"`
	Default        bool                        `json:",omitempty"`
	ConnectTimeout time.Duration               `json:",omitempty"`
	MeshGateway    MeshGatewayConfig           `json:",omitempty"`
	Target         DiscoveryTarget             `json:",omitempty"`
	Failover       *DiscoveryFailover          `json:",omitempty"`
}

// compiled form of ServiceRoute
type DiscoveryRoute struct {
	Definition      *ServiceRoute       `json:",omitempty"`
	DestinationNode *DiscoveryGraphNode `json:",omitempty"`
}

// compiled form of ServiceSplit
type DiscoverySplit struct {
	Weight float32             `json:",omitempty"`
	Node   *DiscoveryGraphNode `json:",omitempty"`
}

// compiled form of ServiceResolverFailover
// TODO(rb): figure out how to get mesh gateways in here
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
	var buf bytes.Buffer
	buf.WriteString(url.QueryEscape(t.Service))
	buf.WriteRune(',')
	buf.WriteString(url.QueryEscape(t.ServiceSubset))
	buf.WriteRune(',')
	if t.Namespace != "default" {
		buf.WriteString(url.QueryEscape(t.Namespace))
	}
	buf.WriteRune(',')
	buf.WriteString(url.QueryEscape(t.Datacenter))
	return buf.Bytes(), nil
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

type DiscoveryTargets []DiscoveryTarget

func (targets DiscoveryTargets) Sort() {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Service < targets[j].Service {
			return true
		} else if targets[i].Service > targets[j].Service {
			return false
		}

		if targets[i].ServiceSubset < targets[j].ServiceSubset {
			return true
		} else if targets[i].ServiceSubset > targets[j].ServiceSubset {
			return false
		}

		if targets[i].Namespace < targets[j].Namespace {
			return true
		} else if targets[i].Namespace > targets[j].Namespace {
			return false
		}

		return targets[i].Datacenter < targets[j].Datacenter
	})
}

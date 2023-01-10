package validate

import (
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
)

type Validate struct {
	snis map[string]struct{}

	listener  bool
	usesRDS   bool
	route     bool
	cluster   bool
	usesEDS   bool
	endpointsOnCluster int
	loadAssignment bool
	endpointsOnLoadAssignment int

	// TODO there is probably an edge case where we match different SNIs during each "makes sense" check.
	listenerDestinationMakesSense bool
	routeDestinationMakesSense bool
	clusterDestinationMakesSense bool
}

var _ builtinextensiontemplate.Plugin = (*Validate)(nil)

func (v *Validate) Errors() error {
	if !v.listener {
		return fmt.Errorf("no listener")
	}

	if !v.listenerDestinationMakesSense {
		return fmt.Errorf("listener destination doesn't make sense")
	}

	if v.usesRDS && !v.route {
		return fmt.Errorf("no route")
	}

	if v.usesRDS && !v.routeDestinationMakesSense {
		return fmt.Errorf("route destination doesn't make sense")
	}

	if !v.cluster {
		return fmt.Errorf("no cluster")
	}

	if !v.clusterDestinationMakesSense {
		return fmt.Errorf("cluster destination doesn't make sense")
	}

	if v.usesEDS && !v.loadAssignment {
		return fmt.Errorf("no cluster load assignment")
	}

	if !v.usesEDS && v.endpointsOnCluster == 0 {
		return fmt.Errorf("expected endpoints on cluster but we didn't get any")
	}

	if v.usesEDS && v.endpointsOnLoadAssignment == 0 {
		return fmt.Errorf("expected endpoints on load assignment but we didn't get any")
	}

	return nil
}

// MakeValidate is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeValidate(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin Validate

	plugin.snis = ext.Upstreams[ext.ServiceName].SNI

	return &plugin, resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p *Validate) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return true
}

func (p *Validate) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	p.route = true
	if !p.routeDestinationMakesSense {
		for sni := range p.snis {
			p.routeDestinationMakesSense = builtinextensiontemplate.RouteMatchesCluster(sni, route)
			break
		}
	}
	return route, false, nil
}

func (p *Validate) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	p.cluster = true
	if c.EdsClusterConfig != nil {
		p.usesEDS = true
		p.clusterDestinationMakesSense = true
	} else {
		la := c.LoadAssignment
		if la == nil {
			return c, false, nil
		}
		p.endpointsOnCluster = len(la.Endpoints) + len(la.NamedEndpoints)
		p.clusterDestinationMakesSense = true
	}
	return c, false, nil
}

func (p *Validate) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	// TODO If a single filter exists for a listener we say it exists.
	p.listener = true

	if config := envoy_resource_v3.GetHTTPConnectionManager(filter); config != nil {
		if config.GetRds() != nil {
			p.usesRDS = true
			p.listenerDestinationMakesSense = true
		}
	}

	if !p.listenerDestinationMakesSense {
		for sni := range p.snis {
			p.listenerDestinationMakesSense = builtinextensiontemplate.FilterDestinationMatch(sni, filter)
			break
		}
	}

	return filter, true, nil
}

func (p *Validate) PatchClusterLoadAssignment(la *envoy_endpoint_v3.ClusterLoadAssignment) (*envoy_endpoint_v3.ClusterLoadAssignment, bool, error) {
	p.loadAssignment = true
	p.endpointsOnLoadAssignment = len(la.Endpoints) + len(la.NamedEndpoints)
	return la, false, nil
}

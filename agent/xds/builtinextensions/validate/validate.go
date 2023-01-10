package validate

import (
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
)

type Validate struct {
	snis map[string]struct{}

	// listener specifies if the service's listener has been seen.
	listener                  bool

	// usesRDS determines if the listener's outgoing filter uses RDS.
	usesRDS                   bool

	// listener specifies if the service's route has been seen.
	route                     bool

	// expectedResources is a mapping from SNI to the expected resources 
	// for that SNI. It is populated based on the cluster names on routes 
	// (whether they are specified on listener filters or routes).
	expectedResources map[string]*expectedResource
}

type expectedResource struct {
	// cluster specifies if the cluster has been seen.
	cluster bool
	// cluster specifies if the load assignment has been seen.
	loadAssignment bool
	// usesEDS specifies if the cluster has EDS configured.
	usesEDS bool
	// The number of endpoints for the cluster or load assignment.
	endpoints int
}

var _ builtinextensiontemplate.Plugin = (*Validate)(nil)

// Errors returns the error based only on Validate's state.
func (v *Validate) Errors() error {
	var resultErr error
	if !v.listener {
		resultErr = multierror.Append(resultErr, fmt.Errorf("no listener"))
	}

	if v.usesRDS && !v.route {
		resultErr = multierror.Append(resultErr, fmt.Errorf("no route"))
	}

	for sni, expectedResource := range v.expectedResources {
		_, ok := v.snis[sni]
		if !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("unexpted route/listener destination cluster %s", sni))
			continue
		}
		
		if !expectedResource.cluster {
			resultErr = multierror.Append(resultErr, fmt.Errorf("no cluster for sni %s", sni))
		}

		if expectedResource.usesEDS && !expectedResource.loadAssignment {
			resultErr = multierror.Append(resultErr, fmt.Errorf("no cluster load assignment %s", sni))
		}

		if expectedResource.endpoints == 0 {
			resultErr = multierror.Append(resultErr, fmt.Errorf("zero endpoints sni %s", sni))
		}
	}

	return resultErr
}

// MakeValidate is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeValidate(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin Validate

	plugin.snis = ext.Upstreams[ext.ServiceName].SNI
	plugin.expectedResources = make(map[string]*expectedResource)

	return &plugin, resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p *Validate) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return true
}

func (p *Validate) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	p.route = true
	for sni := range p.snis {
		if _, ok := p.expectedResources[sni]; ok {
			continue
		}

			
		if builtinextensiontemplate.RouteMatchesCluster(sni, route) {
			p.expectedResources[sni] = &expectedResource{}
		}
	}
	return route, false, nil
}

func (p *Validate) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	v, ok := p.expectedResources[c.Name]
	if !ok {
		return c, false, nil
	}
	v.cluster = true

	if c.EdsClusterConfig != nil {
		v.usesEDS = true
	} else {
		la := c.LoadAssignment
		if la == nil {
			return c, false, nil
		}
		v.endpoints = len(la.Endpoints) + len(la.NamedEndpoints)
	}
	return c, false, nil
}

func (p *Validate) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	// TODO If a single filter exists for a listener we say it exists.
	p.listener = true

	if config := envoy_resource_v3.GetHTTPConnectionManager(filter); config != nil {
		if config.GetRds() != nil {
			p.usesRDS = true
			// No listener destination. Us RDS.
		}
	}

	for sni := range p.snis {
		if _, ok := p.expectedResources[sni]; ok {
			continue
		}

		if builtinextensiontemplate.FilterDestinationMatch(sni, filter) {
			p.expectedResources[sni] = &expectedResource{}
		}
	}

	return filter, true, nil
}

func (p *Validate) PatchClusterLoadAssignment(la *envoy_endpoint_v3.ClusterLoadAssignment) (*envoy_endpoint_v3.ClusterLoadAssignment, bool, error) {
	v, ok := p.expectedResources[la.ClusterName]
	if ok {
		v.loadAssignment = true
		v.endpoints = len(la.Endpoints) + len(la.NamedEndpoints)
	} else {
		return la, false, nil
	}
	return la, false, nil
}

package validate

import (
	"fmt"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_aggregate_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/clusters/aggregate/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
)

type Validate struct {
	envoyID string
	snis    map[string]struct{}

	// listener specifies if the service's listener has been seen.
	listener bool

	// usesRDS determines if the listener's outgoing filter uses RDS.
	usesRDS bool

	// listener specifies if the service's route has been seen.
	route bool

	// resources is a mapping from SNI to the expected resources
	// for that SNI. It is populated based on the cluster names on routes
	// (whether they are specified on listener filters or routes).
	resources map[string]*resource
}

type resource struct {
	// required determines if the resource is required for the given upstream.
	required bool

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

	for sni, resource := range v.resources {
		if !resource.required {
			continue
		}

		_, ok := v.snis[sni]
		if !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("unexpected route/listener destination cluster %s", sni))
			continue
		}

		if !resource.cluster {
			resultErr = multierror.Append(resultErr, fmt.Errorf("no cluster for sni %s", sni))
		}

		if resource.usesEDS && !resource.loadAssignment {
			resultErr = multierror.Append(resultErr, fmt.Errorf("no cluster load assignment %s", sni))
		}

		if resource.endpoints == 0 {
			resultErr = multierror.Append(resultErr, fmt.Errorf("zero endpoints sni %s", sni))
		}
	}

	return resultErr
}

// MakeValidate is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeValidate(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin Validate

	envoyID, _ := ext.EnvoyExtension.Arguments["envoyID"]
	mainEnvoyID, _ := envoyID.(string)
	if len(mainEnvoyID) == 0 {
		return nil, fmt.Errorf("envoyID is required")
	}
	plugin.envoyID = mainEnvoyID
	plugin.snis = ext.Upstreams[ext.ServiceName].SNI
	plugin.resources = make(map[string]*resource)

	return &plugin, resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p *Validate) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return true
}

func (p *Validate) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	if route.Name != p.envoyID {
		return route, false, nil
	}
	p.route = true
	for sni := range builtinextensiontemplate.RouteClusterNames(route) {
		if _, ok := p.resources[sni]; ok {
			continue
		}
		p.resources[sni] = &resource{required: true}
	}
	return route, false, nil
}

func (p *Validate) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	v, ok := p.resources[c.Name]
	if !ok {
		v = &resource{}
		p.resources[c.Name] = v
	}
	v.cluster = true

	cdt, ok := c.ClusterDiscoveryType.(*envoy_cluster_v3.Cluster_ClusterType)

	if ok {
		cct := cdt.ClusterType.TypedConfig
		if cct == nil {
			// TODO what to do here.
			return c, false, nil
		}
		aggregateCluster := &envoy_aggregate_cluster_v3.ClusterConfig{}
		err := anypb.UnmarshalTo(cct, aggregateCluster, proto.UnmarshalOptions{})
		if err != nil {
			// TODO what to do here.
			return c, false, nil
		}
		for _, clusterName := range aggregateCluster.Clusters {
			r, ok := p.resources[clusterName]
			if !ok {
				r = &resource{}
				p.resources[clusterName] = r
			}
			r.required = true
		}
		// If there is more than one aggregate cluster, defer to the endpoints theere.
		v.endpoints = len(aggregateCluster.Clusters)

		return c, false, nil
	}

	// TODO if aggregate cluster make the failover targets required.

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
			return filter, true, nil
		}
	}

	for sni := range builtinextensiontemplate.FilterClusterNames(filter) {
		if _, ok := p.resources[sni]; ok {
			continue
		}

		p.resources[sni] = &resource{required: true}
	}

	return filter, true, nil
}

func (p *Validate) PatchClusterLoadAssignment(la *envoy_endpoint_v3.ClusterLoadAssignment) (*envoy_endpoint_v3.ClusterLoadAssignment, bool, error) {
	v, ok := p.resources[la.ClusterName]
	if ok {
		v.loadAssignment = true
		v.endpoints = len(la.Endpoints) + len(la.NamedEndpoints)
	} else {
		return la, false, nil
	}
	return la, false, nil
}

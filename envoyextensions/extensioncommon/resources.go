// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package extensioncommon

import (
	"errors"
	"fmt"
	"strings"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
)

// MakeUpstreamTLSTransportSocket generates an Envoy transport socket for the given TLS context.
func MakeUpstreamTLSTransportSocket(tlsContext *envoy_tls_v3.UpstreamTlsContext) (*envoy_core_v3.TransportSocket, error) {
	if tlsContext == nil {
		return nil, nil
	}
	return MakeTransportSocket("tls", tlsContext)
}

// MakeTransportSocket generates an Envoy transport socket from the given proto message.
func MakeTransportSocket(name string, config proto.Message) (*envoy_core_v3.TransportSocket, error) {
	any, err := anypb.New(config)
	if err != nil {
		return nil, err
	}
	return &envoy_core_v3.TransportSocket{
		Name: name,
		ConfigType: &envoy_core_v3.TransportSocket_TypedConfig{
			TypedConfig: any,
		},
	}, nil
}

// MakeEnvoyHTTPFilter generates an Envoy HTTP filter from the given proto message.
func MakeEnvoyHTTPFilter(name string, cfg proto.Message) (*envoy_http_v3.HttpFilter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_http_v3.HttpFilter{
		Name:       name,
		ConfigType: &envoy_http_v3.HttpFilter_TypedConfig{TypedConfig: any},
	}, nil
}

// MakeFilter generates an Envoy listener filter from the given proto message.
func MakeFilter(name string, cfg proto.Message) (*envoy_listener_v3.Filter, error) {
	any, err := anypb.New(cfg)
	if err != nil {
		return nil, err
	}

	return &envoy_listener_v3.Filter{
		Name:       name,
		ConfigType: &envoy_listener_v3.Filter_TypedConfig{TypedConfig: any},
	}, nil
}

// TrafficDirection determines whether inbound or outbound Envoy resources will be patched.
type TrafficDirection string

const (
	TrafficDirectionInbound  TrafficDirection = "inbound"
	TrafficDirectionOutbound TrafficDirection = "outbound"
)

var TrafficDirections = StringSet{string(TrafficDirectionInbound): {}, string(TrafficDirectionOutbound): {}}

type StringSet map[string]struct{}

func (c *StringSet) CheckRequired(v, fieldName string) error {
	if _, ok := (*c)[v]; !ok {
		if v == "" {
			return fmt.Errorf("field %s is required", fieldName)
		}

		var keys []string
		for k := range *c {
			keys = append(keys, k)
		}

		return fmt.Errorf("invalid %s '%q'; supported values: %s",
			fieldName, v, strings.Join(keys, ", "))
	}
	return nil
}

func NormalizeEmptyToDefault(s string) string {
	if s == "" {
		return "default"
	}

	return s
}

func NormalizeServiceName(sn *api.CompoundServiceName) {
	sn.Namespace = NormalizeEmptyToDefault(sn.Namespace)
	sn.Partition = NormalizeEmptyToDefault(sn.Partition)
}

// Payload represents a single Envoy resource to be modified by extensions.
// It associates the RuntimeConfig of the local proxy, the TrafficDirection
// of the resource, and the CompoundServiceName and UpstreamData (if outbound)
// of a service the Envoy resource corresponds to.
type Payload[K proto.Message] struct {
	RuntimeConfig    *RuntimeConfig
	ServiceName      *api.CompoundServiceName
	Upstream         *UpstreamData
	TrafficDirection TrafficDirection
	Message          K
}

func (p Payload[K]) IsInbound() bool {
	return p.TrafficDirection == TrafficDirectionInbound
}

type ClusterPayload = Payload[*envoy_cluster_v3.Cluster]
type ClusterLoadAssignmentPayload = Payload[*envoy_endpoint_v3.ClusterLoadAssignment]
type ListenerPayload = Payload[*envoy_listener_v3.Listener]
type FilterPayload = Payload[*envoy_listener_v3.Filter]
type RoutePayload = Payload[*envoy_route_v3.RouteConfiguration]

func (cfg *RuntimeConfig) GetClusterPayload(c *envoy_cluster_v3.Cluster) ClusterPayload {
	d := TrafficDirectionOutbound
	var u *UpstreamData
	var sn *api.CompoundServiceName

	if IsLocalAppCluster(c) {
		d = TrafficDirectionInbound
	} else {
		u, sn = cfg.findUpstreamBySNI(c.Name)
	}

	return ClusterPayload{
		RuntimeConfig:    cfg,
		ServiceName:      sn,
		Upstream:         u,
		TrafficDirection: d,
		Message:          c,
	}
}

func (c *RuntimeConfig) GetListenerPayload(l *envoy_listener_v3.Listener) ListenerPayload {
	d := TrafficDirectionOutbound
	var u *UpstreamData
	var sn *api.CompoundServiceName

	if IsInboundPublicListener(l) {
		d = TrafficDirectionInbound
	} else {
		u, sn = c.findUpstreamByEnvoyID(GetListenerEnvoyID(l))
	}

	return ListenerPayload{
		RuntimeConfig:    c,
		ServiceName:      sn,
		Upstream:         u,
		TrafficDirection: d,
		Message:          l,
	}
}

func (c *RuntimeConfig) GetFilterPayload(f *envoy_listener_v3.Filter, l *envoy_listener_v3.Listener) FilterPayload {
	d := TrafficDirectionOutbound
	var u *UpstreamData
	var sn *api.CompoundServiceName

	if IsInboundPublicListener(l) {
		d = TrafficDirectionInbound
	} else {
		u, sn = c.findUpstreamByEnvoyID(GetListenerEnvoyID(l))
	}

	return FilterPayload{
		RuntimeConfig:    c,
		ServiceName:      sn,
		Upstream:         u,
		TrafficDirection: d,
		Message:          f,
	}
}

func (c *RuntimeConfig) GetRoutePayload(r *envoy_route_v3.RouteConfiguration) RoutePayload {
	d := TrafficDirectionOutbound
	var u *UpstreamData
	var sn *api.CompoundServiceName

	if IsRouteToLocalAppCluster(r) {
		d = TrafficDirectionInbound
	} else {
		u, sn = c.findUpstreamByEnvoyID(r.Name)
	}

	return RoutePayload{
		RuntimeConfig:    c,
		ServiceName:      sn,
		Upstream:         u,
		TrafficDirection: d,
		Message:          r,
	}
}

func (cfg *RuntimeConfig) GetClusterLoadAssignmentPayload(c *envoy_endpoint_v3.ClusterLoadAssignment) ClusterLoadAssignmentPayload {
	d := TrafficDirectionOutbound
	var u *UpstreamData
	var sn *api.CompoundServiceName

	if IsLocalAppClusterLoadAssignment(c) {
		d = TrafficDirectionInbound
	} else {
		u, sn = cfg.findUpstreamBySNI(c.ClusterName)

	}

	return ClusterLoadAssignmentPayload{
		RuntimeConfig:    cfg,
		ServiceName:      sn,
		Upstream:         u,
		TrafficDirection: d,
		Message:          c,
	}
}

func (c *RuntimeConfig) findUpstreamByEnvoyID(envoyID string) (*UpstreamData, *api.CompoundServiceName) {
	for sn, u := range c.Upstreams {
		if u.EnvoyID == envoyID {
			return u, &sn
		}
	}

	return nil, nil
}

func (c *RuntimeConfig) findUpstreamBySNI(sni string) (*UpstreamData, *api.CompoundServiceName) {
	for sn, u := range c.Upstreams {
		_, ok := u.SNIs[sni]
		if ok {
			return u, &sn
		}

		if strings.HasPrefix(sni, xdscommon.FailoverClusterNamePrefix) {
			parts := strings.Split(sni, "~")

			if len(parts) != 3 {
				continue
			}

			id := parts[2]
			_, ok := u.SNIs[id]
			if ok {
				return u, &sn
			}
		}
	}

	return nil, nil
}

// GetListenerEnvoyID returns the Envoy ID string parsed from the name of the given Listener. If none is found, it
// returns the empty string.
func GetListenerEnvoyID(l *envoy_listener_v3.Listener) string {
	if id, _, found := strings.Cut(l.Name, ":"); found {
		return id
	}
	return ""
}

// IsLocalAppCluster returns true if the given Cluster represents the local Cluster, which receives inbound traffic to
// the local proxy.
func IsLocalAppCluster(c *envoy_cluster_v3.Cluster) bool {
	return c.Name == xdscommon.LocalAppClusterName
}

// IsLocalAppClusterLoadAssignment returns true if the given ClusterLoadAssignment represents the local Cluster, which
// receives inbound traffic to the local proxy.
func IsLocalAppClusterLoadAssignment(a *envoy_endpoint_v3.ClusterLoadAssignment) bool {
	return a.ClusterName == xdscommon.LocalAppClusterName
}

// IsRouteToLocalAppCluster takes a RouteConfiguration and returns true if all routes within it target the local
// Cluster. Note that because we currently target RouteConfiguration in PatchRoute, we have to check multiple individual
// Route resources.
func IsRouteToLocalAppCluster(r *envoy_route_v3.RouteConfiguration) bool {
	clusterNames := RouteClusterNames(r)
	_, match := clusterNames[xdscommon.LocalAppClusterName]

	return match && len(clusterNames) == 1
}

// IsInboundPublicListener returns true if the given Listener represents the inbound public Listener for the local
// service.
func IsInboundPublicListener(l *envoy_listener_v3.Listener) bool {
	return GetListenerEnvoyID(l) == xdscommon.PublicListenerName
}

// IsOutboundTProxyListener returns true if the given Listener represents the outbound TProxy Listener for the local
// service.
func IsOutboundTProxyListener(l *envoy_listener_v3.Listener) bool {
	return GetListenerEnvoyID(l) == xdscommon.OutboundListenerName
}

func filterChainTProxyMatch(vip string, filterChain *envoy_listener_v3.FilterChain) bool {
	for _, prefixRange := range filterChain.FilterChainMatch.PrefixRanges {
		// Since we always set the address prefix as the full VIP (rather than a prefix), we can just check if they are
		// equal to find the matching filter chain.
		if vip == prefixRange.AddressPrefix {
			return true
		}
	}

	return false
}

func FilterClusterNames(filter *envoy_listener_v3.Filter) map[string]struct{} {
	clusterNames := make(map[string]struct{})
	if filter == nil {
		return clusterNames
	}

	if config := envoy_resource_v3.GetHTTPConnectionManager(filter); config != nil {
		// If it's using RDS, the cluster names will be in the route, rather than in the http filter's route config, so
		// we don't return any cluster names in this case. They can be gathered from the route.
		if config.GetRds() != nil {
			return clusterNames
		}

		cfg := config.GetRouteConfig()

		clusterNames = RouteClusterNames(cfg)
	}

	if config := GetTCPProxy(filter); config != nil {
		clusterNames[config.GetCluster()] = struct{}{}
	}

	return clusterNames
}

func RouteClusterNames(route *envoy_route_v3.RouteConfiguration) map[string]struct{} {
	if route == nil {
		return nil
	}

	clusterNames := make(map[string]struct{})

	for _, virtualHost := range route.VirtualHosts {
		for _, route := range virtualHost.Routes {
			r := route.GetRoute()
			if r == nil {
				continue
			}
			if c := r.GetCluster(); c != "" {
				clusterNames[r.GetCluster()] = struct{}{}
			}

			if wc := r.GetWeightedClusters(); wc != nil {
				for _, c := range wc.GetClusters() {
					if c.Name != "" {
						clusterNames[c.Name] = struct{}{}
					}
				}
			}
		}
	}
	return clusterNames
}

func GetTCPProxy(filter *envoy_listener_v3.Filter) *envoy_tcp_proxy_v3.TcpProxy {
	if typedConfig := filter.GetTypedConfig(); typedConfig != nil {
		config := &envoy_tcp_proxy_v3.TcpProxy{}
		if err := anypb.UnmarshalTo(typedConfig, config, proto.UnmarshalOptions{}); err == nil {
			return config
		}
	}

	return nil
}

func getSNI(chain *envoy_listener_v3.FilterChain) string {
	var sni string

	if chain == nil {
		return sni
	}

	if chain.FilterChainMatch == nil {
		return sni
	}

	if len(chain.FilterChainMatch.ServerNames) == 0 {
		return sni
	}

	return chain.FilterChainMatch.ServerNames[0]
}

// GetHTTPConnectionManager returns the Envoy HttpConnectionManager filter from the list of network filters.
// It also returns the index within the list of filters where the connection manager was found in case the
// caller needs to overwrite the original filter.
// It returns a non-nil error if the HttpConnectionManager is not found.
func GetHTTPConnectionManager(filters ...*envoy_listener_v3.Filter) (*envoy_http_v3.HttpConnectionManager, int, error) {
	for idx, filter := range filters {
		if filter.Name == "envoy.filters.network.http_connection_manager" {
			if httpConnMgr := envoy_resource_v3.GetHTTPConnectionManager(filter); httpConnMgr != nil {
				return httpConnMgr, idx, nil
			}
		}
	}
	return nil, 0, errors.New("failed to get HTTP connection manager")
}

// InsertLocation indicates where to insert an Envoy resource within a list of resources.
type InsertLocation string

const (
	// InsertFirst inserts the resource as the first entry in the list.
	InsertFirst InsertLocation = "First"
	// InsertLast inserts the resource as the last entry in the list.
	InsertLast InsertLocation = "Last"
	// InsertBeforeFirstMatch inserts the resource before the first resource with a matching name.
	InsertBeforeFirstMatch InsertLocation = "BeforeFirstMatch"
	// InsertAfterFirstMatch inserts the resource after the first resource with a matching name.
	InsertAfterFirstMatch InsertLocation = "AfterFirstMatch"
	// InsertBeforeLastMatch inserts the resource before the last resource with a matching name.
	InsertBeforeLastMatch InsertLocation = "BeforeLastMatch"
	// InsertAfterLastMatch inserts the resource after the last resource with a matching name.
	InsertAfterLastMatch InsertLocation = "AfterLastMatch"
)

// InsertOptions controls how and where to insert Envoy resources.
type InsertOptions struct {
	// Location defines where to insert the resource within the list.
	Location InsertLocation
	// FilterName indicates the name of the resource to insert relative to.
	FilterName string
}

// InsertHTTPFilter inserts the given HTTP filter into the HttpConnectionManager's filter chain in the location
// determined by the insert options. This list of filters must include the HttpConnectionManager network
// filter or the operation will fail.
//
// It returns the modified list of filters including the updated HttpConnectionManager.
// If a matching location is not found to insert the filter, a non-nil error is returned.
func InsertHTTPFilter(filters []*envoy_listener_v3.Filter, filter *envoy_http_v3.HttpFilter, opts InsertOptions) ([]*envoy_listener_v3.Filter, error) {
	httpConnMgr, idx, err := GetHTTPConnectionManager(filters...)
	if err != nil {
		return filters, err
	}

	namedFilters := make([]namedFilter, 0, len(httpConnMgr.HttpFilters)+1)
	for _, f := range httpConnMgr.HttpFilters {
		namedFilters = append(namedFilters, f)
	}
	insertIdx, err := locateInsertIndex(opts, namedFilters)
	if err != nil {
		return filters, fmt.Errorf("failed to insert %q filter: %w", filter.Name, err)
	}

	currIdx := 0
	newHttpFilters := make([]*envoy_http_v3.HttpFilter, len(httpConnMgr.HttpFilters)+1)
	for idx, httpFilter := range httpConnMgr.HttpFilters {
		if idx == insertIdx {
			newHttpFilters[currIdx] = filter
			currIdx++
		}
		newHttpFilters[currIdx] = httpFilter
		currIdx++
	}
	if currIdx == insertIdx {
		newHttpFilters[currIdx] = filter
	}

	httpConnMgr.HttpFilters = newHttpFilters
	newHttpConMan, err := MakeFilter("envoy.filters.network.http_connection_manager", httpConnMgr)
	if err != nil {
		return filters, errors.New("failed to insert new HTTP connection manager filter")
	}
	filtersCopy := make([]*envoy_listener_v3.Filter, len(filters))
	copy(filtersCopy, filters)
	filtersCopy[idx] = newHttpConMan

	return filtersCopy, nil
}

// InsertNetworkFilter inserts the given network filter into the filter chain in the location
// determined by the insert options.
//
// It returns the modified list of filters including the new filter.
// If a matching location is not found to insert the filter, a non-nil error is returned.
func InsertNetworkFilter(filters []*envoy_listener_v3.Filter, filter *envoy_listener_v3.Filter, opts InsertOptions) ([]*envoy_listener_v3.Filter, error) {
	namedFilters := make([]namedFilter, 0, len(filters)+1)
	for _, f := range filters {
		namedFilters = append(namedFilters, f)
	}
	insertIdx, err := locateInsertIndex(opts, namedFilters)
	if err != nil {
		return filters, fmt.Errorf("failed to insert %q filter: %w", filter.Name, err)
	}

	currIdx := 0
	newFilters := make([]*envoy_listener_v3.Filter, len(filters)+1)
	for idx, f := range filters {
		if idx == insertIdx {
			newFilters[currIdx] = filter
			currIdx++
		}
		newFilters[currIdx] = f
		currIdx++
	}
	if currIdx == insertIdx {
		newFilters[currIdx] = filter
	}

	return newFilters, nil
}

// namedFilter is a convenience interface for locating Envoy filters based on name.
type namedFilter interface {
	GetName() string
}

// locateInsertIndex returns the index where a filter should be inserted based on the given
// insert options.
func locateInsertIndex(opts InsertOptions, filters []namedFilter) (int, error) {
	idx := 0
	if opts.Location == InsertFirst {
		return idx, nil
	}
	if opts.Location == InsertLast {
		return len(filters), nil
	}

	matched := false
	for currIdx, filter := range filters {
		if filter.GetName() == opts.FilterName {
			matched = true
			switch opts.Location {
			case InsertBeforeFirstMatch:
				return currIdx, nil
			case InsertAfterFirstMatch:
				return currIdx + 1, nil
			case InsertBeforeLastMatch:
				idx = currIdx
			case InsertAfterLastMatch:
				idx = currIdx + 1
			}
		}
	}
	if matched {
		return idx, nil
	}
	return idx, fmt.Errorf("failed to find insert location %q for %q", opts.Location, opts.FilterName)
}

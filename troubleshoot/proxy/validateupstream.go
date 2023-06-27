// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package troubleshoot

import (
	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/troubleshoot/validate"
)

const (
	listenersType string = "type.googleapis.com/envoy.admin.v3.ListenersConfigDump"
	clustersType  string = "type.googleapis.com/envoy.admin.v3.ClustersConfigDump"
	routesType    string = "type.googleapis.com/envoy.admin.v3.RoutesConfigDump"
	endpointsType string = "type.googleapis.com/envoy.admin.v3.EndpointsConfigDump"
)

func ParseConfigDump(rawConfig []byte) (*xdscommon.IndexedResources, error) {
	config := &envoy_admin_v3.ConfigDump{}

	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := unmarshal.Unmarshal(rawConfig, config)
	if err != nil {
		return nil, err
	}

	return ProxyConfigDumpToIndexedResources(config)
}

func ParseClusters(rawClusters []byte) (*envoy_admin_v3.Clusters, error) {
	clusters := &envoy_admin_v3.Clusters{}
	unmarshal := &protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err := unmarshal.Unmarshal(rawClusters, clusters)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

// Validate validates the Envoy resources (indexedResources) for a given upstream service, peer, and vip. The peer
// should be "" for an upstream not on a remote peer. The vip is required for a transparent proxy upstream.
func Validate(indexedResources *xdscommon.IndexedResources, envoyID string, vip string, validateEndpoints bool, clusters *envoy_admin_v3.Clusters) validate.Messages {
	// Get all SNIs from the clusters in the configuration. Not all SNIs will need to be validated, but this ensures we
	// capture SNIs which aren't directly identical to the upstream service name, but are still used for that upstream
	// service. For example, in the case of having a splitter/redirect or another L7 config entry, the upstream service
	// name could be "db" but due to a redirect SNI would be something like
	// "redis.default.dc1.internal.<trustdomain>.consul". The envoyID will be used to limit which SNIs we actually
	// validate.
	snis := map[string]struct{}{}
	for s := range indexedResources.Index[xdscommon.ClusterType] {
		snis[s] = struct{}{}
	}

	// For this extension runtime configuration, we are only validating one upstream service, so the map key doesn't
	// need the full service name.
	emptyServiceKey := api.CompoundServiceName{}

	// Build an ExtensionConfiguration for Validate plugin.
	extConfig := extensioncommon.RuntimeConfig{
		EnvoyExtension: api.EnvoyExtension{
			Name: "builtin/proxy/validate",
			Arguments: map[string]interface{}{
				"envoyID": envoyID,
			},
		},
		ServiceName:           emptyServiceKey,
		IsSourcedFromUpstream: true,
		Upstreams: map[api.CompoundServiceName]*extensioncommon.UpstreamData{
			emptyServiceKey: {
				VIP: vip,
				// Even though snis are under the upstream service name we're validating, it actually contains all
				// the cluster SNIs configured on this proxy, not just the upstream being validated. This means the
				// PatchCluster function in the Validate plugin will be run on all clusters, but errors will only
				// surface for clusters related to the upstream being validated.
				SNIs:    snis,
				EnvoyID: envoyID,
			},
		},
		Kind: api.ServiceKindConnectProxy,
	}
	ext, err := validate.MakeValidate(extConfig)
	if err != nil {
		return []validate.Message{{Message: err.Error()}}
	}
	extender := extensioncommon.UpstreamEnvoyExtender{
		Extension: ext,
	}
	err = extender.Validate(&extConfig)
	if err != nil {
		return []validate.Message{{Message: err.Error()}}
	}

	_, err = extender.Extend(indexedResources, &extConfig)
	if err != nil {
		return []validate.Message{{Message: err.Error()}}
	}

	v, ok := extender.Extension.(*validate.Validate)
	if !ok {
		panic("validate plugin was not correctly created")
	}

	return v.GetMessages(validateEndpoints, validate.DoEndpointValidation, clusters)
}

func ProxyConfigDumpToIndexedResources(config *envoy_admin_v3.ConfigDump) (*xdscommon.IndexedResources, error) {
	indexedResources := xdscommon.EmptyIndexedResources()
	unmarshal := &proto.UnmarshalOptions{
		DiscardUnknown: true,
	}

	for _, cfg := range config.Configs {
		switch cfg.TypeUrl {
		case listenersType:
			lcd := &envoy_admin_v3.ListenersConfigDump{}

			err := unmarshal.Unmarshal(cfg.GetValue(), lcd)
			if err != nil {
				return indexedResources, err
			}

			for _, listener := range lcd.GetDynamicListeners() {
				// TODO We should care about these:
				// listener.GetErrorState()
				// listener.GetDrainingState()
				// listener.GetWarmingState()

				r := indexedResources.Index[xdscommon.ListenerType]
				if r == nil {
					r = make(map[string]proto.Message)
				}
				as := listener.GetActiveState()
				if as == nil {
					continue
				}

				l := &envoy_listener_v3.Listener{}
				unmarshal.Unmarshal(as.Listener.GetValue(), l)
				if err != nil {
					return indexedResources, err
				}

				r[listener.Name] = l
				indexedResources.Index[xdscommon.ListenerType] = r
			}
		case clustersType:
			ccd := &envoy_admin_v3.ClustersConfigDump{}

			err := unmarshal.Unmarshal(cfg.GetValue(), ccd)
			if err != nil {
				return indexedResources, err
			}

			// TODO we should care about ccd.GetDynamicWarmingClusters()
			for _, cluster := range ccd.GetDynamicActiveClusters() {
				r := indexedResources.Index[xdscommon.ClusterType]
				if r == nil {
					r = make(map[string]proto.Message)
				}

				c := &envoy_cluster_v3.Cluster{}
				unmarshal.Unmarshal(cluster.GetCluster().Value, c)
				if err != nil {
					return indexedResources, err
				}

				r[c.Name] = c
				indexedResources.Index[xdscommon.ClusterType] = r
			}
		case routesType:
			rcd := &envoy_admin_v3.RoutesConfigDump{}

			err := unmarshal.Unmarshal(cfg.GetValue(), rcd)
			if err != nil {
				return indexedResources, err
			}

			for _, route := range rcd.GetDynamicRouteConfigs() {
				r := indexedResources.Index[xdscommon.RouteType]
				if r == nil {
					r = make(map[string]proto.Message)
				}

				rc := &envoy_route_v3.RouteConfiguration{}
				unmarshal.Unmarshal(route.GetRouteConfig().Value, rc)
				if err != nil {
					return indexedResources, err
				}

				r[rc.Name] = rc
				indexedResources.Index[xdscommon.RouteType] = r
			}
		case endpointsType:
			ecd := &envoy_admin_v3.EndpointsConfigDump{}

			err := unmarshal.Unmarshal(cfg.GetValue(), ecd)
			if err != nil {
				return indexedResources, err
			}

			for _, route := range ecd.GetDynamicEndpointConfigs() {
				r := indexedResources.Index[xdscommon.EndpointType]
				if r == nil {
					r = make(map[string]proto.Message)
				}

				rc := &envoy_endpoint_v3.ClusterLoadAssignment{}
				err := unmarshal.Unmarshal(route.EndpointConfig.GetValue(), rc)
				if err != nil {
					return indexedResources, err
				}

				r[rc.ClusterName] = rc
				indexedResources.Index[xdscommon.EndpointType] = r
			}
		}
	}

	return indexedResources, nil
}

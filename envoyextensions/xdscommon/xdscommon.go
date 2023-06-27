// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xdscommon

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/go-hclog"
)

const (
	// PublicListenerName is the name we give the public listener in Envoy config.
	PublicListenerName = "public_listener"

	// OutboundListenerName is the name we give the outbound Envoy listener when transparent proxy mode is enabled.
	OutboundListenerName = "outbound_listener"

	// LocalAppClusterName is the name we give the local application "cluster" in
	// Envoy config. Note that all cluster names may collide with service names
	// since we want cluster names and service names to match to enable nice
	// metrics correlation without massaging prefixes on cluster names.
	//
	// We should probably make this more unlikely to collide however changing it
	// potentially breaks upgrade compatibility without restarting all Envoy's as
	// it will no longer match their existing cluster name. Changing this will
	// affect metrics output so could break dashboards (for local app traffic).
	//
	// We should probably just make it configurable if anyone actually has
	// services named "local_app" in the future.
	LocalAppClusterName = "local_app"

	// Resource types in xDS v3. These are copied from
	// envoyproxy/go-control-plane/pkg/resource/v3/resource.go since we don't need any of
	// the rest of that package.
	apiTypePrefix = "type.googleapis.com/"

	// EndpointType is the TypeURL for Endpoint discovery responses.
	EndpointType = apiTypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"

	// ClusterType is the TypeURL for Cluster discovery responses.
	ClusterType = apiTypePrefix + "envoy.config.cluster.v3.Cluster"

	// RouteType is the TypeURL for Route discovery responses.
	RouteType = apiTypePrefix + "envoy.config.route.v3.RouteConfiguration"

	// ListenerType is the TypeURL for Listener discovery responses.
	ListenerType = apiTypePrefix + "envoy.config.listener.v3.Listener"

	// SecretType is the TypeURL for Secret discovery responses.
	SecretType = apiTypePrefix + "envoy.extensions.transport_sockets.tls.v3.Secret"

	FailoverClusterNamePrefix = "failover-target~"
)

type IndexedResources struct {
	// Index is a map of typeURL => resourceName => resource
	Index map[string]map[string]proto.Message

	// ChildIndex is a map of typeURL => parentResourceName => list of
	// childResourceNames. This only applies if the child and parent do not
	// share a name.
	ChildIndex map[string]map[string][]string
}

func IndexResources(logger hclog.Logger, resources map[string][]proto.Message) *IndexedResources {
	data := EmptyIndexedResources()

	for typeURL, typeRes := range resources {
		for _, res := range typeRes {
			name := GetResourceName(res)
			if name == "" {
				logger.Warn("skipping unexpected xDS type found in delta snapshot", "typeURL", typeURL)
			} else {
				data.Index[typeURL][name] = res
			}
		}
	}

	return data
}

func GetResourceName(res proto.Message) string {
	// NOTE: this only covers types that we currently care about for LDS/RDS/CDS/EDS
	switch x := res.(type) {
	case *envoy_listener_v3.Listener: // LDS
		return x.Name
	case *envoy_route_v3.RouteConfiguration: // RDS
		return x.Name
	case *envoy_cluster_v3.Cluster: // CDS
		return x.Name
	case *envoy_endpoint_v3.ClusterLoadAssignment: // EDS
		return x.ClusterName
	default:
		return ""
	}
}

func EmptyIndexedResources() *IndexedResources {
	return &IndexedResources{
		Index: map[string]map[string]proto.Message{
			ListenerType: make(map[string]proto.Message),
			RouteType:    make(map[string]proto.Message),
			ClusterType:  make(map[string]proto.Message),
			EndpointType: make(map[string]proto.Message),
		},
		ChildIndex: map[string]map[string][]string{
			ListenerType: make(map[string][]string),
			ClusterType:  make(map[string][]string),
		},
	}
}

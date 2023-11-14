// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
)

// ResourceGenerator is associated with a single gRPC stream and creates xDS
// resources for a single client.
type ResourceGenerator struct {
	Logger        hclog.Logger
	ProxyFeatures xdscommon.SupportedProxyFeatures
}

// NewResourceGenerator will create a new ResourceGenerator.
func NewResourceGenerator(
	logger hclog.Logger,
) *ResourceGenerator {
	return &ResourceGenerator{
		Logger: logger,
	}
}

// ProxyResources is the main state used to convert proxyState resources to Envoy resources.
type ProxyResources struct {
	// proxyState is the final proxyState computed by Consul controllers.
	proxyState *proxytracker.ProxyState
	// envoyResources is a map of each resource type (listener, endpoint, route, cluster, etc.)
	// with a corresponding map of k/v pairs of resource name to envoy proto message.
	// map[string]map[string]proto.Message is used over map[string][]proto.Message because
	// AllResourcesFromIR() will create envoy resource by walking the object graph from listener
	// to endpoint.  In the process, the same resource might be referenced more than once,
	// so the map is used to prevent duplicate resources being created and also will use
	// an O(1) lookup to see if it exists (it actually will set the map key rather than
	// checks everywhere) where as each lookup would be O(n) with a []proto structure.
	envoyResources map[string]map[string]proto.Message
}

func (g *ResourceGenerator) AllResourcesFromIR(proxyState *proxytracker.ProxyState) (map[string][]proto.Message, error) {
	pr := &ProxyResources{
		proxyState:     proxyState,
		envoyResources: make(map[string]map[string]proto.Message),
	}
	pr.envoyResources[xdscommon.ListenerType] = make(map[string]proto.Message)
	pr.envoyResources[xdscommon.RouteType] = make(map[string]proto.Message)
	pr.envoyResources[xdscommon.ClusterType] = make(map[string]proto.Message)
	pr.envoyResources[xdscommon.EndpointType] = make(map[string]proto.Message)

	err := pr.makeEnvoyResourceGraphsStartingFromListeners()
	if err != nil {
		return nil, fmt.Errorf("failed to generate xDS resources for ProxyState: %v", err)
	}

	// Now account for Clusters that did not have a destination.
	for name := range proxyState.Clusters {
		if _, ok := pr.envoyResources[xdscommon.ClusterType][name]; !ok {
			pr.addEnvoyClustersAndEndpointsToEnvoyResources(name)
		}
	}

	envoyResources := convertResourceMapsToResourceArrays(pr.envoyResources)
	return envoyResources, nil
}

// convertResourceMapsToResourceArrays will convert map[string]map[string]proto.Message, which is used to
// prevent duplicate resource being created, to map[string][]proto.Message which is used by Delta server.
func convertResourceMapsToResourceArrays(resourceMap map[string]map[string]proto.Message) map[string][]proto.Message {
	resources := make(map[string][]proto.Message)
	resources[xdscommon.ListenerType] = make([]proto.Message, 0)
	resources[xdscommon.RouteType] = make([]proto.Message, 0)
	resources[xdscommon.ClusterType] = make([]proto.Message, 0)
	resources[xdscommon.EndpointType] = make([]proto.Message, 0)

	// This conversion incurs processing cost which is done once in the generating envoy resources.
	// This tradeoff is preferable to doing array scan every time an envoy resource needs to be
	// to pr.envoyResource to see if it already exists.
	for resourceTypeName, resourceMap := range resourceMap {
		for _, resource := range resourceMap {
			resources[resourceTypeName] = append(resources[resourceTypeName], resource)
		}
	}
	return resources
}

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

func NewResourceGenerator(
	logger hclog.Logger,
) *ResourceGenerator {
	return &ResourceGenerator{
		Logger: logger,
	}
}

type ProxyResources struct {
	proxyState     *proxytracker.ProxyState
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

	err := pr.generateXDSResources()
	if err != nil {
		return nil, fmt.Errorf("failed to generate xDS resources for ProxyState: %v", err)
	}
	envoyResources := make(map[string][]proto.Message)
	envoyResources[xdscommon.ListenerType] = make([]proto.Message, 0)
	envoyResources[xdscommon.RouteType] = make([]proto.Message, 0)
	envoyResources[xdscommon.ClusterType] = make([]proto.Message, 0)
	envoyResources[xdscommon.EndpointType] = make([]proto.Message, 0)
	for resourceTypeName, resourceMap := range pr.envoyResources {
		for _, resource := range resourceMap {
			envoyResources[resourceTypeName] = append(envoyResources[resourceTypeName], resource)
		}
	}
	return envoyResources, nil
}

func (pr *ProxyResources) generateXDSResources() error {
	err := pr.makeXDSListeners()
	if err != nil {
		return err
	}

	return nil
}

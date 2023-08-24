// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"
	"github.com/hashicorp/consul/internal/mesh"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
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
	proxyState     *mesh.ProxyState
	envoyResources map[string][]proto.Message
}

func (g *ResourceGenerator) AllResourcesFromIR(proxyState *mesh.ProxyState) (map[string][]proto.Message, error) {
	pr := &ProxyResources{
		proxyState:     proxyState,
		envoyResources: make(map[string][]proto.Message),
	}
	err := pr.generateXDSResources()
	if err != nil {
		return nil, fmt.Errorf("failed to generate xDS resources for ProxyState: %v", err)
	}
	return pr.envoyResources, nil
}

func (pr *ProxyResources) generateXDSResources() error {
	listeners, err := pr.makeXDSListeners()
	if err != nil {
		return err
	}

	pr.envoyResources[xdscommon.ListenerType] = listeners

	clusters, err := pr.makeXDSClusters()
	if err != nil {
		return err
	}
	pr.envoyResources[xdscommon.ClusterType] = clusters

	endpoints, err := pr.makeXDSEndpoints()
	if err != nil {
		return err
	}
	pr.envoyResources[xdscommon.EndpointType] = endpoints

	routes, err := pr.makeXDSRoutes()
	if err != nil {
		return err
	}
	pr.envoyResources[xdscommon.RouteType] = routes

	return nil
}

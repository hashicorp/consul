// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
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
	proxyState     *pbmesh.ProxyState
	envoyResources map[string][]proto.Message
}

func (g *ResourceGenerator) AllResourcesFromIR(proxyState *pbmesh.ProxyState) (map[string][]proto.Message, error) {
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
	listeners := make([]proto.Message, 0)
	routes := make([]proto.Message, 0)

	for _, l := range pr.proxyState.Listeners {
		protoListener, err := pr.makeListener(l)
		// TODO: aggregate errors for listeners and still return any properly formed listeners.
		if err != nil {
			return err
		}
		listeners = append(listeners, protoListener)
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

	pr.envoyResources[xdscommon.RouteType] = routes

	return nil
}

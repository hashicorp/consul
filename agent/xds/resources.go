package xds

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/proxycfg"
)

// ResourceGenerator is associated with a single gRPC stream and creates xDS
// resources for a single client.
type ResourceGenerator struct {
	Logger         hclog.Logger
	CheckFetcher   HTTPCheckFetcher
	CfgFetcher     ConfigFetcher
	IncrementalXDS bool

	ProxyFeatures supportedProxyFeatures
}

func newResourceGenerator(
	logger hclog.Logger,
	checkFetcher HTTPCheckFetcher,
	cfgFetcher ConfigFetcher,
	incrementalXDS bool,
) *ResourceGenerator {
	return &ResourceGenerator{
		Logger:         logger,
		CheckFetcher:   checkFetcher,
		CfgFetcher:     cfgFetcher,
		IncrementalXDS: incrementalXDS,
	}
}

func (g *ResourceGenerator) allResourcesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) (map[string][]proto.Message, error) {
	all := make(map[string][]proto.Message)
	for _, typeUrl := range []string{ListenerType, RouteType, ClusterType, EndpointType} {
		res, err := g.resourcesFromSnapshot(typeUrl, cfgSnap)
		if err != nil {
			return nil, fmt.Errorf("failed to generate xDS resources for %q: %v", typeUrl, err)
		}
		all[typeUrl] = res
	}
	return all, nil
}

func (g *ResourceGenerator) resourcesFromSnapshot(typeUrl string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	switch typeUrl {
	case ListenerType:
		return g.listenersFromSnapshot(cfgSnap)
	case RouteType:
		return g.routesFromSnapshot(cfgSnap)
	case ClusterType:
		return g.clustersFromSnapshot(cfgSnap)
	case EndpointType:
		return g.endpointsFromSnapshot(cfgSnap)
	default:
		return nil, fmt.Errorf("unknown typeUrl: %s", typeUrl)
	}
}

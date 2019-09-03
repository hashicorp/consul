package xds

import (
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/consul/agent/structs"
)

func firstHealthyTarget(
	targets map[string]*structs.DiscoveryTarget,
	targetHealth map[string]structs.CheckServiceNodes,
	primaryTarget string,
	secondaryTargets []string,
) string {
	series := make([]string, 0, len(secondaryTargets)+1)
	series = append(series, primaryTarget)
	series = append(series, secondaryTargets...)

	for _, targetID := range series {
		target, ok := targets[targetID]
		if !ok {
			continue
		}

		endpoints, ok := targetHealth[targetID]
		if !ok {
			continue
		}
		for _, ep := range endpoints {
			healthStatus, _ := calculateEndpointHealthAndWeight(ep, target.Subset.OnlyPassing)
			if healthStatus == envoycore.HealthStatus_HEALTHY {
				return targetID
			}
		}
	}

	return primaryTarget // if everything is broken just use the primary for now
}

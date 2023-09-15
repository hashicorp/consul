// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

func (b *Builder) backendTargetToClusterName(
	backendTarget string,
	targets map[string]*pbmesh.BackendTargetDetails,
	defaultDC func(string) string,
) string {
	if backendTarget == types.NullRouteBackend {
		return NullRouteClusterName
	}

	details, ok := targets[backendTarget]
	if !ok {
		panic("dangling reference")
	}

	backendRef := details.BackendRef

	dc := defaultDC(backendRef.Datacenter)

	sni := DestinationSNI(
		details.BackendRef.Ref,
		dc,
		b.trustDomain,
	)

	return fmt.Sprintf("%s.%s", details.BackendRef.Port, sni)
}

func (b *Builder) makeHTTPRouteDestination(
	computedBackendRefs []*pbmesh.ComputedHTTPBackendRef,
	destConfig *pbproxystate.DestinationConfiguration,
	targets map[string]*pbmesh.BackendTargetDetails,
	defaultDC func(string) string,
) *pbproxystate.RouteDestination {
	switch len(computedBackendRefs) {
	case 0:
		panic("not possible to have a route rule with no backend refs")
	case 1:
		return b.makeRouteDestinationForDirect(computedBackendRefs[0].BackendTarget, destConfig, targets, defaultDC)
	default:
		clusters := make([]*pbproxystate.L7WeightedDestinationCluster, 0, len(computedBackendRefs))
		for _, computedBackendRef := range computedBackendRefs {
			clusterName := b.backendTargetToClusterName(computedBackendRef.BackendTarget, targets, defaultDC)

			clusters = append(clusters, &pbproxystate.L7WeightedDestinationCluster{
				Name: clusterName,
				Weight: &wrapperspb.UInt32Value{
					Value: computedBackendRef.Weight,
				},
			})
		}
		return b.makeRouteDestinationForSplit(clusters, destConfig)
	}
}

func (b *Builder) makeGRPCRouteDestination(
	computedBackendRefs []*pbmesh.ComputedGRPCBackendRef,
	destConfig *pbproxystate.DestinationConfiguration,
	targets map[string]*pbmesh.BackendTargetDetails,
	defaultDC func(string) string,
) *pbproxystate.RouteDestination {
	switch len(computedBackendRefs) {
	case 0:
		panic("not possible to have a route rule with no backend refs")
	case 1:
		return b.makeRouteDestinationForDirect(computedBackendRefs[0].BackendTarget, destConfig, targets, defaultDC)
	default:
		clusters := make([]*pbproxystate.L7WeightedDestinationCluster, 0, len(computedBackendRefs))
		for _, computedBackendRef := range computedBackendRefs {
			clusterName := b.backendTargetToClusterName(computedBackendRef.BackendTarget, targets, defaultDC)

			clusters = append(clusters, &pbproxystate.L7WeightedDestinationCluster{
				Name: clusterName,
				Weight: &wrapperspb.UInt32Value{
					Value: computedBackendRef.Weight,
				},
			})
		}
		return b.makeRouteDestinationForSplit(clusters, destConfig)
	}
}

func (b *Builder) makeRouteDestinationForDirect(
	backendTarget string,
	destConfig *pbproxystate.DestinationConfiguration,
	targets map[string]*pbmesh.BackendTargetDetails,
	defaultDC func(string) string,
) *pbproxystate.RouteDestination {
	clusterName := b.backendTargetToClusterName(backendTarget, targets, defaultDC)

	return &pbproxystate.RouteDestination{
		Destination: &pbproxystate.RouteDestination_Cluster{
			Cluster: &pbproxystate.DestinationCluster{
				Name: clusterName,
			},
		},
		DestinationConfiguration: destConfig,
	}
}

func (b *Builder) makeRouteDestinationForSplit(
	clusters []*pbproxystate.L7WeightedDestinationCluster,
	destConfig *pbproxystate.DestinationConfiguration,
) *pbproxystate.RouteDestination {
	return &pbproxystate.RouteDestination{
		Destination: &pbproxystate.RouteDestination_WeightedClusters{
			WeightedClusters: &pbproxystate.L7WeightedClusterGroup{
				Clusters: clusters,
			},
		},
		DestinationConfiguration: destConfig,
	}
}

func (b *Builder) makeDestinationConfiguration(
	timeouts *pbmesh.HTTPRouteTimeouts,
	retries *pbmesh.HTTPRouteRetries,
) *pbproxystate.DestinationConfiguration {
	// TODO: prefix rewrite,  lb config

	cfg := &pbproxystate.DestinationConfiguration{
		TimeoutConfig: translateTimeouts(timeouts),
		RetryPolicy:   translateRetries(retries),
	}
	if cfg.TimeoutConfig == nil && cfg.RetryPolicy == nil {
		return nil
	}

	return cfg
}

func makeGRPCRouteMatch(match *pbmesh.GRPCRouteMatch) *pbproxystate.RouteMatch {
	panic("TODO")
}

func makeHTTPRouteMatch(match *pbmesh.HTTPRouteMatch) *pbproxystate.RouteMatch {
	em := &pbproxystate.RouteMatch{}

	if match.Path != nil {
		// enumcover:pbmesh.PathMatchType
		switch match.Path.Type {
		case pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT:
			em.PathMatch = &pbproxystate.PathMatch{
				PathMatch: &pbproxystate.PathMatch_Exact{
					Exact: match.Path.Value,
				},
			}
		case pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX:
			em.PathMatch = &pbproxystate.PathMatch{
				PathMatch: &pbproxystate.PathMatch_Prefix{
					Prefix: match.Path.Value,
				},
			}
		case pbmesh.PathMatchType_PATH_MATCH_TYPE_REGEX:
			em.PathMatch = &pbproxystate.PathMatch{
				PathMatch: &pbproxystate.PathMatch_Regex{
					Regex: match.Path.Value,
				},
			}
		case pbmesh.PathMatchType_PATH_MATCH_TYPE_UNSPECIFIED:
			fallthrough // to default
		default:
			panic(fmt.Sprintf("unknown path match type: %v", match.Path.Type))
		}
	} else {
		em.PathMatch = &pbproxystate.PathMatch{
			PathMatch: &pbproxystate.PathMatch_Prefix{
				Prefix: "/",
			},
		}
	}

	if len(match.Headers) > 0 {
		em.HeaderMatches = make([]*pbproxystate.HeaderMatch, 0, len(match.Headers))
		for _, hdr := range match.Headers {
			eh := &pbproxystate.HeaderMatch{
				Name: hdr.Name,
			}

			// enumcover:pbmesh.HeaderMatchType
			switch hdr.Type {
			case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT:
				eh.Match = &pbproxystate.HeaderMatch_Exact{
					Exact: hdr.Value,
				}
			case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_REGEX:
				eh.Match = &pbproxystate.HeaderMatch_Regex{
					Regex: hdr.Value,
				}
			case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_PREFIX:
				eh.Match = &pbproxystate.HeaderMatch_Prefix{
					Prefix: hdr.Value,
				}
			case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_SUFFIX:
				eh.Match = &pbproxystate.HeaderMatch_Suffix{
					Suffix: hdr.Value,
				}
			case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_PRESENT:
				eh.Match = &pbproxystate.HeaderMatch_Present{
					Present: true,
				}
			case pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_UNSPECIFIED:
				fallthrough // to default
			default:
				panic(fmt.Sprintf("unknown header match type: %v", hdr.Type))
			}

			if hdr.Invert {
				eh.InvertMatch = true
			}

			em.HeaderMatches = append(em.HeaderMatches, eh)
		}
	}

	if match.Method != "" {
		em.MethodMatches = []string{match.Method}
	}

	if len(match.QueryParams) > 0 {
		em.QueryParameterMatches = make([]*pbproxystate.QueryParameterMatch, 0, len(match.QueryParams))
		for _, qm := range match.QueryParams {
			eq := &pbproxystate.QueryParameterMatch{
				Name: qm.Name,
			}

			//	*QueryParameterMatch_Exact
			// enumcover:pbmesh.QueryParamMatchType
			switch qm.Type {
			case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT:
				eq.Match = &pbproxystate.QueryParameterMatch_Exact{
					Exact: qm.Value,
				}
			case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_REGEX:
				eq.Match = &pbproxystate.QueryParameterMatch_Regex{
					Regex: qm.Value,
				}
			case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_PRESENT:
				eq.Match = &pbproxystate.QueryParameterMatch_Present{
					Present: true,
				}
			case pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_UNSPECIFIED:
				fallthrough // to default
			default:
				panic(fmt.Sprintf("unknown query param match type: %v", qm.Type))
			}

			em.QueryParameterMatches = append(em.QueryParameterMatches, eq)
		}
	}

	return em
}

func translateTimeouts(timeouts *pbmesh.HTTPRouteTimeouts) *pbproxystate.TimeoutConfig {
	if timeouts == nil || (timeouts.Request == nil && timeouts.Idle == nil) {
		return nil
	}

	return &pbproxystate.TimeoutConfig{
		Timeout:     timeouts.Request,
		IdleTimeout: timeouts.Idle,
	}
}

func translateRetries(retries *pbmesh.HTTPRouteRetries) *pbproxystate.RetryPolicy {
	if retries == nil {
		return nil
	}

	retryPolicy := &pbproxystate.RetryPolicy{}
	if retries.Number != nil {
		retryPolicy.NumRetries = retries.Number
	}

	// The RetryOn magic values come from: https://www.envoyproxy.io/docs/envoy/v1.10.0/configuration/http_filters/router_filter#config-http-filters-router-x-envoy-retry-on
	var retryStrings []string

	if len(retries.OnConditions) > 0 {
		retryStrings = append(retryStrings, retries.OnConditions...)
	}

	if retries.OnConnectFailure {
		// connect-failure can be enabled by either adding connect-failure to the RetryOn list or by using the legacy RetryOnConnectFailure option
		// Check that it's not already in the RetryOn list, so we don't set it twice
		connectFailureExists := false
		for _, r := range retryStrings {
			if r == "connect-failure" {
				connectFailureExists = true
			}
		}
		if !connectFailureExists {
			retryStrings = append(retryStrings, "connect-failure")
		}
	}

	if len(retries.OnStatusCodes) > 0 {
		retryStrings = append(retryStrings, "retriable-status-codes")
		retryPolicy.RetriableStatusCodes = retries.OnStatusCodes
	}

	retryPolicy.RetryOn = strings.Join(retryStrings, ",")

	return retryPolicy
}

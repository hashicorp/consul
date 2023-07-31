// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extensioncommon

import (
	"fmt"
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"
)

// UpstreamEnvoyExtender facilitates uncommon scenarios in which an upstream service's extension needs to apply changes
// to downstram proxies. Separating this mode from the more typical case of extensions patching just the local proxy for
// the configured service allows us to more effectively enforce controls over this elevated level of privilege.
//
// THIS EXTENDER SHOULD NOT BE USED BY ANY NEW EXTENSIONS! It is only intended for use by the builtin AWS Lambda
// extension and Validate (read-only) pseudo-extension to support their existing behavior. Future changes to the
// extension API will introduce stronger controls around privileged capabilities, at which point this extender can be
// removed.
//
// See documentation in RuntimeConfig.IsSourcedFromUpstream for more details.
type UpstreamEnvoyExtender struct {
	Extension BasicExtension
}

var _ EnvoyExtender = (*UpstreamEnvoyExtender)(nil)

func (ext *UpstreamEnvoyExtender) Validate(_ *RuntimeConfig) error {
	return nil
}

func (ext *UpstreamEnvoyExtender) Extend(resources *xdscommon.IndexedResources, config *RuntimeConfig) (*xdscommon.IndexedResources, error) {
	var resultErr error

	// Assert that extension configuration is exclusively from upstreams of the local service.
	if !config.IsSourcedFromUpstream {
		return nil, fmt.Errorf("%q extension applied as upstream config but is not sourced from an upstream of the local service", config.EnvoyExtension.Name)
	}

	// Only the AWS Lambda and Validate extensions are allowed to apply to downstream proxies.
	switch config.EnvoyExtension.Name {
	case api.BuiltinAWSLambdaExtension, api.BuiltinValidateExtension:
	default:
		return nil, fmt.Errorf("extension %q is not permitted to be applied via upstream service config", config.EnvoyExtension.Name)
	}

	// The extensions used by this extender only support terminating gateways and connect proxies.
	switch config.Kind {
	case api.ServiceKindTerminatingGateway, api.ServiceKindConnectProxy:
	default:
		return resources, nil
	}

	if !ext.Extension.CanApply(config) {
		return resources, nil
	}

	for _, indexType := range []string{
		xdscommon.ListenerType,
		xdscommon.RouteType,
		xdscommon.ClusterType,
	} {
		for nameOrSNI, msg := range resources.Index[indexType] {
			switch resource := msg.(type) {
			case *envoy_cluster_v3.Cluster:
				// If the Envoy extension configuration is for an upstream service, the Cluster's
				// name must match the upstream service's SNI.
				if !config.MatchesUpstreamServiceSNI(nameOrSNI) {
					continue
				}

				newCluster, patched, err := ext.Extension.PatchCluster(config, resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching cluster: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.ClusterType][nameOrSNI] = newCluster
				}

			case *envoy_listener_v3.Listener:
				newListener, patched, err := ext.patchListener(config, resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.ListenerType][nameOrSNI] = newListener
				}

			case *envoy_route_v3.RouteConfiguration:
				// If the Envoy extension configuration is for an upstream service, the Route's
				// name must match the upstream service's Envoy ID.
				matchesEnvoyID := config.UpstreamEnvoyID() == nameOrSNI
				if !config.MatchesUpstreamServiceSNI(nameOrSNI) && !matchesEnvoyID {
					continue
				}

				newRoute, patched, err := ext.Extension.PatchRoute(config, resource)
				if err != nil {
					resultErr = multierror.Append(resultErr, fmt.Errorf("error patching route: %w", err))
					continue
				}
				if patched {
					resources.Index[xdscommon.RouteType][nameOrSNI] = newRoute
				}
			default:
				resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported type was skipped: %T", resource))
			}
		}
	}

	return resources, resultErr
}

func (ext *UpstreamEnvoyExtender) patchListener(config *RuntimeConfig, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	switch config.Kind {
	case api.ServiceKindTerminatingGateway:
		return ext.patchTerminatingGatewayListener(config, l)
	case api.ServiceKindConnectProxy:
		return ext.patchConnectProxyListener(config, l)
	}
	return l, false, nil
}

func (ext *UpstreamEnvoyExtender) patchTerminatingGatewayListener(config *RuntimeConfig, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error
	patched := false
	for _, filterChain := range l.FilterChains {
		sni := getSNI(filterChain)

		if sni == "" {
			continue
		}

		// The filter chain's SNI must match the upstream service's SNI.
		if !config.MatchesUpstreamServiceSNI(sni) {
			continue
		}

		var filters []*envoy_listener_v3.Filter

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := ext.Extension.PatchFilter(config, filter, IsInboundPublicListener(l))

			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				filters = append(filters, filter)
				continue
			}
			if ok {
				filters = append(filters, newFilter)
				patched = true
			} else {
				filters = append(filters, filter)
			}
		}
		filterChain.Filters = filters
	}

	return l, patched, resultErr
}

func (ext *UpstreamEnvoyExtender) patchConnectProxyListener(config *RuntimeConfig, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error
	envoyID := GetListenerEnvoyID(l)

	// TProxy outbound listeners must be targeted _carefully_ by upstream extensions
	// because they will affect any downstream's local proxy (there's a single outbound
	// listener for all upstreams). Resources specific to that upstream such as the
	// individual filter that targets the upstream should be targeted.
	if IsOutboundTProxyListener(l) {
		return ext.patchTProxyListener(config, l)
	}

	// If the Envoy extension configuration is for an upstream service, the listener's
	// name must match the upstream service's EnvoyID or be the outbound listener.
	if envoyID != config.UpstreamEnvoyID() {
		return l, false, nil
	}

	// Below is where we handle upstream listeners when not in TProxy mode.
	var patched bool
	for _, filterChain := range l.FilterChains {
		var filters []*envoy_listener_v3.Filter

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := ext.Extension.PatchFilter(config, filter, IsInboundPublicListener(l))
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				filters = append(filters, filter)
				continue
			}

			if ok {
				filters = append(filters, newFilter)
				patched = true
			} else {
				filters = append(filters, filter)
			}
		}
		filterChain.Filters = filters
	}

	return l, patched, resultErr
}

func (ext *UpstreamEnvoyExtender) patchTProxyListener(config *RuntimeConfig, l *envoy_listener_v3.Listener) (proto.Message, bool, error) {
	var resultErr error
	patched := false

	upstream := config.Upstreams[config.ServiceName]
	if upstream == nil {
		return l, false, nil
	}
	vip := upstream.VIP

	for _, filterChain := range l.FilterChains {
		var filters []*envoy_listener_v3.Filter

		match := filterChainTProxyMatch(vip, filterChain)
		if !match {
			continue
		}

		for _, filter := range filterChain.Filters {
			newFilter, ok, err := ext.Extension.PatchFilter(config, filter, IsInboundPublicListener(l))
			if err != nil {
				resultErr = multierror.Append(resultErr, fmt.Errorf("error patching listener filter: %w", err))
				filters = append(filters, filter)
				continue
			}

			if ok {
				filters = append(filters, newFilter)
				patched = true
			} else {
				filters = append(filters, filter)
			}
		}
		filterChain.Filters = filters
	}

	return l, patched, resultErr
}

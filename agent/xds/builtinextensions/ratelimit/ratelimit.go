package ratelimit

import (
	"errors"
	"fmt"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_ratelimit "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

type ratelimit struct {
	ProxyType     string
	Listener      string
	MaxTokens     int
	TokensPerFill int
	FillInterval  int
}

var _ builtinextensiontemplate.Plugin = (*ratelimit)(nil)

// MakeRatelimitExtension is a builtinextensiontemplate.PluginConstructor for a builtinextensiontemplate.EnvoyExtension.
func MakeRatelimitExtension(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.Plugin, error) {
	var resultErr error
	var plugin ratelimit

	if name := ext.EnvoyExtension.Name; name != api.BuiltinRatelimitExtension {
		return nil, fmt.Errorf("expected extension name 'ratelimit' but got %q", name)
	}

	if err := mapstructure.Decode(ext.EnvoyExtension.Arguments, &plugin); err != nil {
		return nil, fmt.Errorf("error decoding extension arguments: %v", err)
	}

	if plugin.FillInterval == 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("fillInterval is required"))
	}

	if plugin.MaxTokens == 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("MaxTokens must be greater than 0"))
	}

	if plugin.TokensPerFill == 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("TokensPerFill must be greater than 0"))
	}

	if err := validateProxyType(plugin.ProxyType); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	if err := validateListener(plugin.Listener); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return plugin, resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p ratelimit) CanApply(config xdscommon.ExtensionConfiguration) bool {
	return string(config.Kind) == p.ProxyType && p.matchesListenerDirection(config)
}

func (p ratelimit) matchesListenerDirection(config xdscommon.ExtensionConfiguration) bool {
	return !config.IsUpstream() && p.Listener == "inbound"
}

// PatchRoute does nothing.
func (p ratelimit) PatchRoute(route *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return route, false, nil
}

// PatchCluster does nothing.
func (p ratelimit) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, nil
}

// PatchFilter inserts a http local rate_limit filter at the head of
// envoy.filters.network.http_connection_manager filters
func (p ratelimit) PatchFilter(filter *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	if filter.Name != "envoy.filters.network.http_connection_manager" {
		return filter, false, nil
	}
	if typedConfig := filter.GetTypedConfig(); typedConfig == nil {
		return filter, false, errors.New("error getting typed config for http filter")
	}

	config := envoy_resource_v3.GetHTTPConnectionManager(filter)
	if config == nil {
		return filter, false, errors.New("error unmarshalling filter")
	}

	tokenBucket := envoy_type_v3.TokenBucket{
		MaxTokens: uint32(p.MaxTokens),
		TokensPerFill: &wrappers.UInt32Value{
			Value: uint32(p.TokensPerFill),
		},
		FillInterval: durationpb.New(time.Duration(p.FillInterval) * time.Second),
	}

	ratelimitHttpFilter, err := makeEnvoyHTTPFilter(
		"envoy.filters.http.local_ratelimit",
		&envoy_ratelimit.LocalRateLimit{
			TokenBucket: &tokenBucket,
			StatPrefix:  "local_ratelimit.",
			FilterEnabled: &envoy_core_v3.RuntimeFractionalPercent{
				DefaultValue: &envoy_type_v3.FractionalPercent{
					Numerator:   100,
					Denominator: envoy_type_v3.FractionalPercent_HUNDRED,
				},
			},
			FilterEnforced: &envoy_core_v3.RuntimeFractionalPercent{
				DefaultValue: &envoy_type_v3.FractionalPercent{
					Numerator:   100,
					Denominator: envoy_type_v3.FractionalPercent_HUNDRED,
				},
			},
		},
	)
	if err != nil {
		return filter, false, err
	}

	changedFilters := make([]*envoy_http_v3.HttpFilter, 0, len(config.HttpFilters)+1)

	// The ratelimitHttpFilter is inserted as the first element of the http
	// filter chain.
	changedFilters = append(changedFilters, ratelimitHttpFilter)
	changedFilters = append(changedFilters, config.HttpFilters...)
	config.HttpFilters = changedFilters

	newFilter, err := makeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}

func validateProxyType(t string) error {
	if t != "connect-proxy" {
		return fmt.Errorf("unexpected ProxyType %q", t)
	}

	return nil
}

func validateListener(t string) error {
	if t != "inbound" && t != "outbound" {
		return fmt.Errorf("unexpected Listener %q", t)
	}

	return nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package localratelimit

import (
	"errors"
	"fmt"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_ratelimit "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/local_ratelimit/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

type ratelimit struct {
	extensioncommon.BasicExtensionAdapter

	ProxyType string

	// Token bucket of the rate limit
	MaxTokens     *int
	TokensPerFill *int
	FillInterval  *int

	// Percent of requests to be rate limited
	FilterEnabled  *uint32
	FilterEnforced *uint32
}

var _ extensioncommon.BasicExtension = (*ratelimit)(nil)

// Constructor follows a specific function signature required for the extension registration.
func Constructor(ext api.EnvoyExtension) (extensioncommon.EnvoyExtender, error) {
	var r ratelimit
	if name := ext.Name; name != api.BuiltinLocalRatelimitExtension {
		return nil, fmt.Errorf("expected extension name 'ratelimit' but got %q", name)
	}

	if err := r.fromArguments(ext.Arguments); err != nil {
		return nil, err
	}

	return &extensioncommon.BasicEnvoyExtender{
		Extension: &r,
	}, nil
}

func (r *ratelimit) fromArguments(args map[string]interface{}) error {
	if err := mapstructure.Decode(args, r); err != nil {
		return fmt.Errorf("error decoding extension arguments: %v", err)
	}
	if r.ProxyType == "" {
		r.ProxyType = string(api.ServiceKindConnectProxy)
	}
	return r.validate()
}

func (r *ratelimit) validate() error {
	var resultErr error

	// NOTE: Envoy requires FillInterval value must be greater than 0.
	// If unset, it is considered as 0.
	if r.FillInterval == nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("FillInterval(in second) is missing"))
	} else if *r.FillInterval <= 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("FillInterval(in second) must be greater than 0, got %d", *r.FillInterval))
	}

	// NOTE: Envoy requires MaxToken value must be greater than 0.
	// If unset, it is considered as 0.
	if r.MaxTokens == nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("MaxTokens is missing"))
	} else if *r.MaxTokens <= 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("MaxTokens must be greater than 0, got %d", r.MaxTokens))
	}

	// TokensPerFill is allowed to unset. In this case, envoy
	// uses its default value, which is 1.
	if r.TokensPerFill != nil && *r.TokensPerFill <= 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("TokensPerFill must be greater than 0, got %d", *r.TokensPerFill))
	}

	if err := validateProxyType(r.ProxyType); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return resultErr
}

// CanApply determines if the extension can apply to the given extension configuration.
func (p *ratelimit) CanApply(config *extensioncommon.RuntimeConfig) bool {
	return string(config.Kind) == p.ProxyType
}

// PatchFilter inserts a http local rate_limit filter at the head of
// envoy.filters.network.http_connection_manager filters
func (r ratelimit) PatchFilter(p extensioncommon.FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
	filter := p.Message
	// rate limit is only applied to the inbound listener of the service itself
	// since the limit is aggregated from all downstream connections.
	if !p.IsInbound() {
		return filter, false, nil
	}

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

	tokenBucket := envoy_type_v3.TokenBucket{}

	if r.TokensPerFill != nil {
		tokenBucket.TokensPerFill = &wrapperspb.UInt32Value{
			Value: uint32(*r.TokensPerFill),
		}
	}
	if r.MaxTokens != nil {
		tokenBucket.MaxTokens = uint32(*r.MaxTokens)
	}

	if r.FillInterval != nil {
		tokenBucket.FillInterval = durationpb.New(time.Duration(*r.FillInterval) * time.Second)
	}

	var FilterEnabledDefault *envoy_core_v3.RuntimeFractionalPercent
	if r.FilterEnabled != nil {
		FilterEnabledDefault = &envoy_core_v3.RuntimeFractionalPercent{
			DefaultValue: &envoy_type_v3.FractionalPercent{
				Numerator:   *r.FilterEnabled,
				Denominator: envoy_type_v3.FractionalPercent_HUNDRED,
			},
		}
	}

	var FilterEnforcedDefault *envoy_core_v3.RuntimeFractionalPercent
	if r.FilterEnforced != nil {
		FilterEnforcedDefault = &envoy_core_v3.RuntimeFractionalPercent{
			DefaultValue: &envoy_type_v3.FractionalPercent{
				Numerator:   *r.FilterEnforced,
				Denominator: envoy_type_v3.FractionalPercent_HUNDRED,
			},
		}
	}

	ratelimitHttpFilter, err := extensioncommon.MakeEnvoyHTTPFilter(
		"envoy.filters.http.local_ratelimit",
		&envoy_ratelimit.LocalRateLimit{
			TokenBucket:    &tokenBucket,
			StatPrefix:     "local_ratelimit",
			FilterEnabled:  FilterEnabledDefault,
			FilterEnforced: FilterEnforcedDefault,
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

	newFilter, err := extensioncommon.MakeFilter("envoy.filters.network.http_connection_manager", config)
	if err != nil {
		return filter, false, errors.New("error making new filter")
	}

	return newFilter, true, nil
}

func validateProxyType(t string) error {
	if t != string(api.ServiceKindConnectProxy) {
		return fmt.Errorf("unexpected ProxyType %q", t)
	}

	return nil
}

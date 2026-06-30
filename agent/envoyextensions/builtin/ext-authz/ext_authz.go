// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package extauthz

import (
	"fmt"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/go-viper/mapstructure/v2"

	"github.com/hashicorp/consul/api"
	ext_cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
)

type extAuthz struct {
	ext_cmn.BasicExtensionAdapter

	// ProxyType identifies the type of Envoy proxy that this extension applies to.
	// The extension will only be configured for proxies that match this type and
	// will be ignored for all other proxy types.
	ProxyType api.ServiceKind
	// InsertOptions controls how the extension inserts the filter.
	InsertOptions ext_cmn.InsertOptions
	// ListenerType controls which listener the extension applies to. It supports "inbound" or "outbound" listeners.
	ListenerType string
	// Config holds the extension configuration.
	Config extAuthzConfig
}

var _ ext_cmn.BasicExtension = (*extAuthz)(nil)

func Constructor(ext api.EnvoyExtension) (ext_cmn.EnvoyExtender, error) {
	auth, err := newExtAuthz(ext)
	if err != nil {
		return nil, err
	}
	return &ext_cmn.BasicEnvoyExtender{
		Extension: auth,
	}, nil
}

// CanApply indicates if the ext-authz extension can be applied to the given extension runtime configuration.
func (a *extAuthz) CanApply(config *ext_cmn.RuntimeConfig) bool {
	return config.Kind == api.ServiceKindConnectProxy ||
		(apiGatewayExtAuthzSupported() && config.Kind == api.ServiceKindAPIGateway)
}

// PatchClusters modifies the cluster resources for the ext-authz extension.
//
// If the extension is configured to target an ext-authz service running on the local host network
// this func will insert a cluster for calling that service. It does nothing if the extension is
// configured to target an upstream service because the existing cluster for the upstream will be
// used directly by the filter.
func (a *extAuthz) PatchClusters(cfg *ext_cmn.RuntimeConfig, c ext_cmn.ClusterMap) (ext_cmn.ClusterMap, error) {
	cluster, err := a.Config.toEnvoyCluster(cfg)
	if err != nil {
		return c, err
	}
	if cluster != nil {
		c[cluster.Name] = cluster
	}
	return c, nil
}

func (a *extAuthz) matchesListenerDirection(isInboundListener bool) bool {
	return (!isInboundListener && a.ListenerType == "outbound") || (isInboundListener && a.ListenerType == "inbound")
}

// PatchFilters inserts an ext-authz filter into the list of network filters or the filter chain of the HTTP connection manager.
func (a *extAuthz) PatchFilters(cfg *ext_cmn.RuntimeConfig, filters []*envoy_listener_v3.Filter, isInboundListener bool) ([]*envoy_listener_v3.Filter, error) {
	// On an API Gateway the ext_authz filter is applied to the gateway's HTTP
	// listeners. Gateway listeners are not the sidecar "public_listener" and so are
	// not classified as inbound; we therefore bypass the inbound/outbound direction
	// check that applies to sidecar proxies. This path is enterprise-only.
	if apiGatewayExtAuthzSupported() && cfg.Kind == api.ServiceKindAPIGateway {
		return a.patchAPIGatewayFilters(cfg, filters)
	}

	// The ext_authz extension only patches filters for inbound listeners.
	if !a.matchesListenerDirection(isInboundListener) {
		return filters, nil
	}

	a.configureInsertOptions(cfg.Protocol)

	switch cfg.Protocol {
	case "grpc", "http2", "http":
		extAuthzFilter, err := a.Config.toEnvoyHttpFilter(cfg)
		if err != nil {
			return filters, err
		}
		return ext_cmn.InsertHTTPFilter(filters, extAuthzFilter, a.InsertOptions)
	case "tcp":
		fallthrough
	default:
		extAuthzFilter, err := a.Config.toEnvoyNetworkFilter(cfg)
		if err != nil {
			return filters, err
		}
		return ext_cmn.InsertNetworkFilter(filters, extAuthzFilter, a.InsertOptions)
	}
}

// patchAPIGatewayFilters inserts the ext_authz HTTP filter into the HTTP connection
// manager of an API Gateway listener filter chain. Filter chains that are not managed
// by an HTTP connection manager (e.g. TCPRoute listeners) are skipped without error.
func (a *extAuthz) patchAPIGatewayFilters(cfg *ext_cmn.RuntimeConfig, filters []*envoy_listener_v3.Filter) ([]*envoy_listener_v3.Filter, error) {
	if _, _, err := ext_cmn.GetHTTPConnectionManager(filters...); err != nil {
		// No HTTP connection manager on this filter chain; nothing to patch.
		return filters, nil
	}

	a.configureInsertOptions("http")
	extAuthzFilter, err := a.Config.toEnvoyHttpFilter(cfg)
	if err != nil {
		return filters, err
	}
	return ext_cmn.InsertHTTPFilter(filters, extAuthzFilter, a.InsertOptions)
}

func newExtAuthz(ext api.EnvoyExtension) (*extAuthz, error) {
	auth := &extAuthz{}
	if ext.Name != api.BuiltinExtAuthzExtension {
		return auth, fmt.Errorf("expected extension name %q but got %q", api.BuiltinExtAuthzExtension, ext.Name)
	}
	if err := auth.fromArguments(ext.Arguments); err != nil {
		return auth, err
	}
	// By default the filter's failure mode derives from whether the extension is
	// required (required => fail closed). An explicit Config.FailureModeAllow, if
	// set, takes precedence and overrides that default.
	auth.Config.failureModeAllow = !ext.Required
	if auth.Config.FailureModeAllow != nil {
		auth.Config.failureModeAllow = *auth.Config.FailureModeAllow
	}
	return auth, nil
}

func (a *extAuthz) fromArguments(args map[string]any) error {
	if err := mapstructure.Decode(args, a); err != nil {
		return err
	}
	a.normalize()
	return a.validate()
}

func (a *extAuthz) configureInsertOptions(protocol string) {
	// If the insert options have been expressly configured, then use them.
	if a.InsertOptions.Location != "" {
		return
	}

	// Configure the default, insert the filter immediately before the terminal filter.
	a.InsertOptions.Location = ext_cmn.InsertBeforeFirstMatch
	switch protocol {
	case "grpc", "http2", "http":
		a.InsertOptions.FilterName = "envoy.filters.http.router"
	default:
		a.InsertOptions.FilterName = "envoy.filters.network.tcp_proxy"
	}
}

func (a *extAuthz) normalize() {
	if a.ProxyType == "" {
		a.ProxyType = api.ServiceKindConnectProxy
	}

	if a.ListenerType == "" {
		a.ListenerType = "inbound"
	}

	a.Config.normalize()
	// Propagate the resolved proxy kind so extAuthzConfig.validate() can apply
	// kind-specific rules (e.g. URI host restrictions differ between sidecar and gateway).
	a.Config.proxyKind = a.ProxyType
}

func (a *extAuthz) validate() error {
	var resultErr error
	switch {
	case a.ProxyType == api.ServiceKindConnectProxy:
	case a.ProxyType == api.ServiceKindAPIGateway && apiGatewayExtAuthzSupported():
	default:
		// The set of supported proxy types depends on the build: API Gateway
		// support for ext-authz is enterprise-only.
		supported := fmt.Sprintf("%q", api.ServiceKindConnectProxy)
		if apiGatewayExtAuthzSupported() {
			supported = fmt.Sprintf("%q or %q", api.ServiceKindConnectProxy, api.ServiceKindAPIGateway)
		}
		resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported ProxyType %q, supported values are %s",
			a.ProxyType, supported))
	}

	if a.ListenerType != "inbound" && a.ListenerType != "outbound" {
		resultErr = multierror.Append(resultErr, fmt.Errorf(`unexpected ListenerType %q, supported values are "inbound" or "outbound"`, a.ListenerType))
	}

	if err := a.Config.validate(); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return resultErr
}

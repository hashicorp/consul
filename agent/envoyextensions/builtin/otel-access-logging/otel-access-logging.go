// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package otelaccesslogging

import (
	"fmt"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/api"
	ext_cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
)

type otelAccessLogging struct {
	ext_cmn.BasicExtensionAdapter

	// ProxyType identifies the type of Envoy proxy that this extension applies to.
	// The extension will only be configured for proxies that match this type and
	// will be ignored for all other proxy types.
	ProxyType api.ServiceKind
	// InsertOptions controls how the extension inserts the filter.
	InsertOptions ext_cmn.InsertOptions
	// Config holds the extension configuration.
	Config AccessLog
}

var _ ext_cmn.BasicExtension = (*otelAccessLogging)(nil)

func Constructor(ext api.EnvoyExtension) (ext_cmn.EnvoyExtender, error) {
	auth, err := newOTELAccessLogging(ext)
	if err != nil {
		return nil, err
	}
	return &ext_cmn.BasicEnvoyExtender{
		Extension: auth,
	}, nil
}

// CanApply indicates if the ext-authz extension can be applied to the given extension runtime configuration.
func (a *otelAccessLogging) CanApply(config *ext_cmn.RuntimeConfig) bool {
	return config.Kind == api.ServiceKindConnectProxy
}

// PatchClusters modifies the cluster resources for the ext-authz extension.
//
// If the extension is configured to target an ext-authz service running on the local host network
// this func will insert a cluster for calling that service. It does nothing if the extension is
// configured to target an upstream service because the existing cluster for the upstream will be
// used directly by the filter.
func (a *otelAccessLogging) PatchClusters(cfg *ext_cmn.RuntimeConfig, c ext_cmn.ClusterMap) (ext_cmn.ClusterMap, error) {
	cluster, err := a.Config.CommonConfig.toEnvoyCluster(cfg)
	if err != nil {
		return c, err
	}
	if cluster != nil {
		c[cluster.Name] = cluster
	}
	return c, nil
}

// PatchFilters inserts an ext-authz filter into the list of network filters or the filter chain of the HTTP connection manager.
func (a *otelAccessLogging) PatchFilters(cfg *ext_cmn.RuntimeConfig, filters []*envoy_listener_v3.Filter, isInboundListener bool) ([]*envoy_listener_v3.Filter, error) {
	// The ext_authz extension only patches filters for inbound listeners.
	if !isInboundListener {
		return filters, nil
	}

	a.configureInsertOptions()

	extAuthzFilter, err := a.Config.toEnvoyNetworkFilter(cfg)
	if err != nil {
		return filters, err
	}
	return ext_cmn.InsertNetworkFilter(filters, extAuthzFilter, a.InsertOptions)
}

func newOTELAccessLogging(ext api.EnvoyExtension) (*otelAccessLogging, error) {
	auth := &otelAccessLogging{}
	if ext.Name != api.BuiltinExtAuthzExtension {
		return auth, fmt.Errorf("expected extension name %q but got %q", api.BuiltinExtAuthzExtension, ext.Name)
	}
	if err := auth.fromArguments(ext.Arguments); err != nil {
		return auth, err
	}

	return auth, nil
}

func (a *otelAccessLogging) fromArguments(args map[string]any) error {
	if err := mapstructure.Decode(args, a); err != nil {
		return err
	}
	a.normalize()
	return a.validate()
}

func (a *otelAccessLogging) configureInsertOptions() {
	// If the insert options have been expressly configured, then use them.
	if a.InsertOptions.Location != "" {
		return
	}

	// Configure the default, insert the filter immediately before the terminal filter.
	a.InsertOptions.Location = ext_cmn.InsertBeforeFirstMatch
	a.InsertOptions.FilterName = "envoy.filters.network.http_connection_manager"
}

func (a *otelAccessLogging) normalize() {
	if a.ProxyType == "" {
		a.ProxyType = api.ServiceKindConnectProxy
	}
	a.Config.normalize()
}

func (a *otelAccessLogging) validate() error {
	var resultErr error
	if a.ProxyType != api.ServiceKindConnectProxy {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported ProxyType %q, only %q is supported",
			a.ProxyType,
			api.ServiceKindConnectProxy))
	}

	if err := a.Config.validate(); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return resultErr
}

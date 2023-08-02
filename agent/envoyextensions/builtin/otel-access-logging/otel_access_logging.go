// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package otelaccesslogging

import (
	"fmt"

	envoy_extensions_access_loggers_v3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_extensions_access_loggers_otel_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/open_telemetry/v3"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
	cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
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
	otel, err := newOTELAccessLogging(ext)
	if err != nil {
		return nil, err
	}
	return &ext_cmn.BasicEnvoyExtender{
		Extension: otel,
	}, nil
}

// CanApply indicates if the extension can be applied to the given extension runtime configuration.
func (a *otelAccessLogging) CanApply(config *ext_cmn.RuntimeConfig) bool {
	return config.Kind == api.ServiceKindConnectProxy
}

// PatchClusters modifies the cluster resources for the extension.
//
// If the extension is configured to target the OTEL service running on the local host network
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

// PatchFilters inserts a filter into the list of network filters or the filter chain of the HTTP connection manager.
func (a *otelAccessLogging) PatchFilter(p ext_cmn.FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
	httpConnectionManager, _, err := ext_cmn.GetHTTPConnectionManager(p.Message)
	if err != nil {
		return nil, false, err
	}

	accessLog, err := a.toEnvoyAccessLog(p.RuntimeConfig)
	if err != nil {
		return nil, false, err
	}

	httpConnectionManager.AccessLog = append(httpConnectionManager.AccessLog, accessLog)
	newHCM, err := ext_cmn.MakeFilter("envoy.filters.network.http_connection_manager", httpConnectionManager)
	if err != nil {
		return nil, false, err
	}

	return newHCM, true, nil
}

func newOTELAccessLogging(ext api.EnvoyExtension) (*otelAccessLogging, error) {
	otel := &otelAccessLogging{}
	if ext.Name != api.BuiltinOTELAccessLoggingExtension {
		return otel, fmt.Errorf("expected extension name %q but got %q", api.BuiltinOTELAccessLoggingExtension, ext.Name)
	}
	if err := otel.fromArguments(ext.Arguments); err != nil {
		return otel, err
	}

	return otel, nil
}

func (a *otelAccessLogging) fromArguments(args map[string]any) error {
	if err := mapstructure.Decode(args, a); err != nil {
		return err
	}
	a.normalize()
	return a.validate()
}

func (a *otelAccessLogging) toEnvoyAccessLog(cfg *cmn.RuntimeConfig) (*envoy_extensions_access_loggers_v3.AccessLog, error) {
	commonConfig, err := a.Config.CommonConfig.toEnvoy(cfg)
	if err != nil {
		return nil, err
	}

	otelAccessLogConfig := &envoy_extensions_access_loggers_otel_v3.OpenTelemetryAccessLogConfig{
		CommonConfig:       commonConfig,
		ResourceAttributes: a.Config.Attributes,
		Body:               a.Config.Body,
		Attributes:         a.Config.Attributes,
	}

	// Marshal the struct to bytes.
	otelAccessLogConfigBytes, err := protojson.Marshal(otelAccessLogConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenTelemetryAccessLogConfig: %w", err)
	}

	return &envoy_extensions_access_loggers_v3.AccessLog{
		Name: "envoy.access_loggers.open_telemetry",
		ConfigType: &envoy_extensions_access_loggers_v3.AccessLog_TypedConfig{
			TypedConfig: &anypb.Any{
				Value:   otelAccessLogConfigBytes,
				TypeUrl: "type.googleapis.com/envoy.extensions.access_loggers.open_telemetry.v3.OpenTelemetryAccessLogConfig",
			},
		},
	}, nil
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

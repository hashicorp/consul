// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package otelaccesslogging

import (
	"errors"
	"fmt"

	envoy_extensions_access_loggers_v3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_extensions_access_loggers_otel_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/open_telemetry/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
	ext_cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
	v1 "go.opentelemetry.io/proto/otlp/common/v1"
)

type otelAccessLogging struct {
	ext_cmn.BasicExtensionAdapter

	// ProxyType identifies the type of Envoy proxy that this extension applies to.
	// The extension will only be configured for proxies that match this type and
	// will be ignored for all other proxy types.
	ProxyType api.ServiceKind
	// InsertOptions controls how the extension inserts the filter.
	InsertOptions ext_cmn.InsertOptions
	ListenerType  string
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

func (a *otelAccessLogging) matchesListenerDirection(p extensioncommon.FilterPayload) bool {
	isInboundListener := p.IsInbound()
	return (!isInboundListener && a.ListenerType == "outbound") || (isInboundListener && a.ListenerType == "inbound")
}

// PatchFilter adds the OTEL access log in the HTTP connection manager.
func (a *otelAccessLogging) PatchFilter(p ext_cmn.FilterPayload) (*envoy_listener_v3.Filter, bool, error) {
	filter := p.Message
	// Make sure filter matches extension config.
	if !a.matchesListenerDirection(p) {
		return filter, false, nil
	}

	if filter.Name != "envoy.filters.network.http_connection_manager" {
		return filter, false, nil
	}
	if typedConfig := filter.GetTypedConfig(); typedConfig == nil {
		return filter, false, errors.New("error getting typed config for http filter")
	}

	httpConnectionManager := envoy_resource_v3.GetHTTPConnectionManager(filter)
	if httpConnectionManager == nil {
		return filter, false, errors.New("error unmarshalling filter")
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
	if err := a.normalize(); err != nil {
		return err
	}
	return a.validate()
}

func (a *otelAccessLogging) toEnvoyAccessLog(cfg *cmn.RuntimeConfig) (*envoy_extensions_access_loggers_v3.AccessLog, error) {
	commonConfig, err := a.Config.CommonConfig.toEnvoy(cfg)
	if err != nil {
		return nil, err
	}

	body, err := toEnvoyAnyValue(a.Config.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Body: %w", err)
	}

	attributes, err := toEnvoyKeyValueList(a.Config.Attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Attributes: %w", err)
	}

	resourceAttributes, err := toEnvoyKeyValueList(a.Config.ResourceAttributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ResourceAttributes: %w", err)
	}

	otelAccessLogConfig := &envoy_extensions_access_loggers_otel_v3.OpenTelemetryAccessLogConfig{
		CommonConfig:       commonConfig,
		Body:               body,
		Attributes:         attributes,
		ResourceAttributes: resourceAttributes,
	}

	// Marshal the struct to bytes.
	otelAccessLogConfigBytes, err := proto.Marshal(otelAccessLogConfig)
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

func (a *otelAccessLogging) normalize() error {
	if a.ProxyType == "" {
		a.ProxyType = api.ServiceKindConnectProxy
	}

	if a.ListenerType == "" {
		a.ListenerType = "inbound"
	}

	return a.Config.normalize(a.ListenerType)
}

func (a *otelAccessLogging) validate() error {
	var resultErr error
	if a.ProxyType != api.ServiceKindConnectProxy {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported ProxyType %q, only %q is supported",
			a.ProxyType,
			api.ServiceKindConnectProxy))
	}

	if a.ListenerType != "inbound" && a.ListenerType != "outbound" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unexpected Listener %q", a.ListenerType))
	}

	if err := a.Config.validate(a.ListenerType); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	return resultErr
}

func toEnvoyKeyValueList(attributes map[string]any) (*v1.KeyValueList, error) {
	keyValueList := &v1.KeyValueList{}
	for key, value := range attributes {
		anyValue, err := toEnvoyAnyValue(value)
		if err != nil {
			return nil, err
		}
		keyValueList.Values = append(keyValueList.Values, &v1.KeyValue{
			Key:   key,
			Value: anyValue,
		})
	}

	return keyValueList, nil
}

func toEnvoyAnyValue(value interface{}) (*v1.AnyValue, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case string:
		return &v1.AnyValue{
			Value: &v1.AnyValue_StringValue{
				StringValue: v,
			},
		}, nil
	case int:
		return &v1.AnyValue{
			Value: &v1.AnyValue_IntValue{
				IntValue: int64(v),
			},
		}, nil
	case int32:
		return &v1.AnyValue{
			Value: &v1.AnyValue_IntValue{
				IntValue: int64(v),
			},
		}, nil
	case int64:
		return &v1.AnyValue{
			Value: &v1.AnyValue_IntValue{
				IntValue: v,
			},
		}, nil
	case float32:
		return &v1.AnyValue{
			Value: &v1.AnyValue_DoubleValue{
				DoubleValue: float64(v),
			},
		}, nil
	case float64:
		return &v1.AnyValue{
			Value: &v1.AnyValue_DoubleValue{
				DoubleValue: v,
			},
		}, nil
	case bool:
		return &v1.AnyValue{
			Value: &v1.AnyValue_BoolValue{
				BoolValue: v,
			},
		}, nil
	case []byte:
		return &v1.AnyValue{
			Value: &v1.AnyValue_BytesValue{
				BytesValue: v,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", v)
	}
}

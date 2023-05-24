// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wasm

import (
	"fmt"
	"net/url"
	"time"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_wasm_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// wasmConfig defines the configuration for a Wasm Envoy extension.
type wasmConfig struct {
	// Protocol is the type of Wasm filter to apply, "tcp" or "http".
	Protocol string
	// ProxyType identifies the type of Envoy proxy that this extension applies to.
	// The extension will only be configured for proxies that match this type and
	// will be ignored for all other proxy types.
	ProxyType api.ServiceKind
	// ListenerType identifies the listener type the filter will be applied to.
	ListenerType string
	// PluginConfig holds the configuration for the Wasm plugin.
	PluginConfig pluginConfig
}

// pluginConfig defines a Wasm plugin configuration.
type pluginConfig struct {
	// Name is the unique name for the filter in a VM. For use in identifying the
	// filter if multiple filters are handled by the same VmID and RootID.
	// Also used for logging/debugging.
	Name string
	// RootID is a unique ID for a set of filters in a VM which will share a
	// RootContext and Contexts if applicable (e.g. a Wasm HttpFilter and a Wasm AccessLog).
	// All filters with the same RootID and VmID will share Context(s).
	RootID string
	// VmConfig is the configuration for starting or finding the Wasm VM that the
	// filter will run in.
	VmConfig vmConfig
	// Configuration holds the configuration that will be encoded as bytes and passed to
	// the plugin on startup (proxy_on_configure).
	Configuration string
	// CapabilityRestrictionConfiguration controls the Wasm ABI capabilities available
	// to the filter.
	CapabilityRestrictionConfiguration capabilityRestrictionConfiguration

	// failOpen controls the behavior when a runtime error occurs during filter
	// processing.
	//
	// If set to false runtime errors will result in a failed request.
	// For TCP filters, the connection will be closed. For HTTP filters a 503
	// status is returned.
	//
	// If set to true, a runtime error will result in the filter being bypassed.
	failOpen bool
}

// vmConfig defines a Wasm VM configuration.
type vmConfig struct {
	// VmID is an ID which will be used along with a hash of the Wasm code to
	// determine which VM will be used for the plugin. All plugins which use
	// the same VmID and code will use the same VM. May be left blank.
	VmID string
	// Runtime is the Wasm runtime type, one of: v8, wasmtime, wamr, or wavm.
	Runtime string
	// Code references the Wasm code that will run in the filter.
	Code dataSource
	// Configuration holds the configuration that will be encoded as bytes and
	// passed to the plugin during VM startup (proxy_on_vm_start).
	Configuration string
	// EnvironmentVariables specifies environment variables to be injected to
	// this VM which will be available through WASI’s environ_get and
	// environ_get_sizes system calls.
	EnvironmentVariables environmentVariables
}

// dataSource defines a local or remote location where Wasm code will be loaded from.
type dataSource struct {
	// Local supports loading files from a local volume.
	Local localDataSource
	// Remote supports loading files from a remote server.
	Remote remoteDataSource
}

// environmentVariables defines the environment variables that will be made available
// to the Wasm filter.
type environmentVariables struct {
	// HostEnvKeys holds the keys of Envoy’s environment variables exposed to this VM.
	// If a key exists in Envoy’s environment variables, then that key-value pair will
	// be injected into the Wasm VM. If a key does not exist, it will be ignored.
	HostEnvKeys []string
	// KeyValues is a list of key-value pairs to be injected to this VM in the form of "KEY=VALUE".
	KeyValues map[string]string
}

// localDataSource defines a file from a local file system.
type localDataSource struct {
	// Filename is the path to the Wasm file on the local file system.
	Filename string
}

// remoteDataSource defines a file from a remote file server.
type remoteDataSource struct {
	// HttpURI
	HttpURI httpURI
	// SHA256 of the remote file. Used to validate the remote file.
	SHA256 string
	// RetryPolicy determines how retries are handled.
	RetryPolicy retryPolicy
}

// httpURI defines a remote file using an HTTP URI.
type httpURI struct {
	// Service is the upstream service the Wasm plugin will be fetched from.
	Service api.CompoundServiceName
	// URI is the location of the Wasm file on the upstream service.
	URI string
	// Timeout sets the maximum duration that a response can take.
	Timeout string

	timeout time.Duration
}

// retryPolicy defines how to handle retries when fetching remote files.
type retryPolicy struct {
	// RetryBackOff holds parameters that control retry backoff strategy.
	RetryBackOff retryBackoff
	// NumRetries specifies the allowed number of retries.
	NumRetries int
}

// retryBackoff holds parameters that control retry backoff strategy.
type retryBackoff struct {
	// BaseInterval is the base interval to be used for the next back off
	// computation. It should be greater than zero and less than or equal
	// to MaxInterval.
	BaseInterval string
	// MaxInterval is the maximum interval between retries.
	MaxInterval string

	baseInterval time.Duration
	maxInterval  time.Duration
}

// capabilityRestrictionConfiguration controls Wasm capabilities available to modules.
type capabilityRestrictionConfiguration struct {
	// AllowedCapabilities specifies the Wasm capabilities which will be allowed.
	// Capabilities are mapped by name. The value which each capability maps to is
	// currently ignored and should be left empty.
	AllowedCapabilities map[string]any
}

// newWasmConfig creates a filterConfig from the given args.
// It starts with the default wasm configuration and merges in the config
// from the given args.
func newWasmConfig(args map[string]any) (*wasmConfig, error) {
	cfg := &wasmConfig{}
	if err := mapstructure.Decode(args, cfg); err != nil {
		return cfg, err
	}
	cfg.normalize()
	return cfg, nil
}

func (p *pluginConfig) asyncDataSource(rtCfg *extensioncommon.RuntimeConfig) (*envoy_core_v3.AsyncDataSource, error) {

	// Local data source
	if filename := p.VmConfig.Code.Local.Filename; filename != "" {
		return &envoy_core_v3.AsyncDataSource{
			Specifier: &envoy_core_v3.AsyncDataSource_Local{
				Local: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: filename,
					},
				},
			},
		}, nil
	}

	// Remote data source
	// For a remote file, ensure there is an upstream cluster for the host specified in the URL.
	// Envoy requires an explicit cluster in order to perform the DNS lookup required to actually
	// fetch the data from the upstream source.
	remote := &p.VmConfig.Code.Remote
	clusterSNI := ""
	for service, upstream := range rtCfg.Upstreams {
		if service == remote.HttpURI.Service {
			for sni := range upstream.SNI {
				clusterSNI = sni
				break
			}
		}
	}
	if clusterSNI == "" {
		return nil, fmt.Errorf("no upstream found for remote service %q", remote.HttpURI.Service.Name)
	}

	d := time.Second
	if remote.HttpURI.timeout > 0 {
		d = remote.HttpURI.timeout
	}
	timeout := &durationpb.Duration{Seconds: int64(d.Seconds())}

	return &envoy_core_v3.AsyncDataSource{
		Specifier: &envoy_core_v3.AsyncDataSource_Remote{
			Remote: &envoy_core_v3.RemoteDataSource{
				Sha256: remote.SHA256,
				HttpUri: &envoy_core_v3.HttpUri{
					Uri: remote.HttpURI.URI,
					HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{
						Cluster: clusterSNI,
					},
					Timeout: timeout,
				},
				RetryPolicy: p.retryPolicy(),
			},
		},
	}, nil
}

func (p *pluginConfig) capConfig() *envoy_wasm_v3.CapabilityRestrictionConfig {
	if len(p.CapabilityRestrictionConfiguration.AllowedCapabilities) == 0 {
		return nil
	}

	allowedCaps := make(map[string]*envoy_wasm_v3.SanitizationConfig)
	for key := range p.CapabilityRestrictionConfiguration.AllowedCapabilities {
		allowedCaps[key] = &envoy_wasm_v3.SanitizationConfig{}
	}

	return &envoy_wasm_v3.CapabilityRestrictionConfig{
		AllowedCapabilities: allowedCaps,
	}
}

func (p *pluginConfig) envoyPluginConfig(rtCfg *extensioncommon.RuntimeConfig) (*envoy_wasm_v3.PluginConfig, error) {
	var err error
	var pluginCfgData, vmCfgData *anypb.Any

	if p.Configuration != "" {
		pluginCfgData, err = anypb.New(wrapperspb.String(p.Configuration))
		if err != nil {
			return nil, fmt.Errorf("failed to encode Wasm plugin configuration: %w", err)
		}
	}

	if p.VmConfig.Configuration != "" {
		vmCfgData, err = anypb.New(wrapperspb.String(p.VmConfig.Configuration))
		if err != nil {
			return nil, fmt.Errorf("failed to encode Wasm VM configuration: %w", err)
		}
	}

	code, err := p.asyncDataSource(rtCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode async data source configuration: %w", err)
	}

	var envVars *envoy_wasm_v3.EnvironmentVariables
	if len(p.VmConfig.EnvironmentVariables.HostEnvKeys) > 0 ||
		len(p.VmConfig.EnvironmentVariables.KeyValues) > 0 {
		envVars = &envoy_wasm_v3.EnvironmentVariables{
			HostEnvKeys: p.VmConfig.EnvironmentVariables.HostEnvKeys,
			KeyValues:   p.VmConfig.EnvironmentVariables.KeyValues,
		}
	}

	return &envoy_wasm_v3.PluginConfig{
		Name:   p.Name,
		RootId: p.RootID,
		Vm: &envoy_wasm_v3.PluginConfig_VmConfig{
			VmConfig: &envoy_wasm_v3.VmConfig{
				VmId:                 p.VmConfig.VmID,
				Runtime:              fmt.Sprintf("envoy.wasm.runtime.%s", p.VmConfig.Runtime),
				Code:                 code,
				Configuration:        vmCfgData,
				EnvironmentVariables: envVars,
			},
		},
		Configuration:               pluginCfgData,
		CapabilityRestrictionConfig: p.capConfig(),
		FailOpen:                    p.failOpen,
	}, nil
}

func (p *pluginConfig) retryPolicy() *envoy_core_v3.RetryPolicy {
	remote := &p.VmConfig.Code.Remote
	if remote.RetryPolicy.NumRetries <= 0 &&
		remote.RetryPolicy.RetryBackOff.BaseInterval == "" &&
		remote.RetryPolicy.RetryBackOff.MaxInterval == "" {
		return nil
	}

	retryPolicy := &envoy_core_v3.RetryPolicy{}

	if remote.RetryPolicy.NumRetries > 0 {
		retryPolicy.NumRetries = wrapperspb.UInt32(uint32(remote.RetryPolicy.NumRetries))
	}

	var baseInterval, maxInterval *durationpb.Duration
	if remote.RetryPolicy.RetryBackOff.baseInterval > 0 {
		baseInterval = &durationpb.Duration{Seconds: int64(remote.RetryPolicy.RetryBackOff.baseInterval.Seconds())}
	}
	if remote.RetryPolicy.RetryBackOff.maxInterval > 0 {
		maxInterval = &durationpb.Duration{Seconds: int64(remote.RetryPolicy.RetryBackOff.maxInterval.Seconds())}
	}

	if baseInterval != nil || maxInterval != nil {
		retryPolicy.RetryBackOff = &envoy_core_v3.BackoffStrategy{
			BaseInterval: baseInterval,
			MaxInterval:  maxInterval,
		}
	}

	return retryPolicy
}

func (w *wasmConfig) normalize() {
	if w.ProxyType == "" {
		w.ProxyType = api.ServiceKindConnectProxy
	}

	if w.PluginConfig.VmConfig.Runtime == "" {
		w.PluginConfig.VmConfig.Runtime = supportedRuntimes[0]
	}

	httpURI := &w.PluginConfig.VmConfig.Code.Remote.HttpURI
	httpURI.Service.Namespace = acl.NamespaceOrDefault(httpURI.Service.Namespace)
	httpURI.Service.Partition = acl.PartitionOrDefault(httpURI.Service.Partition)
	if httpURI.timeout <= 0 {
		httpURI.timeout = time.Second
	}
}

// validate ensures the filterConfig is valid or it returns an error.
// This method must be called before using the configuration.
func (w *wasmConfig) validate() error {
	var err, resultErr error
	if w.Protocol != "tcp" && w.Protocol != "http" {
		resultErr = multierror.Append(resultErr, fmt.Errorf(`unsupported Protocol %q, expected "tcp" or "http"`, w.Protocol))
	}
	if w.ProxyType != api.ServiceKindConnectProxy {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unsupported ProxyType %q, only %q is supported", w.ProxyType, api.ServiceKindConnectProxy))
	}
	if w.ListenerType != "inbound" && w.ListenerType != "outbound" {
		resultErr = multierror.Append(resultErr, fmt.Errorf(`unsupported ListenerType %q, expected "inbound" or "outbound"`, w.ListenerType))
	}
	if err = validateRuntime(w.PluginConfig.VmConfig.Runtime); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}

	httpURI := &w.PluginConfig.VmConfig.Code.Remote.HttpURI
	isLocal := w.PluginConfig.VmConfig.Code.Local.Filename != ""
	isRemote := httpURI.Service.Name != "" || httpURI.URI != ""
	if isLocal == isRemote {
		resultErr = multierror.Append(resultErr, fmt.Errorf("VmConfig.Code must provide exactly one of Local or Remote data source"))
	}

	// If the data source is Local then validation is complete.
	if isLocal {
		return resultErr
	}

	// Validate the remote data source fields.
	// Both Service and URI are required inputs for remote data sources.
	// We could catch this above in the isRemote check; however, we do an explicit check
	// here for UX to give the user extra feedback in case they only provide one or the other.
	if httpURI.Service.Name == "" || httpURI.URI == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("both Service and URI are required for Remote data sources"))
	}
	if w.PluginConfig.VmConfig.Code.Remote.SHA256 == "" {
		resultErr = multierror.Append(resultErr, fmt.Errorf("SHA256 checksum is required for Remote data sources"))
	}
	if _, err := url.Parse(httpURI.URI); err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("invalid HttpURI.URI: %w", err))
	}
	if httpURI.Timeout != "" {
		httpURI.timeout, err = time.ParseDuration(httpURI.Timeout)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to parse HttpURI.Timeout %q as a duration: %w", httpURI.Timeout, err))
		}
	}

	retryPolicy := &w.PluginConfig.VmConfig.Code.Remote.RetryPolicy
	if retryPolicy.NumRetries < 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("RetryPolicy.NumRetries must be greater than or equal to 0"))
	}

	if retryPolicy.RetryBackOff.BaseInterval != "" {
		retryPolicy.RetryBackOff.baseInterval, err = time.ParseDuration(retryPolicy.RetryBackOff.BaseInterval)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to parse RetryBackOff.BaseInterval %q: %w", retryPolicy.RetryBackOff.BaseInterval, err))
		}
	}
	if retryPolicy.RetryBackOff.MaxInterval != "" {
		retryPolicy.RetryBackOff.maxInterval, err = time.ParseDuration(retryPolicy.RetryBackOff.MaxInterval)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to parse RetryBackOff.MaxInterval %q: %w", retryPolicy.RetryBackOff.MaxInterval, err))
		}
	}

	if retryPolicy.RetryBackOff.BaseInterval != "" && retryPolicy.RetryBackOff.baseInterval <= 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("RetryBackOff.BaseInterval %q must be greater than zero and less than or equal to RetryBackOff.MaxInterval %q",
			retryPolicy.RetryBackOff.BaseInterval,
			retryPolicy.RetryBackOff.MaxInterval),
		)
	}
	if retryPolicy.RetryBackOff.MaxInterval != "" &&
		retryPolicy.RetryBackOff.maxInterval < retryPolicy.RetryBackOff.baseInterval {
		resultErr = multierror.Append(resultErr, fmt.Errorf("RetryBackOff.MaxInterval %q must be greater than or equal to RetryBackOff.BaseInterval %q",
			retryPolicy.RetryBackOff.MaxInterval,
			retryPolicy.RetryBackOff.BaseInterval),
		)
	}
	return resultErr
}

func validateRuntime(s string) error {
	for _, rt := range supportedRuntimes {
		if s == rt {
			return nil
		}
	}
	return fmt.Errorf("unsupported runtime %q", s)
}

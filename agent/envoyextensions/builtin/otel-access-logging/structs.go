// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package otelaccesslogging

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_extensions_access_loggers_grpc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	envoy_upstreams_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	LocalAccessLogClusterName = "local_access_log"

	localhost     = "localhost"
	localhostIPv4 = "127.0.0.1"
	localhostIPv6 = "::1"
)

type AccessLog struct {
	CommonConfig       *CommonConfig
	Body               interface{}
	Attributes         map[string]interface{}
	ResourceAttributes map[string]interface{}
}

func (a *AccessLog) normalize(listenerType string) error {
	if a.CommonConfig == nil {
		return fmt.Errorf("missing CommonConfig")
	}

	return a.CommonConfig.normalize(listenerType)
}

func (a *AccessLog) validate(listenerType string) error {
	if err := a.normalize(listenerType); err != nil {
		return err
	}

	return a.CommonConfig.validate(listenerType)
}

type CommonConfig struct {
	LogName                 string
	GrpcService             *GrpcService
	BufferFlushInterval     *time.Duration
	BufferSizeBytes         uint32
	FilterStateObjectsToLog []string
	RetryPolicy             *RetryPolicy
}

func (c *CommonConfig) normalize(listenerType string) error {
	if c.GrpcService != nil {
		c.GrpcService.normalize()
	} else {
		return fmt.Errorf("missing GrpcService")
	}

	if c.RetryPolicy != nil {
		c.RetryPolicy.normalize()
	}

	if c.LogName == "" {
		c.LogName = listenerType
	}

	return nil
}

func (c *CommonConfig) validate(listenerType string) error {
	if c == nil {
		return nil
	}

	c.normalize(listenerType)

	var resultErr error

	var field string
	var validate func() error
	field = "GrpcService"
	validate = c.GrpcService.validate

	if err := validate(); err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.%s: %w", field, err))
	}

	return resultErr
}

func (c CommonConfig) envoyGrpcService(cfg *cmn.RuntimeConfig) (*envoy_core_v3.GrpcService, error) {
	target := c.GrpcService.Target
	clusterName, err := c.getClusterName(cfg, target)
	if err != nil {
		return nil, err
	}

	var initialMetadata []*envoy_core_v3.HeaderValue
	for _, meta := range c.GrpcService.InitialMetadata {
		initialMetadata = append(initialMetadata, meta.toEnvoy())
	}

	return &envoy_core_v3.GrpcService{
		TargetSpecifier: &envoy_core_v3.GrpcService_EnvoyGrpc_{
			EnvoyGrpc: &envoy_core_v3.GrpcService_EnvoyGrpc{
				ClusterName: clusterName,
				Authority:   c.GrpcService.Authority,
			},
		},
		Timeout:         target.timeoutDurationPB(),
		InitialMetadata: initialMetadata,
	}, nil
}

// getClusterName returns the name of the cluster for the OpenTelemetry access logging service.
// If the extension is configured with an upstream OpenTelemetry access logging service then the name of the cluster for
// that upstream is returned. If the extension is configured with a URI, the only allowed host is `localhost`
// and the extension will insert a new cluster with the name "local_access_log", so we use that name.
func (c CommonConfig) getClusterName(cfg *cmn.RuntimeConfig, target *Target) (string, error) {
	var err error
	clusterName := LocalAccessLogClusterName
	if target.isService() {
		if clusterName, err = target.clusterName(cfg); err != nil {
			return "", err
		}
	}
	return clusterName, nil
}

func (c CommonConfig) isGRPC() bool {
	return c.GrpcService != nil
}

// toEnvoyCluster returns an Envoy cluster for connecting to the OpenTelemetry access logging service.
// If the extension is configured with the OpenTelemetry access logging service locally via the URI set to localhost,
// this func will return a new cluster definition that will allow the proxy to connect to the OpenTelemetry access logging
// service running on localhost on the configured port.
//
// If the extension is configured with the OpenTelemetry access logging service as an upstream there is no need to insert
// a new cluster so this method returns nil.
func (c *CommonConfig) toEnvoyCluster(_ *cmn.RuntimeConfig) (*envoy_cluster_v3.Cluster, error) {
	target := c.GrpcService.Target

	// If the target is an upstream we do not need to create a cluster. We will use the cluster of the upstream.
	if target.isService() {
		return nil, nil
	}

	host, port, err := target.addr()
	if err != nil {
		return nil, err
	}

	clusterType := &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC}
	if host == localhost {
		// If the host is "localhost" use a STRICT_DNS cluster type to perform DNS lookup.
		clusterType = &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STRICT_DNS}
	}

	var typedExtProtoOpts map[string]*anypb.Any
	if c.isGRPC() {
		// By default HTTP/1.1 is used for the transport protocol. gRPC requires that we explicitly configure HTTP/2
		httpProtoOpts := &envoy_upstreams_http_v3.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
				},
			},
		}
		httpProtoOptsAny, err := anypb.New(httpProtoOpts)
		if err != nil {
			return nil, err
		}
		typedExtProtoOpts = make(map[string]*anypb.Any)
		typedExtProtoOpts["envoy.extensions.upstreams.http.v3.HttpProtocolOptions"] = httpProtoOptsAny
	}

	return &envoy_cluster_v3.Cluster{
		Name:                 LocalAccessLogClusterName,
		ClusterDiscoveryType: clusterType,
		ConnectTimeout:       target.timeoutDurationPB(),
		LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: LocalAccessLogClusterName,
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{{
						HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
							Endpoint: &envoy_endpoint_v3.Endpoint{
								Address: &envoy_core_v3.Address{
									Address: &envoy_core_v3.Address_SocketAddress{
										SocketAddress: &envoy_core_v3.SocketAddress{
											Address: host,
											PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
												PortValue: uint32(port),
											},
										},
									},
								},
							},
						},
					}},
				},
			},
		},
		TypedExtensionProtocolOptions: typedExtProtoOpts,
	}, nil
}

func (c CommonConfig) toEnvoy(cfg *cmn.RuntimeConfig) (*envoy_extensions_access_loggers_grpc_v3.CommonGrpcAccessLogConfig, error) {
	config := &envoy_extensions_access_loggers_grpc_v3.CommonGrpcAccessLogConfig{
		LogName:                 c.LogName,
		BufferSizeBytes:         wrapperspb.UInt32(c.BufferSizeBytes),
		FilterStateObjectsToLog: c.FilterStateObjectsToLog,
		TransportApiVersion:     envoy_core_v3.ApiVersion_V3,
	}

	if c.BufferFlushInterval != nil {
		config.BufferFlushInterval = durationpb.New(*c.BufferFlushInterval)
	}

	if c.RetryPolicy != nil {
		config.GrpcStreamRetryPolicy = c.RetryPolicy.toEnvoy()
	}

	grpcSvc, err := c.envoyGrpcService(cfg)
	if err != nil {
		return nil, err
	}
	config.GrpcService = grpcSvc

	return config, nil
}

type GrpcService struct {
	Target          *Target
	Authority       string
	InitialMetadata []*HeaderValue
}

func (v *GrpcService) normalize() {
	if v == nil {
		return
	}
	v.Target.normalize()
}

func (v *GrpcService) validate() error {
	var resultErr error
	if v == nil {
		return resultErr
	}

	if v.Target == nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("GrpcService.Target must be set"))
	}
	if err := v.Target.validate(); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}
	return resultErr
}

type HeaderValue struct {
	Key   string
	Value string
}

func (h *HeaderValue) toEnvoy() *envoy_core_v3.HeaderValue {
	if h == nil {
		return nil
	}
	return &envoy_core_v3.HeaderValue{Key: h.Key, Value: h.Value}
}

type Target struct {
	Service api.CompoundServiceName
	URI     string
	Timeout string

	timeout *time.Duration
	host    string
	port    int
}

// addr returns the host and port for the target when the target is a URI.
// It returns a non-nil error if the target is not a URI.
func (t Target) addr() (string, int, error) {
	if !t.isURI() {
		return "", 0, fmt.Errorf("target is not configured with a URI, set Target.URI")
	}
	return t.host, t.port, nil
}

// clusterName returns the cluster name for the target when the target is an upstream service.
// It searches through the upstreams in the provided runtime configuration and returns the name
// of the cluster for the first upstream service that matches the target service.
// It returns a non-nil error if a matching cluster is not found or if the target is not an
// upstream service.
func (t Target) clusterName(cfg *cmn.RuntimeConfig) (string, error) {
	if !t.isService() {
		return "", fmt.Errorf("target is not configured with an upstream service, set Target.Service")
	}

	for service, upstream := range cfg.Upstreams {
		if service == t.Service {
			for sni := range upstream.SNIs {
				return sni, nil
			}
		}
	}
	return "", fmt.Errorf("no upstream definition found for service %q", t.Service.Name)
}

func (t Target) isService() bool {
	return t.Service.Name != ""
}

func (t Target) isURI() bool {
	return t.URI != ""
}

func (t *Target) normalize() {
	if t == nil {
		return
	}
	t.Service.Namespace = acl.NamespaceOrDefault(t.Service.Namespace)
	t.Service.Partition = acl.PartitionOrDefault(t.Service.Partition)
}

// timeoutDurationPB returns the target's timeout as a *durationpb.Duration.
// It returns nil if the timeout has not been explicitly set.
func (t *Target) timeoutDurationPB() *durationpb.Duration {
	if t == nil || t.timeout == nil {
		return nil
	}
	return durationpb.New(*t.timeout)
}

func (t *Target) validate() error {
	var err, resultErr error
	if t == nil {
		return resultErr
	}

	if t.isURI() == t.isService() {
		resultErr = multierror.Append(resultErr, fmt.Errorf("exactly one of Target.Service or Target.URI must be set"))
	}

	if t.isURI() {
		t.host, t.port, err = parseAddr(t.URI)
		if err == nil {
			switch t.host {
			case localhost, localhostIPv4, localhostIPv6:
			default:
				resultErr = multierror.Append(resultErr,
					fmt.Errorf("invalid host for Target.URI %q: expected %q, %q, or %q", t.URI, localhost, localhostIPv4, localhostIPv6))
			}
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("invalid format for Target.URI %q: expected host:port", t.URI))
		}
	}

	if t.Timeout != "" {
		if d, err := time.ParseDuration(t.Timeout); err == nil {
			t.timeout = &d
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to parse Target.Timeout %q as a duration: %w", t.Timeout, err))
		}
	}
	return resultErr
}

type RetryPolicy struct {
	RetryBackOff *RetryBackOff
	NumRetries   uint32
}

func (r *RetryPolicy) normalize() {
	if r == nil {
		return
	}
	r.RetryBackOff.normalize()
}

func (r *RetryPolicy) toEnvoy() *envoy_core_v3.RetryPolicy {
	if r == nil {
		return nil
	}

	return &envoy_core_v3.RetryPolicy{
		RetryBackOff: r.RetryBackOff.toEnvoy(),
		NumRetries:   wrapperspb.UInt32(r.NumRetries),
	}
}

type RetryBackOff struct {
	BaseInterval *time.Duration
	MaxInterval  *time.Duration
}

func (v *RetryBackOff) normalize() {
	if v == nil {
		return
	}

	if v.BaseInterval == nil {
		v.BaseInterval = new(time.Duration)
		*v.BaseInterval = time.Second
	}

	if v.MaxInterval == nil {
		v.MaxInterval = new(time.Duration)
		*v.MaxInterval = time.Second * 30
	}
}

func (r *RetryBackOff) toEnvoy() *envoy_core_v3.BackoffStrategy {
	if r == nil {
		return nil
	}

	return &envoy_core_v3.BackoffStrategy{
		BaseInterval: durationpb.New(*r.BaseInterval),
		MaxInterval:  durationpb.New(*r.MaxInterval),
	}
}

func parseAddr(s string) (host string, port int, err error) {
	// Strip the protocol if one was provided
	if _, addr, hasProto := strings.Cut(s, "://"); hasProto {
		s = addr
	}
	idx := strings.LastIndex(s, ":")
	switch idx {
	case -1, len(s) - 1:
		err = fmt.Errorf("invalid input format %q: expected host:port", s)
	case 0:
		host = localhost
		port, err = strconv.Atoi(s[idx+1:])
	default:
		host = s[:idx]
		port, err = strconv.Atoi(s[idx+1:])
	}
	return
}

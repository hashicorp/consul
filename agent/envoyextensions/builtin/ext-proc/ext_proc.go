// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ext_proc

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_ext_proc_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	envoy_upstreams_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/go-viper/mapstructure/v2"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	ext_cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

const LocalExtProcClusterName = "local_ext_proc"

// extProcFilterName is the canonical Envoy HTTP filter name and the default used
// when FilterName is not set.
const extProcFilterName = "envoy.filters.http.ext_proc"

// extProc is the top-level extension struct decoded from HCL Arguments.
type extProc struct {
	ext_cmn.BasicExtensionAdapter

	ProxyType    api.ServiceKind
	ListenerType string
	// FilterName is the unique Envoy HTTP filter name for this ext_proc instance.
	// Set a distinct value per extension to run multiple ext_proc filters on the
	// same listener. Defaults to "envoy.filters.http.ext_proc".
	FilterName    string
	InsertOptions ext_cmn.InsertOptions
	Config        extProcConfig
}

// extProcConfig configures the ext_proc filter. Mirrors the ext-authz extension:
// exactly one of GrpcService or HttpService must be set.
//
// NOTE: route-cache clearing (RouteCacheAction / the processor's clear_route_cache)
// is only honored over gRPC. The HTTP side-stream only applies request-header
// mutations and does not deliver ProcessingResponse route actions.
type extProcConfig struct {
	// Exactly one of GrpcService or HttpService must be set.
	GrpcService *GrpcService
	HttpService *HttpService

	// FailureModeAllow controls whether the request proceeds when the processor is
	// unreachable or errors. Defaults to true.
	FailureModeAllow *bool

	// RouteCacheAction controls Envoy's route cache handling: "DEFAULT", "CLEAR",
	// or "RETAIN". Only effective in gRPC mode.
	RouteCacheAction string

	// EnableRoutes restricts ext_proc to only the routes whose match path equals
	// one of these values; ext_proc is disabled on all other routes. Mutually
	// exclusive with DisableRoutes. Empty means apply to all routes.
	EnableRoutes []string

	// DisableRoutes disables ext_proc on the routes whose match path equals one of
	// these values; all other routes keep ext_proc enabled. Mutually exclusive
	// with EnableRoutes.
	DisableRoutes []string

	// parsed
	failureModeAllow bool
	routeCacheAction envoy_http_ext_proc_v3.ExternalProcessor_RouteCacheAction
}

// GrpcService configures the processor over a gRPC stream (full ProcessingResponse
// semantics, including clear_route_cache).
type GrpcService struct {
	Target    *Target
	Authority string
}

func (s *GrpcService) normalize() {
	if s != nil {
		s.Target.normalize()
	}
}

func (s *GrpcService) validate() error {
	if s == nil {
		return nil
	}
	if s.Target == nil {
		return fmt.Errorf("GrpcService.Target must be set")
	}
	return s.Target.validate()
}

// HttpService configures the processor over the HTTP side-stream (request-header
// mutation only).
type HttpService struct {
	Target *Target
	// Path is the HTTP path on the processor (e.g. "/decide"). Must start with "/".
	Path string
}

func (s *HttpService) normalize() {
	if s != nil {
		s.Target.normalize()
	}
}

func (s *HttpService) validate() error {
	var resultErr error
	if s == nil {
		return resultErr
	}
	if s.Target == nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("HttpService.Target must be set"))
	} else if err := s.Target.validate(); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}
	if s.Path != "" && !strings.HasPrefix(s.Path, "/") {
		resultErr = multierror.Append(resultErr, fmt.Errorf(`HttpService.Path must start with "/"`))
	}
	return resultErr
}

// Target identifies the processor backend: a Consul service (reuses the
// upstream/mesh cluster), a direct host:port URI (synthesizes a local TCP
// cluster), or a Unix domain socket Path (synthesizes a local loopback/UDS
// cluster). Exactly one must be set. The Path form is the loopback transport
// used by the inference gateway to reach its co-located policy processor.
type Target struct {
	Service api.CompoundServiceName
	URI     string
	// Path is an absolute filesystem path to a Unix domain socket the processor
	// listens on. When set, Envoy reaches the processor over a loopback/UDS
	// "pipe" address rather than TCP. Mutually exclusive with Service and URI.
	Path    string
	Timeout string

	timeout *time.Duration
	host    string
	port    int
}

func (t Target) isService() bool { return t.Service.Name != "" }
func (t Target) isURI() bool     { return t.URI != "" }
func (t Target) isUDS() bool     { return t.Path != "" }

func (t *Target) normalize() {
	if t == nil {
		return
	}
	t.Service.Namespace = acl.NamespaceOrDefault(t.Service.Namespace)
	t.Service.Partition = acl.PartitionOrDefault(t.Service.Partition)
}

func (t *Target) timeoutDurationPB() *durationpb.Duration {
	if t == nil || t.timeout == nil {
		return durationpb.New(5 * time.Second)
	}
	return durationpb.New(*t.timeout)
}

func (t *Target) validate() error {
	var err, resultErr error
	if t == nil {
		return resultErr
	}
	set := 0
	for _, isSet := range []bool{t.isService(), t.isURI(), t.isUDS()} {
		if isSet {
			set++
		}
	}
	if set != 1 {
		resultErr = multierror.Append(resultErr, fmt.Errorf("exactly one of Target.Service, Target.URI, or Target.Path must be set"))
	}
	if t.isURI() {
		if t.host, t.port, err = parseAddr(t.URI); err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("invalid Target.URI %q: %w", t.URI, err))
		}
	}
	if t.isUDS() && !strings.HasPrefix(t.Path, "/") {
		resultErr = multierror.Append(resultErr, fmt.Errorf("Target.Path %q must be an absolute Unix socket path", t.Path))
	}
	if t.Timeout != "" {
		if d, perr := time.ParseDuration(t.Timeout); perr == nil {
			t.timeout = &d
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("invalid Target.Timeout %q: %w", t.Timeout, perr))
		}
	}
	return resultErr
}

// clusterName returns the Envoy cluster name (PrimarySNI) for a service target.
func (t Target) clusterName(cfg *ext_cmn.RuntimeConfig) (string, error) {
	if !t.isService() {
		return "", fmt.Errorf("target is not configured with an upstream service, set Target.Service")
	}
	for service, upstream := range cfg.Upstreams {
		if service == t.Service {
			if upstream.PrimarySNI == "" {
				return "", fmt.Errorf("no upstream SNI found for service %q", t.Service.Name)
			}
			return upstream.PrimarySNI, nil
		}
	}
	return "", fmt.Errorf("no upstream definition found for service %q", t.Service.Name)
}

var _ ext_cmn.BasicExtension = (*extProc)(nil)

func Constructor(ext api.EnvoyExtension) (ext_cmn.EnvoyExtender, error) {
	p, err := newExtProc(ext)
	if err != nil {
		return nil, err
	}
	return &ext_cmn.BasicEnvoyExtender{Extension: p}, nil
}

func newExtProc(ext api.EnvoyExtension) (*extProc, error) {
	p := &extProc{}
	if ext.Name != api.BuiltinExtProcExtension {
		return p, fmt.Errorf("expected extension name %q but got %q", api.BuiltinExtProcExtension, ext.Name)
	}
	if err := p.fromArguments(ext.Arguments); err != nil {
		return p, err
	}
	return p, nil
}

func (p *extProc) fromArguments(args map[string]any) error {

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           p,
	})
	if err != nil {
		return err
	}
	if err := decoder.Decode(args); err != nil {
		return err
	}
	p.normalize()
	return p.validate()
}

func (p *extProc) normalize() {
	if p.ProxyType == "" {
		p.ProxyType = api.ServiceKindAPIGateway
	}
	if p.ListenerType == "" {
		p.ListenerType = "inbound"
	}
	if p.FilterName == "" {
		p.FilterName = extProcFilterName
	}
	p.Config.normalize()
}

func (c *extProcConfig) normalize() {
	if c.isGRPC() {
		c.GrpcService.normalize()
	}
	if c.isHTTP() {
		c.HttpService.normalize()
	}
}

func (c *extProcConfig) isGRPC() bool { return c.GrpcService != nil }
func (c *extProcConfig) isHTTP() bool { return c.HttpService != nil }

func (c *extProcConfig) target() *Target {
	switch {
	case c.isGRPC():
		return c.GrpcService.Target
	case c.isHTTP():
		return c.HttpService.Target
	default:
		return nil
	}
}

func (p *extProc) validate() error {
	var resultErr error
	switch p.ProxyType {
	case api.ServiceKindAPIGateway, api.ServiceKindConnectProxy, api.ServiceKindInferenceGateway:
	default:
		resultErr = multierror.Append(resultErr, fmt.Errorf(
			"unsupported ProxyType %q, only %q, %q, or %q is supported",
			p.ProxyType, api.ServiceKindAPIGateway, api.ServiceKindConnectProxy, api.ServiceKindInferenceGateway))
	}
	if p.ListenerType != "inbound" && p.ListenerType != "outbound" {
		resultErr = multierror.Append(resultErr, fmt.Errorf(
			`unexpected ListenerType %q, supported values are "inbound" or "outbound"`, p.ListenerType))
	}
	if err := p.Config.validate(); err != nil {
		resultErr = multierror.Append(resultErr, err)
	}
	return resultErr
}

func (c *extProcConfig) validate() error {
	var resultErr error

	if c.isGRPC() == c.isHTTP() {
		return multierror.Append(resultErr, fmt.Errorf("exactly one of Config.GrpcService or Config.HttpService must be set"))
	}

	if c.isHTTP() {
		if err := c.HttpService.validate(); err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.HttpService: %w", err))
		}
	} else {
		if err := c.GrpcService.validate(); err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.GrpcService: %w", err))
		}
	}

	c.failureModeAllow = false
	if c.FailureModeAllow != nil {
		c.failureModeAllow = *c.FailureModeAllow
	}

	switch strings.ToUpper(c.RouteCacheAction) {
	case "", "DEFAULT":
		c.routeCacheAction = envoy_http_ext_proc_v3.ExternalProcessor_DEFAULT
	case "CLEAR":
		c.routeCacheAction = envoy_http_ext_proc_v3.ExternalProcessor_CLEAR
	case "RETAIN":
		c.routeCacheAction = envoy_http_ext_proc_v3.ExternalProcessor_RETAIN
	default:
		resultErr = multierror.Append(resultErr, fmt.Errorf(
			`invalid Config.RouteCacheAction %q, expected "DEFAULT", "CLEAR", or "RETAIN"`, c.RouteCacheAction))
	}

	if len(c.EnableRoutes) > 0 && len(c.DisableRoutes) > 0 {
		resultErr = multierror.Append(resultErr, fmt.Errorf(
			"only one of Config.EnableRoutes or Config.DisableRoutes may be set"))
	}

	return resultErr
}

func (c *extProcConfig) getClusterName(cfg *ext_cmn.RuntimeConfig) (string, error) {
	t := c.target()
	if t == nil {
		return "", fmt.Errorf("no Config.GrpcService or Config.HttpService target configured")
	}
	if t.isService() {
		return t.clusterName(cfg)
	}
	return LocalExtProcClusterName, nil
}

func (c *extProcConfig) envoyGrpcService(cfg *ext_cmn.RuntimeConfig) (*envoy_core_v3.GrpcService, error) {
	clusterName, err := c.getClusterName(cfg)
	if err != nil {
		return nil, err
	}
	return &envoy_core_v3.GrpcService{
		TargetSpecifier: &envoy_core_v3.GrpcService_EnvoyGrpc_{
			EnvoyGrpc: &envoy_core_v3.GrpcService_EnvoyGrpc{
				ClusterName: clusterName,
				Authority:   c.GrpcService.Authority,
			},
		},
		Timeout: c.GrpcService.Target.timeoutDurationPB(),
	}, nil
}

func (c *extProcConfig) envoyHttpService(cfg *ext_cmn.RuntimeConfig) (*envoy_http_ext_proc_v3.ExtProcHttpService, error) {
	clusterName, err := c.getClusterName(cfg)
	if err != nil {
		return nil, err
	}
	path := c.HttpService.Path
	if path == "" {
		path = "/"
	}
	var uri string
	if c.HttpService.Target.isURI() {
		uri = fmt.Sprintf("http://%s:%d%s", c.HttpService.Target.host, c.HttpService.Target.port, path)
	} else {
		uri = fmt.Sprintf("http://%s%s", clusterName, path)
	}
	return &envoy_http_ext_proc_v3.ExtProcHttpService{
		HttpService: &envoy_core_v3.HttpService{
			HttpUri: &envoy_core_v3.HttpUri{
				Uri:              uri,
				HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{Cluster: clusterName},
				Timeout:          c.HttpService.Target.timeoutDurationPB(),
			},
		},
	}, nil
}

// toEnvoyCluster builds a local cluster for URI and UDS (Path) targets. Service
// targets reuse the existing upstream/mesh cluster (returns nil). gRPC clusters
// use HTTP/2. UDS targets resolve to a STATIC cluster with a "pipe" address.
func (c *extProcConfig) toEnvoyCluster(_ *ext_cmn.RuntimeConfig) (*envoy_cluster_v3.Cluster, error) {
	t := c.target()
	if t == nil || t.isService() {
		return nil, nil
	}

	// Resolve the endpoint address and cluster discovery type. UDS targets use a
	// STATIC cluster pointing at a pipe address; URI targets use STATIC for IPs
	// and STRICT_DNS for hostnames.
	var endpointAddr *envoy_core_v3.Address
	clusterType := &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC}
	isDNS := false
	if t.isUDS() {
		endpointAddr = &envoy_core_v3.Address{
			Address: &envoy_core_v3.Address_Pipe{
				Pipe: &envoy_core_v3.Pipe{Path: t.Path},
			},
		}
	} else {
		host, port := t.host, t.port
		isDNS = net.ParseIP(host) == nil
		if isDNS {
			clusterType = &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STRICT_DNS}
		}
		endpointAddr = &envoy_core_v3.Address{
			Address: &envoy_core_v3.Address_SocketAddress{
				SocketAddress: &envoy_core_v3.SocketAddress{
					Address: host,
					PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{
						PortValue: uint32(port),
					},
				},
			},
		}
	}

	var httpProtoOpts *envoy_upstreams_http_v3.HttpProtocolOptions
	if c.isGRPC() {
		// gRPC requires HTTP/2.
		httpProtoOpts = &envoy_upstreams_http_v3.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
						Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
					},
				},
			},
		}
	} else {
		httpProtoOpts = &envoy_upstreams_http_v3.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoy_upstreams_http_v3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{
						HttpProtocolOptions: &envoy_core_v3.Http1ProtocolOptions{},
					},
				},
			},
		}
	}
	httpProtoOptsAny, err := anypb.New(httpProtoOpts)
	if err != nil {
		return nil, err
	}

	cluster := &envoy_cluster_v3.Cluster{
		Name:                 LocalExtProcClusterName,
		ClusterDiscoveryType: clusterType,
		ConnectTimeout:       t.timeoutDurationPB(),
		LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: LocalExtProcClusterName,
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
				LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{{
					HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
						Endpoint: &envoy_endpoint_v3.Endpoint{
							Address: endpointAddr,
						},
					},
				}},
			}},
		},
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpProtoOptsAny,
		},
	}
	if isDNS {
		cluster.DnsLookupFamily = envoy_cluster_v3.Cluster_V4_ONLY
	}
	return cluster, nil
}

func (p *extProc) CanApply(cfg *ext_cmn.RuntimeConfig) bool {
	return cfg.Kind == p.ProxyType
}

func (p *extProc) matchesListenerDirection(isInboundListener bool) bool {
	// Gateways own a single listener, so the filter always applies there.
	if p.ProxyType == api.ServiceKindAPIGateway || p.ProxyType == api.ServiceKindInferenceGateway {
		return true
	}
	return (!isInboundListener && p.ListenerType == "outbound") || (isInboundListener && p.ListenerType == "inbound")
}

func (p *extProc) configureInsertOptions() {
	if p.InsertOptions.Location != "" {
		return
	}
	p.InsertOptions.Location = ext_cmn.InsertBeforeFirstMatch
	p.InsertOptions.FilterName = "envoy.filters.http.router"
}

func (p *extProc) PatchClusters(cfg *ext_cmn.RuntimeConfig, c ext_cmn.ClusterMap) (ext_cmn.ClusterMap, error) {
	clusterName, err := p.Config.getClusterName(cfg)
	if err != nil {
		return c, err
	}
	if _, exists := c[clusterName]; exists {
		return c, nil
	}
	if p.Config.target().isService() {
		return c, nil
	}
	cluster, err := p.Config.toEnvoyCluster(cfg)
	if err != nil {
		return c, err
	}
	if cluster != nil {
		c[cluster.Name] = cluster
	}
	return c, nil
}

func (p *extProc) PatchFilters(cfg *ext_cmn.RuntimeConfig, filters []*envoy_listener_v3.Filter, isInboundListener bool) ([]*envoy_listener_v3.Filter, error) {
	if !p.matchesListenerDirection(isInboundListener) {
		return filters, nil
	}
	switch cfg.Protocol {
	case "http", "http2", "grpc":
	default:
		return filters, nil
	}
	p.configureInsertOptions()

	filterCfg := &envoy_http_ext_proc_v3.ExternalProcessor{
		// Header-only request processing. Valid for the HTTP side-stream and
		// sufficient for header-based routing decisions over gRPC.
		ProcessingMode: &envoy_http_ext_proc_v3.ProcessingMode{
			RequestHeaderMode:   envoy_http_ext_proc_v3.ProcessingMode_SEND,
			ResponseHeaderMode:  envoy_http_ext_proc_v3.ProcessingMode_SKIP,
			RequestBodyMode:     envoy_http_ext_proc_v3.ProcessingMode_NONE,
			ResponseBodyMode:    envoy_http_ext_proc_v3.ProcessingMode_NONE,
			RequestTrailerMode:  envoy_http_ext_proc_v3.ProcessingMode_SKIP,
			ResponseTrailerMode: envoy_http_ext_proc_v3.ProcessingMode_SKIP,
		},
		FailureModeAllow: p.Config.failureModeAllow,
		RouteCacheAction: p.Config.routeCacheAction,
	}

	// Disambiguate stats when multiple ext_proc filters share a listener.
	if p.FilterName != extProcFilterName {
		filterCfg.StatPrefix = p.FilterName
	}

	if p.Config.isGRPC() {
		grpcSvc, err := p.Config.envoyGrpcService(cfg)
		if err != nil {
			return filters, err
		}
		filterCfg.GrpcService = grpcSvc
	} else {
		httpSvc, err := p.Config.envoyHttpService(cfg)
		if err != nil {
			return filters, err
		}
		filterCfg.HttpService = httpSvc
	}

	httpFilter, err := ext_cmn.MakeEnvoyHTTPFilter(p.FilterName, filterCfg)
	if err != nil {
		return filters, err
	}
	return ext_cmn.InsertHTTPFilter(filters, httpFilter, p.InsertOptions)
}

// PatchRoutes scopes ext_proc to specific routes via per-route overrides:
//   - The processor's own HTTP path (HTTP mode) is always disabled to avoid recursion.
//   - If Config.EnableRoutes is set, ext_proc is disabled on every route that does
//     not match one of those paths (allowlist).
//   - If Config.DisableRoutes is set, ext_proc is disabled on every route that
//     matches one of those paths (denylist).
//
// EnableRoutes and DisableRoutes are mutually exclusive (enforced in validate).
func (p *extProc) PatchRoutes(_ *ext_cmn.RuntimeConfig, routes ext_cmn.RouteMap) (ext_cmn.RouteMap, error) {
	bypassPath := ""
	if p.Config.isHTTP() {
		bypassPath = p.Config.HttpService.Path
	}
	enable := p.Config.EnableRoutes
	disable := p.Config.DisableRoutes

	if bypassPath == "" && len(enable) == 0 && len(disable) == 0 {
		return routes, nil
	}

	disablePerRouteAny, err := anypb.New(&envoy_http_ext_proc_v3.ExtProcPerRoute{
		Override: &envoy_http_ext_proc_v3.ExtProcPerRoute_Disabled{Disabled: true},
	})
	if err != nil {
		return routes, err
	}
	for _, rc := range routes {
		for _, vh := range rc.GetVirtualHosts() {
			for _, r := range vh.GetRoutes() {
				match := r.GetMatch()
				var shouldDisable bool
				switch {
				case bypassPath != "" && routeMatchTargetsBypassPath(match, bypassPath):
					shouldDisable = true
				case len(enable) > 0:
					shouldDisable = !routeMatchesAny(match, enable)
				case len(disable) > 0:
					shouldDisable = routeMatchesAny(match, disable)
				}
				if !shouldDisable {
					continue
				}
				if r.TypedPerFilterConfig == nil {
					r.TypedPerFilterConfig = make(map[string]*anypb.Any)
				}
				r.TypedPerFilterConfig[p.FilterName] = disablePerRouteAny
			}
		}
	}
	return routes, nil
}

// routeMatchesAny reports whether the route match targets any of the given paths.
func routeMatchesAny(match *envoy_route_v3.RouteMatch, routes []string) bool {
	for _, route := range routes {
		if routeMatchTargetsBypassPath(match, route) {
			return true
		}
	}
	return false
}

func routeMatchTargetsBypassPath(match *envoy_route_v3.RouteMatch, route string) bool {
	if match == nil || route == "" {
		return false
	}
	switch p := match.PathSpecifier.(type) {
	case *envoy_route_v3.RouteMatch_Path:
		return p.Path == route
	case *envoy_route_v3.RouteMatch_Prefix:
		return p.Prefix == route
	case *envoy_route_v3.RouteMatch_PathSeparatedPrefix:
		return p.PathSeparatedPrefix == route
	default:
		return false
	}
}

func parseAddr(s string) (host string, port int, err error) {
	if _, addr, hasProto := strings.Cut(s, "://"); hasProto {
		s = addr
	}
	idx := strings.LastIndex(s, ":")
	switch idx {
	case -1, len(s) - 1:
		return "", 0, fmt.Errorf("invalid input format %q: expected host:port", s)
	case 0:
		return "", 0, fmt.Errorf("invalid input format %q: host is required", s)
	default:
		host = s[:idx]
	}
	port, err = strconv.Atoi(s[idx+1:])
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port %q in %q", s[idx+1:], s)
	}
	return host, port, nil
}

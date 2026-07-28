// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package extauthz

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
	envoy_http_ext_authz_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	envoy_ext_authz_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/ext_authz/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_upstreams_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_type_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

const (
	LocalExtAuthzClusterName = "local_ext_authz"

	defaultMetadataNS    = "consul"
	defaultStatPrefix    = "response"
	defaultStatusOnError = 403
	localhost            = "localhost"
	localhostIPv4        = "127.0.0.1"
	localhostIPv6        = "::1"
)

type extAuthzConfig struct {
	BootstrapMetadataLabelsKey string
	ClearRouteCache            *bool
	GrpcService                *GrpcService
	HttpService                *HttpService
	IncludePeerCertificate     *bool
	MetadataContextNamespaces  []string
	StatusOnError              *int
	StatPrefix                 string
	WithRequestBody            *BufferSettings

	// FailureModeAllow, when set, explicitly controls the ext_authz filter's
	// failure-mode behavior and overrides the default that is otherwise derived
	// from the extension's Required flag (failureModeAllow = !Required).
	//   - true  -> requests are ALLOWED when the authorization service is
	//              unreachable or returns an error (fail open).
	//   - false -> such requests are DENIED (fail closed).
	// When nil (omitted) the Required-derived default is used.
	FailureModeAllow *bool

	// AllowedHeaders restricts which client request headers are forwarded to the
	// external authorization service in the check request. It maps to the
	// filter-level Envoy field ext_authz.allowed_headers and therefore applies to
	// BOTH the gRPC and HTTP transports.
	//
	// When unset, Envoy keeps its default behavior: ALL client request headers are
	// sent to a gRPC authorization server, whereas only a small default set is sent
	// to an HTTP authorization server (plus any HttpService.AuthorizationRequest
	// .AllowedHeaders). This field is the only way to restrict the request headers
	// sent to a gRPC authorization server.
	AllowedHeaders ListStringMatcher
	// DisallowedHeaders explicitly prevents the listed client request headers from
	// being forwarded to the external authorization service. It maps to the
	// filter-level Envoy field ext_authz.disallowed_headers and takes precedence
	// over AllowedHeaders when a header matches both.
	DisallowedHeaders ListStringMatcher

	// proxyKind is set by extAuthz.normalize() and controls validation rules that
	// differ between proxy kinds (e.g. URI host restrictions).
	proxyKind api.ServiceKind

	failureModeAllow bool
}

func (c *extAuthzConfig) normalize() {
	if c.StatPrefix == "" {
		c.StatPrefix = defaultStatPrefix
	}
	if c.isGRPC() {
		c.GrpcService.normalize()
	}
	if c.isHTTP() {
		c.HttpService.normalize()
	}
}

func (c *extAuthzConfig) validate() error {
	c.normalize()

	var resultErr error
	if c.isGRPC() == c.isHTTP() {
		resultErr = multierror.Append(resultErr, fmt.Errorf("exactly one of GrpcService or HttpService must be set"))
	}

	var field string
	var validate func() error
	if c.isHTTP() {
		field = "HttpService"
		validate = c.HttpService.validate
	} else {
		field = "GrpcService"
		validate = c.GrpcService.validate
	}

	if err := validate(); err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.%s: %w", field, err))
	}

	// For connect-proxy sidecars the ext_authz backend must be on the local host
	// because the sidecar and the authz service share the same network namespace.
	// For API Gateway proxies the backend can be any network-reachable host (e.g. a
	// separate container), so the localhost restriction is not applied.
	if c.proxyKind != api.ServiceKindAPIGateway {
		var target *Target
		if c.isHTTP() && c.HttpService != nil {
			target = c.HttpService.Target
		} else if c.isGRPC() && c.GrpcService != nil {
			target = c.GrpcService.Target
		}
		if target != nil && target.isURI() && target.host != "" {
			switch target.host {
			case localhost, localhostIPv4, localhostIPv6:
			default:
				resultErr = multierror.Append(resultErr,
					fmt.Errorf("invalid host for Target.URI %q: expected %q, %q, or %q (use ProxyType %q to allow remote hosts)",
						target.URI, localhost, localhostIPv4, localhostIPv6, api.ServiceKindAPIGateway))
			}
		}
	}

	if c.StatusOnError != nil {
		if _, ok := envoy_type_v3.StatusCode_name[int32(*c.StatusOnError)]; !ok {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.StatusOnError:"+
				"status code %d is not supported by Envoy, please refer to the Envoy documentation for supported status codes",
				*c.StatusOnError))
		}
	}

	if err := c.AllowedHeaders.validate(); err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.AllowedHeaders: %w", err))
	}
	if err := c.DisallowedHeaders.validate(); err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("failed to validate Config.DisallowedHeaders: %w", err))
	}

	return resultErr
}

func (c extAuthzConfig) envoyGrpcService(cfg *cmn.RuntimeConfig) (*envoy_core_v3.GrpcService, error) {
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
		Timeout:         c.GrpcService.timeoutDurationPB(),
		InitialMetadata: initialMetadata,
	}, nil
}

func (c extAuthzConfig) envoyHttpService(cfg *cmn.RuntimeConfig) (*envoy_http_ext_authz_v3.HttpService, error) {
	clusterName, err := c.getClusterName(cfg, c.HttpService.Target)
	if err != nil {
		return nil, err
	}

	return &envoy_http_ext_authz_v3.HttpService{
		ServerUri: &envoy_core_v3.HttpUri{
			Uri:              clusterName, // not used by Envoy, set to cluster
			HttpUpstreamType: &envoy_core_v3.HttpUri_Cluster{Cluster: clusterName},
			Timeout:          c.HttpService.timeoutDurationPB(),
		},
		PathPrefix:            c.HttpService.PathPrefix,
		AuthorizationRequest:  c.HttpService.AuthorizationRequest.toEnvoy(),
		AuthorizationResponse: c.HttpService.AuthorizationResponse.toEnvoy(),
	}, nil
}

// getClusterName returns the name of the cluster for the external authorization service.
// If the extension is configured with an upstream ext-authz service then the name of the cluster for
// that upstream is returned. If the extension is configured with a URI, the only allowed host is `localhost`
// and the extension will insert a new cluster with the name "local_ext_authz", so we use that name.
func (c extAuthzConfig) getClusterName(cfg *cmn.RuntimeConfig, target *Target) (string, error) {
	var err error
	clusterName := LocalExtAuthzClusterName
	if target.isService() {
		if clusterName, err = target.clusterName(cfg); err != nil {
			return "", err
		}
	}
	return clusterName, nil
}

func (c extAuthzConfig) isGRPC() bool {
	return c.GrpcService != nil
}

func (c extAuthzConfig) isHTTP() bool {
	return c.HttpService != nil
}

// toEnvoyCluster returns an Envoy cluster for connecting to the ext_authz service.
// If the extension is configured with the ext_authz service locally via the URI set to localhost,
// this func will return a new cluster definition that will allow the proxy to connect to the ext_authz
// service running on localhost on the configured port.
//
// If the extension is configured with the ext_authz service as an upstream there is no need to insert
// a new cluster so this method returns nil.
func (c *extAuthzConfig) toEnvoyCluster(_ *cmn.RuntimeConfig) (*envoy_cluster_v3.Cluster, error) {
	var target *Target
	var connectTimeout *durationpb.Duration
	if c.isHTTP() {
		target = c.HttpService.Target
		connectTimeout = c.HttpService.timeoutDurationPB()
	} else {
		target = c.GrpcService.Target
		connectTimeout = c.GrpcService.timeoutDurationPB()
	}

	// If the target is an upstream we do not need to create a cluster. We will use the cluster of the upstream.
	if target.isService() {
		return nil, nil
	}

	host, port, err := target.addr()
	if err != nil {
		return nil, err
	}

	clusterType := &envoy_cluster_v3.Cluster_Type{Type: envoy_cluster_v3.Cluster_STATIC}
	if net.ParseIP(host) == nil {
		// host is a DNS name (including "localhost" and container names like "oauth2-proxy").
		// Use STRICT_DNS so Envoy resolves it at runtime instead of treating it as a
		// literal IP. STATIC is used only for bare IP addresses.
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
		Name:                 LocalExtAuthzClusterName,
		ClusterDiscoveryType: clusterType,
		ConnectTimeout:       connectTimeout,
		LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: LocalExtAuthzClusterName,
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

func (c extAuthzConfig) toEnvoyHttpFilter(cfg *cmn.RuntimeConfig) (*envoy_http_v3.HttpFilter, error) {
	extAuthzFilter := &envoy_http_ext_authz_v3.ExtAuthz{
		StatPrefix:                 c.StatPrefix,
		WithRequestBody:            c.WithRequestBody.toEnvoy(),
		TransportApiVersion:        envoy_core_v3.ApiVersion_V3,
		MetadataContextNamespaces:  append(c.MetadataContextNamespaces, defaultMetadataNS),
		FailureModeAllow:           c.failureModeAllow,
		BootstrapMetadataLabelsKey: c.BootstrapMetadataLabelsKey,
	}
	if c.isHTTP() {
		httpSvc, err := c.envoyHttpService(cfg)
		if err != nil {
			return nil, err
		}
		extAuthzFilter.Services = &envoy_http_ext_authz_v3.ExtAuthz_HttpService{HttpService: httpSvc}
	} else {
		grpcSvc, err := c.envoyGrpcService(cfg)
		if err != nil {
			return nil, err
		}
		extAuthzFilter.Services = &envoy_http_ext_authz_v3.ExtAuthz_GrpcService{GrpcService: grpcSvc}
	}

	if c.ClearRouteCache != nil {
		extAuthzFilter.ClearRouteCache = *c.ClearRouteCache
	}
	if c.IncludePeerCertificate != nil {
		extAuthzFilter.IncludePeerCertificate = *c.IncludePeerCertificate
	}
	if c.StatusOnError != nil {
		extAuthzFilter.StatusOnError = &envoy_type_v3.HttpStatus{
			Code: envoy_type_v3.StatusCode(*c.StatusOnError),
		}
	}

	// AllowedHeaders / DisallowedHeaders are filter-level controls that apply to
	// both the gRPC and HTTP transports. toEnvoy() returns nil for an empty list,
	// preserving Envoy's default header-forwarding behavior when unset.
	if ah := c.AllowedHeaders.toEnvoy(); ah != nil {
		extAuthzFilter.AllowedHeaders = ah
	}
	if dh := c.DisallowedHeaders.toEnvoy(); dh != nil {
		extAuthzFilter.DisallowedHeaders = dh
	}

	return cmn.MakeEnvoyHTTPFilter("envoy.filters.http.ext_authz", extAuthzFilter)
}

func (c extAuthzConfig) toEnvoyNetworkFilter(cfg *cmn.RuntimeConfig) (*envoy_listener_v3.Filter, error) {
	grpcSvc, err := c.envoyGrpcService(cfg)
	if err != nil {
		return nil, err
	}

	extAuthzFilter := &envoy_ext_authz_v3.ExtAuthz{
		GrpcService:         grpcSvc,
		StatPrefix:          c.StatPrefix,
		TransportApiVersion: envoy_core_v3.ApiVersion_V3,
		FailureModeAllow:    c.failureModeAllow,
	}

	if c.IncludePeerCertificate != nil {
		extAuthzFilter.IncludePeerCertificate = *c.IncludePeerCertificate
	}

	return cmn.MakeFilter("envoy.filters.network.ext_authz", extAuthzFilter)
}

type validator interface {
	validate() error
}

type AuthorizationRequest struct {
	AllowedHeaders ListStringMatcher
	HeadersToAdd   []*HeaderValue
}

func (r *AuthorizationRequest) toEnvoy() *envoy_http_ext_authz_v3.AuthorizationRequest {
	if r == nil {
		return nil
	}
	if len(r.AllowedHeaders) == 0 && len(r.HeadersToAdd) == 0 {
		return nil
	}

	req := &envoy_http_ext_authz_v3.AuthorizationRequest{
		AllowedHeaders: r.AllowedHeaders.toEnvoy(),
	}
	for _, header := range r.HeadersToAdd {
		req.HeadersToAdd = append(req.HeadersToAdd, header.toEnvoy())
	}

	return req
}

func (r *AuthorizationRequest) validate() error {
	var resultErr error
	if r == nil {
		return resultErr
	}
	if err := r.AllowedHeaders.validate(); err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("validation failed for AuthorizationRequest.AllowedHeaders: %w", err))
	}
	return resultErr
}

type AuthorizationResponse struct {
	AllowedUpstreamHeaders         ListStringMatcher
	AllowedUpstreamHeadersToAppend ListStringMatcher
	AllowedClientHeaders           ListStringMatcher
	AllowedClientHeadersOnSuccess  ListStringMatcher
	DynamicMetadataFromHeaders     ListStringMatcher
}

func (r *AuthorizationResponse) toEnvoy() *envoy_http_ext_authz_v3.AuthorizationResponse {
	if r == nil {
		return nil
	}

	return &envoy_http_ext_authz_v3.AuthorizationResponse{
		AllowedUpstreamHeaders:         r.AllowedUpstreamHeaders.toEnvoy(),
		AllowedUpstreamHeadersToAppend: r.AllowedUpstreamHeadersToAppend.toEnvoy(),
		AllowedClientHeaders:           r.AllowedClientHeaders.toEnvoy(),
		AllowedClientHeadersOnSuccess:  r.AllowedClientHeadersOnSuccess.toEnvoy(),
		DynamicMetadataFromHeaders:     r.DynamicMetadataFromHeaders.toEnvoy(),
	}
}

func (r *AuthorizationResponse) validate() error {
	var resultErr error
	if r == nil {
		return resultErr
	}
	for field, matchers := range r.fieldMap() {
		if err := matchers.validate(); err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("validation failed for AuthorizationResponse.%s: %w", field, err))
		}
	}
	return resultErr
}

func (r *AuthorizationResponse) fieldMap() map[string]ListStringMatcher {
	if r == nil {
		return nil
	}
	return map[string]ListStringMatcher{
		"AllowedUpstreamHeaders":         r.AllowedUpstreamHeaders,
		"AllowedUpstreamHeadersToAppend": r.AllowedUpstreamHeadersToAppend,
		"AllowedClientHeaders":           r.AllowedClientHeaders,
		"AllowedClientHeadersOnSuccess":  r.AllowedClientHeadersOnSuccess,
		"DynamicMetadataFromHeaders":     r.DynamicMetadataFromHeaders,
	}
}

type BufferSettings struct {
	MaxRequestBytes     *int64
	AllowPartialMessage *bool
	PackAsBytes         *bool
}

func (b *BufferSettings) toEnvoy() *envoy_http_ext_authz_v3.BufferSettings {
	if b == nil {
		return nil
	}
	if b.AllowPartialMessage == nil &&
		b.MaxRequestBytes == nil &&
		b.PackAsBytes == nil {
		return nil
	}

	bufSet := &envoy_http_ext_authz_v3.BufferSettings{}
	if b.AllowPartialMessage != nil {
		bufSet.AllowPartialMessage = *b.AllowPartialMessage
	}
	if b.MaxRequestBytes != nil {
		bufSet.MaxRequestBytes = uint32(*b.MaxRequestBytes)
	}
	if b.PackAsBytes != nil {
		bufSet.PackAsBytes = *b.PackAsBytes
	}

	return bufSet
}

type GrpcService struct {
	Target          *Target
	Authority       string
	InitialMetadata []*HeaderValue
	// Timeout sets the per-request timeout for calls to the gRPC authorization
	// service, expressed as a Go duration string (e.g. "2s"). When set it takes
	// precedence over Target.Timeout; when empty the Target.Timeout is used.
	Timeout string

	timeout *time.Duration
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
	if v.Timeout != "" {
		if d, err := time.ParseDuration(v.Timeout); err == nil {
			v.timeout = &d
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to parse GrpcService.Timeout %q as a duration: %w", v.Timeout, err))
		}
	}
	return resultErr
}

// timeoutDurationPB returns the effective per-request timeout as a
// *durationpb.Duration. The service-level Timeout takes precedence over
// Target.Timeout; nil is returned when neither is set.
func (v *GrpcService) timeoutDurationPB() *durationpb.Duration {
	if v == nil {
		return nil
	}
	if v.timeout != nil {
		return durationpb.New(*v.timeout)
	}
	return v.Target.timeoutDurationPB()
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

type HttpService struct {
	Target                *Target
	PathPrefix            string
	AuthorizationRequest  *AuthorizationRequest
	AuthorizationResponse *AuthorizationResponse
	// Timeout sets the per-request timeout for calls to the HTTP authorization
	// service, expressed as a Go duration string (e.g. "2s"). When set it takes
	// precedence over Target.Timeout; when empty the Target.Timeout is used.
	Timeout string

	timeout *time.Duration
}

func (v *HttpService) normalize() {
	if v == nil {
		return
	}
	v.Target.normalize()
}

func (v *HttpService) validate() error {
	var resultErr error
	if v == nil {
		return resultErr
	}

	if v.Target == nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("HttpService.Target must be set"))
	}
	for _, val := range []validator{v.Target, v.AuthorizationRequest, v.AuthorizationResponse} {
		if err := val.validate(); err != nil {
			resultErr = multierror.Append(resultErr, err)
		}
	}
	if v.Timeout != "" {
		if d, err := time.ParseDuration(v.Timeout); err == nil {
			v.timeout = &d
		} else {
			resultErr = multierror.Append(resultErr, fmt.Errorf("failed to parse HttpService.Timeout %q as a duration: %w", v.Timeout, err))
		}
	}
	return resultErr
}

// timeoutDurationPB returns the effective per-request timeout as a
// *durationpb.Duration. The service-level Timeout takes precedence over
// Target.Timeout; nil is returned when neither is set.
func (v *HttpService) timeoutDurationPB() *durationpb.Duration {
	if v == nil {
		return nil
	}
	if v.timeout != nil {
		return durationpb.New(*v.timeout)
	}
	return v.Target.timeoutDurationPB()
}

type ListStringMatcher []*StringMatcher

func (l ListStringMatcher) toEnvoy() *envoy_type_matcher_v3.ListStringMatcher {
	if len(l) < 1 {
		return nil
	}
	matchers := &envoy_type_matcher_v3.ListStringMatcher{}
	for _, matcher := range l {
		matchers.Patterns = append(matchers.Patterns, matcher.toEnvoy())
	}
	return matchers
}

func (l ListStringMatcher) validate() error {
	var resultErr error
	if len(l) < 1 {
		return nil
	}
	for idx, matcher := range l {
		if err := matcher.validate(); err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("validation failed for matcher at index %d: %w", idx, err))
		}
	}
	return resultErr
}

type StringMatcher struct {
	Contains   string
	Exact      string
	IgnoreCase bool
	Prefix     string
	SafeRegex  string
	Suffix     string
}

func (s *StringMatcher) toEnvoy() *envoy_type_matcher_v3.StringMatcher {
	if s == nil {
		return nil
	}
	switch {
	case s.Contains != "":
		return &envoy_type_matcher_v3.StringMatcher{
			MatchPattern: &envoy_type_matcher_v3.StringMatcher_Contains{Contains: s.Contains},
			IgnoreCase:   s.IgnoreCase,
		}
	case s.Exact != "":
		return &envoy_type_matcher_v3.StringMatcher{
			MatchPattern: &envoy_type_matcher_v3.StringMatcher_Exact{Exact: s.Exact},
			IgnoreCase:   s.IgnoreCase,
		}
	case s.Prefix != "":
		return &envoy_type_matcher_v3.StringMatcher{
			MatchPattern: &envoy_type_matcher_v3.StringMatcher_Prefix{Prefix: s.Prefix},
			IgnoreCase:   s.IgnoreCase,
		}
	case s.SafeRegex != "":
		return &envoy_type_matcher_v3.StringMatcher{
			MatchPattern: &envoy_type_matcher_v3.StringMatcher_SafeRegex{
				SafeRegex: &envoy_type_matcher_v3.RegexMatcher{
					Regex: s.SafeRegex,
				},
			},
		}
	case s.Suffix != "":
		return &envoy_type_matcher_v3.StringMatcher{
			MatchPattern: &envoy_type_matcher_v3.StringMatcher_Suffix{Suffix: s.Suffix},
			IgnoreCase:   s.IgnoreCase,
		}
	default:
		return nil
	}
}

func (s *StringMatcher) validate() error {
	if s == nil {
		return nil
	}

	set := 0
	for _, s := range []string{s.Contains, s.Exact, s.Prefix, s.SafeRegex, s.Suffix} {
		if s != "" {
			set++
		}
	}
	if set != 1 {
		return fmt.Errorf("exactly one of Contains, Exact, Prefix, SafeRegex or Suffix must be set")
	}
	return nil
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
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("invalid format for Target.URI %q: expected host:port", t.URI))
		}
		// Localhost-only enforcement for connect-proxy is done in extAuthzConfig.validate()
		// where the proxy kind is available. Target.validate() only checks format here.
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

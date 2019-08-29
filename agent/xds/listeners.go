package xds

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	extauthz "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/ext_authz/v2"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	envoytcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// listenersFromSnapshot returns the xDS API representation of the "listeners" in the snapshot.
func (s *Server) listenersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.listenersFromSnapshotConnectProxy(cfgSnap, token)
	case structs.ServiceKindMeshGateway:
		return s.listenersFromSnapshotMeshGateway(cfgSnap, token)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// listenersFromSnapshotConnectProxy returns the "listeners" for a connect proxy service
func (s *Server) listenersFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	// One listener for each upstream plus the public one
	resources := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	// Configure public listener
	var err error
	resources[0], err = s.makePublicListener(cfgSnap, token)
	if err != nil {
		return nil, err
	}
	for i, u := range cfgSnap.Proxy.Upstreams {
		id := u.Identifier()

		var chain *structs.CompiledDiscoveryChain
		if u.DestinationType != structs.UpstreamDestTypePreparedQuery {
			chain = cfgSnap.ConnectProxy.DiscoveryChain[id]
		}

		var upstreamListener proto.Message
		if chain == nil || chain.IsDefault() {
			upstreamListener, err = s.makeUpstreamListenerIgnoreDiscoveryChain(&u, chain, cfgSnap)
		} else {
			upstreamListener, err = s.makeUpstreamListenerForDiscoveryChain(&u, chain, cfgSnap)
		}
		if err != nil {
			return nil, err
		}
		resources[i+1] = upstreamListener
	}
	return resources, nil
}

// listenersFromSnapshotMeshGateway returns the "listener" for a mesh-gateway service
func (s *Server) listenersFromSnapshotMeshGateway(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	cfg, err := ParseMeshGatewayConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse Connect.Proxy.Config: %s", err)
	}

	// TODO - prevent invalid configurations of binding to the same port/addr
	//        twice including with the any addresses

	var resources []proto.Message
	if !cfg.NoDefaultBind {
		addr := cfgSnap.Address
		if addr == "" {
			addr = "0.0.0.0"
		}

		l, err := s.makeGatewayListener("default", addr, cfgSnap.Port, cfgSnap)
		if err != nil {
			return nil, err
		}
		resources = append(resources, l)
	}

	if cfg.BindTaggedAddresses {
		for name, addrCfg := range cfgSnap.TaggedAddresses {
			l, err := s.makeGatewayListener(name, addrCfg.Address, addrCfg.Port, cfgSnap)
			if err != nil {
				return nil, err
			}
			resources = append(resources, l)
		}
	}

	for name, addrCfg := range cfg.BindAddresses {
		l, err := s.makeGatewayListener(name, addrCfg.Address, addrCfg.Port, cfgSnap)
		if err != nil {
			return nil, err
		}
		resources = append(resources, l)
	}

	return resources, err
}

// makeListener returns a listener with name and bind details set. Filters must
// be added before it's useful.
//
// Note on names: Envoy listeners attempt graceful transitions of connections
// when their config changes but that means they can't have their bind address
// or port changed in a running instance. Since our users might choose to change
// a bind address or port for the public or upstream listeners, we need to
// encode those into the unique name for the listener such that if the user
// changes them, we actually create a whole new listener on the new address and
// port. Envoy should take care of closing the old one once it sees it's no
// longer in the config.
func makeListener(name, addr string, port int) *envoy.Listener {
	return &envoy.Listener{
		Name:    fmt.Sprintf("%s:%s:%d", name, addr, port),
		Address: makeAddress(addr, port),
	}
}

// makeListenerFromUserConfig returns the listener config decoded from an
// arbitrary proto3 json format string or an error if it's invalid.
//
// For now we only support embedding in JSON strings because of the hcl parsing
// pain (see config.go comment above call to PatchSliceOfMaps). Until we
// refactor config parser a _lot_ user's opaque config that contains arrays will
// be mangled. We could actually fix that up in mapstructure which knows the
// type of the target so could resolve the slices to singletons unambiguously
// and it would work for us here... but we still have the problem that the
// config would render incorrectly in general in our HTTP API responses so we
// really need to fix it "properly".
//
// When we do that we can support just nesting the config directly into the
// JSON/hcl naturally but this is a stop-gap that gets us an escape hatch
// immediately. It's also probably not a bad thing to support long-term since
// any config generated by other systems will likely be in canonical protobuf
// from rather than our slight variant in JSON/hcl.
func makeListenerFromUserConfig(configJSON string) (*envoy.Listener, error) {
	// Figure out if there is an @type field. We don't require is since we know
	// this will be a listener but unmarshalling into types.Any fails if it's not
	// there and unmarshalling into listener directly fails if it is...
	var jsonFields map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &jsonFields); err != nil {
		return nil, err
	}

	var l envoy.Listener

	if _, ok := jsonFields["@type"]; ok {
		// Type field is present so decode it as a types.Any
		var any types.Any
		err := jsonpb.UnmarshalString(configJSON, &any)
		if err != nil {
			return nil, err
		}
		// And then unmarshal the listener again...
		err = proto.Unmarshal(any.Value, &l)
		if err != nil {
			return nil, err
		}
		return &l, err
	}

	// No @type so try decoding as a straight listener.
	err := jsonpb.UnmarshalString(configJSON, &l)
	return &l, err
}

// Ensure that the first filter in each filter chain of a public listener is the
// authz filter to prevent unauthorized access and that every filter chain uses
// our TLS certs. We might allow users to work around this later if there is a
// good use case but this is actually a feature for now as it allows them to
// specify custom listener params in config but still get our certs delivered
// dynamically and intentions enforced without coming up with some complicated
// templating/merging solution.
func injectConnectFilters(cfgSnap *proxycfg.ConfigSnapshot, token string, listener *envoy.Listener) error {
	authFilter, err := makeExtAuthFilter(token)
	if err != nil {
		return err
	}
	for idx := range listener.FilterChains {
		// Insert our authz filter before any others
		listener.FilterChains[idx].Filters =
			append([]envoylistener.Filter{authFilter}, listener.FilterChains[idx].Filters...)

		// Force our TLS for all filter chains on a public listener
		listener.FilterChains[idx].TlsContext = &envoyauth.DownstreamTlsContext{
			CommonTlsContext:         makeCommonTLSContext(cfgSnap),
			RequireClientCertificate: &types.BoolValue{Value: true},
		}
	}
	return nil
}

func (s *Server) makePublicListener(cfgSnap *proxycfg.ConfigSnapshot, token string) (proto.Message, error) {
	var l *envoy.Listener
	var err error

	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse Connect.Proxy.Config: %s", err)
	}

	if cfg.PublicListenerJSON != "" {
		l, err = makeListenerFromUserConfig(cfg.PublicListenerJSON)
		if err != nil {
			return l, err
		}
		// In the happy path don't return yet as we need to inject TLS config still.
	}

	if l == nil {
		// No user config, use default listener
		addr := cfgSnap.Address

		// Override with bind address if one is set, otherwise default
		// to 0.0.0.0
		if cfg.BindAddress != "" {
			addr = cfg.BindAddress
		} else if addr == "" {
			addr = "0.0.0.0"
		}

		// Override with bind port if one is set, otherwise default to
		// proxy service's address
		port := cfgSnap.Port
		if cfg.BindPort != 0 {
			port = cfg.BindPort
		}

		l = makeListener(PublicListenerName, addr, port)

		filter, err := makeListenerFilter(false, cfg.Protocol, "public_listener", LocalAppClusterName, "", true)
		if err != nil {
			return nil, err
		}
		l.FilterChains = []envoylistener.FilterChain{
			{
				Filters: []envoylistener.Filter{
					filter,
				},
			},
		}
	}

	err = injectConnectFilters(cfgSnap, token, l)
	return l, err
}

// makeUpstreamListenerIgnoreDiscoveryChain counterintuitively takes an (optional) chain
func (s *Server) makeUpstreamListenerIgnoreDiscoveryChain(
	u *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
) (proto.Message, error) {
	cfg, err := ParseUpstreamConfig(u.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse Upstream[%s].Config: %s",
			u.Identifier(), err)
	}
	if cfg.ListenerJSON != "" {
		return makeListenerFromUserConfig(cfg.ListenerJSON)
	}

	addr := u.LocalBindAddress
	if addr == "" {
		addr = "127.0.0.1"
	}

	upstreamID := u.Identifier()

	dc := u.Datacenter
	if dc == "" {
		dc = cfgSnap.Datacenter
	}
	sni := connect.UpstreamSNI(u, "", dc, cfgSnap.Roots.TrustDomain)

	clusterName := CustomizeClusterName(sni, chain)

	l := makeListener(upstreamID, addr, u.LocalBindPort)
	filter, err := makeListenerFilter(false, cfg.Protocol, upstreamID, clusterName, "upstream_", false)
	if err != nil {
		return nil, err
	}

	l.FilterChains = []envoylistener.FilterChain{
		{
			Filters: []envoylistener.Filter{
				filter,
			},
		},
	}
	return l, nil
}

func (s *Server) makeGatewayListener(name, addr string, port int, cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Listener, error) {
	tlsInspector, err := makeTLSInspectorListenerFilter()
	if err != nil {
		return nil, err
	}

	sniCluster, err := makeSNIClusterFilter()
	if err != nil {
		return nil, err
	}

	// The cluster name here doesn't matter as the sni_cluster
	// filter will fill it in for us.
	tcpProxy, err := makeTCPProxyFilter(name, "", "mesh_gateway_local_")
	if err != nil {
		return nil, err
	}

	sniClusterChain := envoylistener.FilterChain{
		Filters: []envoylistener.Filter{
			sniCluster,
			tcpProxy,
		},
	}

	l := makeListener(name, addr, port)
	l.ListenerFilters = []envoylistener.ListenerFilter{tlsInspector}

	// TODO (mesh-gateway) - Do we need to create clusters for all the old trust domains as well?
	// We need 1 Filter Chain per datacenter
	for dc := range cfgSnap.MeshGateway.GatewayGroups {
		clusterName := connect.DatacenterSNI(dc, cfgSnap.Roots.TrustDomain)
		filterName := fmt.Sprintf("%s_%s", name, dc)
		dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_remote_")
		if err != nil {
			return nil, err
		}

		l.FilterChains = append(l.FilterChains, envoylistener.FilterChain{
			FilterChainMatch: &envoylistener.FilterChainMatch{
				ServerNames: []string{fmt.Sprintf("*.%s", clusterName)},
			},
			Filters: []envoylistener.Filter{
				dcTCPProxy,
			},
		})
	}

	// This needs to get tacked on at the end as it has no
	// matching and will act as a catch all
	l.FilterChains = append(l.FilterChains, sniClusterChain)

	return l, nil
}

func (s *Server) makeUpstreamListenerForDiscoveryChain(
	u *structs.Upstream,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
) (proto.Message, error) {
	cfg, err := ParseUpstreamConfigNoDefaults(u.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Printf("[WARN] envoy: failed to parse Upstream[%s].Config: %s",
			u.Identifier(), err)
	}
	if cfg.ListenerJSON != "" {
		s.Logger.Printf("[WARN] envoy: ignoring escape hatch setting Upstream[%s].Config[%s] because a discovery chain for %q is configured",
			u.Identifier(), "envoy_listener_json", chain.ServiceName)
	}

	addr := u.LocalBindAddress
	if addr == "" {
		addr = "127.0.0.1"
	}

	upstreamID := u.Identifier()

	l := makeListener(upstreamID, addr, u.LocalBindPort)

	proto := cfg.Protocol
	if proto == "" {
		proto = chain.Protocol
	}

	if proto == "" {
		proto = "tcp"
	}

	filter, err := makeListenerFilter(true, proto, upstreamID, "", "upstream_", false)
	if err != nil {
		return nil, err
	}

	l.FilterChains = []envoylistener.FilterChain{
		{
			Filters: []envoylistener.Filter{
				filter,
			},
		},
	}
	return l, nil
}

func makeListenerFilter(useRDS bool, protocol, filterName, cluster, statPrefix string, ingress bool) (envoylistener.Filter, error) {
	switch protocol {
	case "grpc":
		return makeHTTPFilter(useRDS, filterName, cluster, statPrefix, ingress, true, true)
	case "http2":
		return makeHTTPFilter(useRDS, filterName, cluster, statPrefix, ingress, false, true)
	case "http":
		return makeHTTPFilter(useRDS, filterName, cluster, statPrefix, ingress, false, false)
	case "tcp":
		fallthrough
	default:
		return makeTCPProxyFilter(filterName, cluster, statPrefix)
	}
}

func makeTLSInspectorListenerFilter() (envoylistener.ListenerFilter, error) {
	return envoylistener.ListenerFilter{Name: util.TlsInspector}, nil
}

// TODO(rb): should this be dead code?
func makeSNIFilterChainMatch(sniMatch string) (*envoylistener.FilterChainMatch, error) {
	return &envoylistener.FilterChainMatch{
		ServerNames: []string{sniMatch},
	}, nil
}

func makeSNIClusterFilter() (envoylistener.Filter, error) {
	// This filter has no config which is why we are not calling make
	return envoylistener.Filter{Name: "envoy.filters.network.sni_cluster"}, nil
}

func makeTCPProxyFilter(filterName, cluster, statPrefix string) (envoylistener.Filter, error) {
	cfg := &envoytcp.TcpProxy{
		StatPrefix:       makeStatPrefix("tcp", statPrefix, filterName),
		ClusterSpecifier: &envoytcp.TcpProxy_Cluster{Cluster: cluster},
	}
	return makeFilter("envoy.tcp_proxy", cfg)
}

func makeStatPrefix(protocol, prefix, filterName string) string {
	// Replace colons here because Envoy does that in the metrics for the actual
	// clusters but doesn't in the stat prefix here while dashboards assume they
	// will match.
	return fmt.Sprintf("%s%s_%s", prefix, strings.Replace(filterName, ":", "_", -1), protocol)
}

func makeHTTPFilter(
	useRDS bool,
	filterName, cluster, statPrefix string,
	ingress, grpc, http2 bool,
) (envoylistener.Filter, error) {
	op := envoyhttp.INGRESS
	if !ingress {
		op = envoyhttp.EGRESS
	}
	proto := "http"
	if grpc {
		proto = "grpc"
	}
	cfg := &envoyhttp.HttpConnectionManager{
		StatPrefix: makeStatPrefix(proto, statPrefix, filterName),
		CodecType:  envoyhttp.AUTO,
		HttpFilters: []*envoyhttp.HttpFilter{
			&envoyhttp.HttpFilter{
				Name: "envoy.router",
			},
		},
		Tracing: &envoyhttp.HttpConnectionManager_Tracing{
			OperationName: op,
			// Don't trace any requests by default unless the client application
			// explicitly propagates trace headers that indicate this should be
			// sampled.
			RandomSampling: &envoytype.Percent{Value: 0.0},
		},
	}

	if useRDS {
		if cluster != "" {
			return envoylistener.Filter{}, fmt.Errorf("cannot specify cluster name when using RDS")
		}
		cfg.RouteSpecifier = &envoyhttp.HttpConnectionManager_Rds{
			Rds: &envoyhttp.Rds{
				RouteConfigName: filterName,
				ConfigSource: envoycore.ConfigSource{
					ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
						Ads: &envoycore.AggregatedConfigSource{},
					},
				},
			},
		}
	} else {
		if cluster == "" {
			return envoylistener.Filter{}, fmt.Errorf("must specify cluster name when not using RDS")
		}
		cfg.RouteSpecifier = &envoyhttp.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy.RouteConfiguration{
				Name: filterName,
				VirtualHosts: []envoyroute.VirtualHost{
					envoyroute.VirtualHost{
						Name:    filterName,
						Domains: []string{"*"},
						Routes: []envoyroute.Route{
							envoyroute.Route{
								Match: envoyroute.RouteMatch{
									PathSpecifier: &envoyroute.RouteMatch_Prefix{
										Prefix: "/",
									},
									// TODO(banks) Envoy supports matching only valid GRPC
									// requests which might be nice to add here for gRPC services
									// but it's not supported in our current envoy SDK version
									// although docs say it was supported by 1.8.0. Going to defer
									// that until we've updated the deps.
								},
								Action: &envoyroute.Route_Route{
									Route: &envoyroute.RouteAction{
										ClusterSpecifier: &envoyroute.RouteAction_Cluster{
											Cluster: cluster,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	if http2 {
		cfg.Http2ProtocolOptions = &envoycore.Http2ProtocolOptions{}
	}

	if grpc {
		// Add grpc bridge before router
		cfg.HttpFilters = append([]*envoyhttp.HttpFilter{&envoyhttp.HttpFilter{
			Name:       "envoy.grpc_http1_bridge",
			ConfigType: &envoyhttp.HttpFilter_Config{Config: &types.Struct{}},
		}}, cfg.HttpFilters...)
	}

	return makeFilter("envoy.http_connection_manager", cfg)
}

func makeExtAuthFilter(token string) (envoylistener.Filter, error) {
	cfg := &extauthz.ExtAuthz{
		StatPrefix: "connect_authz",
		GrpcService: &envoycore.GrpcService{
			// Attach token header so we can authorize the callbacks. Technically
			// authorize is not really protected data but we locked down the HTTP
			// implementation to need service:write and since we have the token that
			// has that it's pretty reasonable to set it up here.
			InitialMetadata: []*envoycore.HeaderValue{
				&envoycore.HeaderValue{
					Key:   "x-consul-token",
					Value: token,
				},
			},
			TargetSpecifier: &envoycore.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &envoycore.GrpcService_EnvoyGrpc{
					ClusterName: LocalAgentClusterName,
				},
			},
		},
		FailureModeAllow: false,
	}
	return makeFilter("envoy.ext_authz", cfg)
}

func makeFilter(name string, cfg proto.Message) (envoylistener.Filter, error) {
	// Ridiculous dance to make that pbstruct into types.Struct by... encoding it
	// as JSON and decoding again!!
	cfgStruct, err := util.MessageToStruct(cfg)
	if err != nil {
		return envoylistener.Filter{}, err
	}

	return envoylistener.Filter{
		Name:       name,
		ConfigType: &envoylistener.Filter_Config{Config: cfgStruct},
	}, nil
}

func makeCommonTLSContext(cfgSnap *proxycfg.ConfigSnapshot) *envoyauth.CommonTlsContext {
	// Concatenate all the root PEMs into one.
	// TODO(banks): verify this actually works with Envoy (docs are not clear).
	rootPEMS := ""
	if cfgSnap.Roots == nil {
		return nil
	}
	for _, root := range cfgSnap.Roots.Roots {
		rootPEMS += root.RootCert
	}

	return &envoyauth.CommonTlsContext{
		TlsParams: &envoyauth.TlsParameters{},
		TlsCertificates: []*envoyauth.TlsCertificate{
			&envoyauth.TlsCertificate{
				CertificateChain: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_InlineString{
						InlineString: cfgSnap.ConnectProxy.Leaf.CertPEM,
					},
				},
				PrivateKey: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_InlineString{
						InlineString: cfgSnap.ConnectProxy.Leaf.PrivateKeyPEM,
					},
				},
			},
		},
		ValidationContextType: &envoyauth.CommonTlsContext_ValidationContext{
			ValidationContext: &envoyauth.CertificateValidationContext{
				// TODO(banks): later for L7 support we may need to configure ALPN here.
				TrustedCa: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_InlineString{
						InlineString: rootPEMS,
					},
				},
			},
		},
	}
}

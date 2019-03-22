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
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	extauthz "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/ext_authz/v2"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	envoytcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// listenersFromSnapshot returns the xDS API representation of the "listeners"
// in the snapshot.
func (s *Server) listenersFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	// One listener for each upstream plus the public one
	resources := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	// Configure public listener
	var err error
	resources[0], err = s.makePublicListener(cfgSnap, token)
	if err != nil {
		return nil, err
	}
	for i, u := range cfgSnap.Proxy.Upstreams {
		resources[i+1], err = s.makeUpstreamListener(&u)
		if err != nil {
			return nil, err
		}
	}
	return resources, nil
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
// pain (see config.go comment above call to patchSliceOfMaps). Until we
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
		if addr == "" {
			addr = "0.0.0.0"
		}
		l = makeListener(PublicListenerName, addr, cfgSnap.Port)

		filter, err := makeListenerFilter(cfg.Protocol, "public_listener", LocalAppClusterName, "")
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

func (s *Server) makeUpstreamListener(u *structs.Upstream) (proto.Message, error) {
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

	l := makeListener(u.Identifier(), addr, u.LocalBindPort)
	filter, err := makeListenerFilter(cfg.Protocol, u.Identifier(), u.Identifier(), "upstream_")
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

func makeListenerFilter(protocol, filterName, cluster, statPrefix string) (envoylistener.Filter, error) {
	switch protocol {
	case "grpc":
		// TODO(banks) test this, we probably need to inject extra settings to
		// make gRPC work nicely.
		fallthrough
	case "http":
		return makeHTTPFilter(filterName, cluster, statPrefix)
	case "tcp":
		fallthrough
	default:
		return makeTCPProxyFilter(filterName, cluster, statPrefix)
	}
}

func makeTCPProxyFilter(filterName, cluster, statPrefix string) (envoylistener.Filter, error) {
	cfg := &envoytcp.TcpProxy{
		StatPrefix: makeStatPrefix("tcp", statPrefix, filterName),
		Cluster:    cluster,
	}
	return makeFilter("envoy.tcp_proxy", cfg)
}

func makeStatPrefix(protocol, prefix, filterName string) string {
	// Replace colons here because Envoy does that in the metrics for the actual
	// clusters but doesn't in the stat prefix here while dashboards assume they
	// will match.
	return fmt.Sprintf("%s%s_%s", prefix, strings.Replace(filterName, ":", "_", -1), protocol)
}

func makeHTTPFilter(filterName, cluster, statPrefix string) (envoylistener.Filter, error) {
	cfg := &envoyhttp.HttpConnectionManager{
		StatPrefix: makeStatPrefix("http", statPrefix, filterName),
		CodecType:  envoyhttp.AUTO,
		RouteSpecifier: &envoyhttp.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy.RouteConfiguration{
				Name: filterName,
				VirtualHosts: []route.VirtualHost{
					route.VirtualHost{
						Name:    filterName,
						Domains: []string{"*"},
						Routes: []route.Route{
							route.Route{
								Match: route.RouteMatch{
									PathSpecifier: &route.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &route.Route_Route{
									Route: &route.RouteAction{
										ClusterSpecifier: &route.RouteAction_Cluster{
											Cluster: cluster,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		HttpFilters: []*envoyhttp.HttpFilter{
			&envoyhttp.HttpFilter{
				Name: "envoy.router",
			},
		},
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
		Name:   name,
		Config: cfgStruct,
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
						InlineString: cfgSnap.Leaf.CertPEM,
					},
				},
				PrivateKey: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_InlineString{
						InlineString: cfgSnap.Leaf.PrivateKeyPEM,
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

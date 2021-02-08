package xds

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
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
	"github.com/envoyproxy/go-control-plane/pkg/conversion"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
)

// listenersFromSnapshot returns the xDS API representation of the "listeners" in the snapshot.
func (s *Server) listenersFromSnapshot(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		return s.listenersFromSnapshotConnectProxy(cInfo, cfgSnap)
	case structs.ServiceKindTerminatingGateway:
		return s.listenersFromSnapshotGateway(cInfo, cfgSnap)
	case structs.ServiceKindMeshGateway:
		return s.listenersFromSnapshotGateway(cInfo, cfgSnap)
	case structs.ServiceKindIngressGateway:
		return s.listenersFromSnapshotGateway(cInfo, cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

// listenersFromSnapshotConnectProxy returns the "listeners" for a connect proxy service
func (s *Server) listenersFromSnapshotConnectProxy(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	// One listener for each upstream plus the public one
	resources := make([]proto.Message, len(cfgSnap.Proxy.Upstreams)+1)

	// Configure public listener
	var err error
	resources[0], err = s.makePublicListener(cInfo, cfgSnap)
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
		upstreamListener, err = s.makeUpstreamListenerForDiscoveryChain(
			&u,
			u.LocalBindAddress,
			chain,
			cfgSnap,
			nil,
		)
		if err != nil {
			return nil, err
		}
		resources[i+1] = upstreamListener
	}

	cfgSnap.Proxy.Expose.Finalize()
	paths := cfgSnap.Proxy.Expose.Paths

	// Add service health checks to the list of paths to create listeners for if needed
	if cfgSnap.Proxy.Expose.Checks {
		psid := structs.NewServiceID(cfgSnap.Proxy.DestinationServiceID, &cfgSnap.ProxyID.EnterpriseMeta)
		for _, check := range s.CheckFetcher.ServiceHTTPBasedChecks(psid) {
			p, err := parseCheckPath(check)
			if err != nil {
				s.Logger.Warn("failed to create listener for", "check", check.CheckID, "error", err)
				continue
			}
			paths = append(paths, p)
		}
	}

	// Configure additional listener for exposed check paths
	for _, path := range paths {
		clusterName := LocalAppClusterName
		if path.LocalPathPort != cfgSnap.Proxy.LocalServicePort {
			clusterName = makeExposeClusterName(path.LocalPathPort)
		}

		l, err := s.makeExposedCheckListener(cfgSnap, clusterName, path)
		if err != nil {
			return nil, err
		}
		resources = append(resources, l)
	}

	return resources, nil
}

func parseCheckPath(check structs.CheckType) (structs.ExposePath, error) {
	var path structs.ExposePath

	if check.HTTP != "" {
		path.Protocol = "http"

		// Get path and local port from original HTTP target
		u, err := url.Parse(check.HTTP)
		if err != nil {
			return path, fmt.Errorf("failed to parse url '%s': %v", check.HTTP, err)
		}
		path.Path = u.Path

		_, portStr, err := net.SplitHostPort(u.Host)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.HTTP, err)
		}
		path.LocalPathPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.HTTP, err)
		}

		// Get listener port from proxied HTTP target
		u, err = url.Parse(check.ProxyHTTP)
		if err != nil {
			return path, fmt.Errorf("failed to parse url '%s': %v", check.ProxyHTTP, err)
		}

		_, portStr, err = net.SplitHostPort(u.Host)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.ProxyHTTP, err)
		}
		path.ListenerPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.ProxyHTTP, err)
		}
	}

	if check.GRPC != "" {
		path.Path = "/grpc.health.v1.Health/Check"
		path.Protocol = "http2"

		// Get local port from original GRPC target of the form: host/service
		proxyServerAndService := strings.SplitN(check.GRPC, "/", 2)
		_, portStr, err := net.SplitHostPort(proxyServerAndService[0])
		if err != nil {
			return path, fmt.Errorf("failed to split host/port from '%s': %v", check.GRPC, err)
		}
		path.LocalPathPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.GRPC, err)
		}

		// Get listener port from proxied GRPC target of the form: host/service
		proxyServerAndService = strings.SplitN(check.ProxyGRPC, "/", 2)
		_, portStr, err = net.SplitHostPort(proxyServerAndService[0])
		if err != nil {
			return path, fmt.Errorf("failed to split host/port from '%s': %v", check.ProxyGRPC, err)
		}
		path.ListenerPort, err = strconv.Atoi(portStr)
		if err != nil {
			return path, fmt.Errorf("failed to parse port from '%s': %v", check.ProxyGRPC, err)
		}
	}

	path.ParsedFromCheck = true

	return path, nil
}

// listenersFromSnapshotGateway returns the "listener" for a terminating-gateway or mesh-gateway service
func (s *Server) listenersFromSnapshotGateway(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	cfg, err := ParseGatewayConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// We'll collect all of the desired listeners first, and deduplicate them later.
	type namedAddress struct {
		name string
		structs.ServiceAddress
	}
	addrs := make([]namedAddress, 0)

	var resources []proto.Message
	if !cfg.NoDefaultBind {
		addr := cfgSnap.Address
		if addr == "" {
			addr = "0.0.0.0"
		}

		a := structs.ServiceAddress{
			Address: addr,
			Port:    cfgSnap.Port,
		}
		addrs = append(addrs, namedAddress{name: "default", ServiceAddress: a})
	}

	if cfg.BindTaggedAddresses {
		for name, addrCfg := range cfgSnap.TaggedAddresses {
			a := structs.ServiceAddress{
				Address: addrCfg.Address,
				Port:    addrCfg.Port,
			}
			addrs = append(addrs, namedAddress{name: name, ServiceAddress: a})
		}
	}

	for name, addrCfg := range cfg.BindAddresses {
		a := structs.ServiceAddress{
			Address: addrCfg.Address,
			Port:    addrCfg.Port,
		}
		addrs = append(addrs, namedAddress{name: name, ServiceAddress: a})
	}

	// Prevent invalid configurations of binding to the same port/addr twice
	// including with the any addresses
	//
	// Sort the list and then if two items share a service address, take the
	// first one to ensure we generate one listener per address and it's
	// stable.
	sort.Slice(addrs, func(i, j int) bool {
		return addrs[i].name < addrs[j].name
	})

	// Make listeners and deduplicate on the fly.
	seen := make(map[structs.ServiceAddress]bool)
	for _, a := range addrs {
		if seen[a.ServiceAddress] {
			continue
		}
		seen[a.ServiceAddress] = true

		var l *envoy.Listener

		switch cfgSnap.Kind {
		case structs.ServiceKindTerminatingGateway:
			l, err = s.makeTerminatingGatewayListener(cInfo, cfgSnap, a.name, a.Address, a.Port)
			if err != nil {
				return nil, err
			}
		case structs.ServiceKindIngressGateway:
			listeners, err := s.makeIngressGatewayListeners(a.Address, cfgSnap)
			if err != nil {
				return nil, err
			}
			resources = append(resources, listeners...)
		case structs.ServiceKindMeshGateway:
			l, err = s.makeMeshGatewayListener(a.name, a.Address, a.Port, cfgSnap)
			if err != nil {
				return nil, err
			}
		}
		if l != nil {
			resources = append(resources, l)
		}
	}
	return resources, err
}

func (s *Server) makeIngressGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var resources []proto.Message

	for listenerKey, upstreams := range cfgSnap.IngressGateway.Upstreams {
		var tlsContext *envoyauth.DownstreamTlsContext
		if cfgSnap.IngressGateway.TLSEnabled {
			tlsContext = &envoyauth.DownstreamTlsContext{
				CommonTlsContext:         makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf()),
				RequireClientCertificate: &wrappers.BoolValue{Value: false},
			}
		}

		if listenerKey.Protocol == "tcp" {
			// We rely on the invariant of upstreams slice always having at least 1
			// member, because this key/value pair is created only when a
			// GatewayService is returned in the RPC
			u := upstreams[0]
			id := u.Identifier()

			chain := cfgSnap.IngressGateway.DiscoveryChain[id]

			var upstreamListener proto.Message
			upstreamListener, err := s.makeUpstreamListenerForDiscoveryChain(
				&u,
				address,
				chain,
				cfgSnap,
				tlsContext,
			)
			if err != nil {
				return nil, err
			}
			resources = append(resources, upstreamListener)
		} else {
			// If multiple upstreams share this port, make a special listener for the protocol.
			listener := makeListener(listenerKey.Protocol, address, listenerKey.Port)
			filter, err := makeListenerFilter(
				true, listenerKey.Protocol, listenerKey.RouteName(), "", "ingress_upstream_", "", false)
			if err != nil {
				return nil, err
			}

			listener.FilterChains = []*envoylistener.FilterChain{
				{
					Filters: []*envoylistener.Filter{
						filter,
					},
					TlsContext: tlsContext,
				},
			}
			resources = append(resources, listener)
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
	// this will be a listener but unmarshalling into any.Any fails if it's not
	// there and unmarshalling into listener directly fails if it is...
	var jsonFields map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &jsonFields); err != nil {
		return nil, err
	}

	var l envoy.Listener

	if _, ok := jsonFields["@type"]; ok {
		// Type field is present so decode it as a any.Any
		var any any.Any
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
func injectConnectFilters(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot, listener *envoy.Listener) error {
	authFilter, err := makeExtAuthFilter(cInfo.Token)
	if err != nil {
		return err
	}
	for idx := range listener.FilterChains {
		// Insert our authz filter before any others
		listener.FilterChains[idx].Filters =
			append([]*envoylistener.Filter{authFilter}, listener.FilterChains[idx].Filters...)

		listener.FilterChains[idx].TlsContext = &envoyauth.DownstreamTlsContext{
			CommonTlsContext:         makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.Leaf()),
			RequireClientCertificate: &wrappers.BoolValue{Value: true},
		}
	}
	return nil
}

func (s *Server) makePublicListener(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) (proto.Message, error) {
	var l *envoy.Listener
	var err error

	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
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

		filter, err := makeListenerFilter(
			false, cfg.Protocol, "public_listener", LocalAppClusterName, "", "", true)
		if err != nil {
			return nil, err
		}
		l.FilterChains = []*envoylistener.FilterChain{
			{
				Filters: []*envoylistener.Filter{
					filter,
				},
			},
		}
	}

	err = injectConnectFilters(cInfo, cfgSnap, l)
	return l, err
}

func (s *Server) makeExposedCheckListener(cfgSnap *proxycfg.ConfigSnapshot, cluster string, path structs.ExposePath) (proto.Message, error) {
	cfg, err := ParseProxyConfig(cfgSnap.Proxy.Config)
	if err != nil {
		// Don't hard fail on a config typo, just warn. The parse func returns
		// default config if there is an error so it's safe to continue.
		s.Logger.Warn("failed to parse Connect.Proxy.Config", "error", err)
	}

	// No user config, use default listener
	addr := cfgSnap.Address

	// Override with bind address if one is set, otherwise default to 0.0.0.0
	if cfg.BindAddress != "" {
		addr = cfg.BindAddress
	} else if addr == "" {
		addr = "0.0.0.0"
	}

	// Strip any special characters from path to make a valid and hopefully unique name
	r := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	strippedPath := r.ReplaceAllString(path.Path, "")
	listenerName := fmt.Sprintf("exposed_path_%s", strippedPath)

	l := makeListener(listenerName, addr, path.ListenerPort)

	filterName := fmt.Sprintf("exposed_path_filter_%s_%d", strippedPath, path.ListenerPort)

	f, err := makeListenerFilter(false, path.Protocol, filterName, cluster, "", path.Path, true)
	if err != nil {
		return nil, err
	}

	chain := &envoylistener.FilterChain{
		Filters: []*envoylistener.Filter{f},
	}

	// For registered checks restrict traffic sources to localhost and Consul's advertise addr
	if path.ParsedFromCheck {

		// For the advertise addr we use a CidrRange that only matches one address
		advertise := s.CfgFetcher.AdvertiseAddrLAN()

		// Get prefix length based on whether address is ipv4 (32 bits) or ipv6 (128 bits)
		advertiseLen := 32
		ip := net.ParseIP(advertise)
		if ip != nil && strings.Contains(advertise, ":") {
			advertiseLen = 128
		}

		chain.FilterChainMatch = &envoylistener.FilterChainMatch{
			SourcePrefixRanges: []*envoycore.CidrRange{
				{AddressPrefix: "127.0.0.1", PrefixLen: &wrappers.UInt32Value{Value: 8}},
				{AddressPrefix: "::1", PrefixLen: &wrappers.UInt32Value{Value: 128}},
				{AddressPrefix: advertise, PrefixLen: &wrappers.UInt32Value{Value: uint32(advertiseLen)}},
			},
		}
	}

	l.FilterChains = []*envoylistener.FilterChain{chain}

	return l, err
}

func (s *Server) makeTerminatingGatewayListener(
	cInfo connectionInfo,
	cfgSnap *proxycfg.ConfigSnapshot,
	name, addr string,
	port int,
) (*envoy.Listener, error) {
	l := makeListener(name, addr, port)

	tlsInspector, err := makeTLSInspectorListenerFilter()
	if err != nil {
		return nil, err
	}
	l.ListenerFilters = []*envoylistener.ListenerFilter{tlsInspector}

	// Make a FilterChain for each linked service
	// Match on the cluster name,
	for svc, _ := range cfgSnap.TerminatingGateway.ServiceGroups {
		clusterName := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)
		resolver, hasResolver := cfgSnap.TerminatingGateway.ServiceResolvers[svc]

		// Skip the service if we don't have a cert to present for mTLS
		if cert, ok := cfgSnap.TerminatingGateway.ServiceLeaves[svc]; !ok || cert == nil {
			// TODO (gateways) (freddy) Should the error suggest that the issue may be ACLs? (need service:write on service)
			s.Logger.Named(logging.TerminatingGateway).
				Error("no client certificate available for linked service, skipping filter chain creation",
					"service", svc.String(), "error", err)
			continue
		}

		clusterChain, err := s.sniFilterChainTerminatingGateway(cInfo, cfgSnap, name, clusterName, svc)
		if err != nil {
			return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", clusterName, err)
		}
		l.FilterChains = append(l.FilterChains, clusterChain)

		// if there is a service-resolver for this service then also setup subset filter chains for it
		if hasResolver {
			// generate 1 filter chain for each service subset
			for subsetName := range resolver.Subsets {
				clusterName := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

				clusterChain, err := s.sniFilterChainTerminatingGateway(cInfo, cfgSnap, name, clusterName, svc)
				if err != nil {
					return nil, fmt.Errorf("failed to make filter chain for cluster %q: %v", clusterName, err)
				}
				l.FilterChains = append(l.FilterChains, clusterChain)
			}
		}
	}

	// Before we add the fallback, sort these chains by the matched name. All
	// of these filter chains are independent, but envoy requires them to be in
	// some order. If we put them in a random order then every xDS iteration
	// envoy will force the listener to be replaced. Sorting these has no
	// effect on how they operate, but it does mean that we won't churn
	// listeners at idle.
	sort.Slice(l.FilterChains, func(i, j int) bool {
		return l.FilterChains[i].FilterChainMatch.ServerNames[0] < l.FilterChains[j].FilterChainMatch.ServerNames[0]
	})

	// This fallback catch-all filter ensures a listener will be present for health checks to pass
	// Envoy will reset these connections since known endpoints are caught by filter chain matches above
	tcpProxy, err := makeTCPProxyFilter(name, "", "terminating_gateway_")
	if err != nil {
		return nil, err
	}
	fallback := &envoylistener.FilterChain{
		Filters: []*envoylistener.Filter{
			{Name: "envoy.filters.network.sni_cluster"},
			tcpProxy,
		},
	}
	l.FilterChains = append(l.FilterChains, fallback)

	return l, nil
}

func (s *Server) sniFilterChainTerminatingGateway(
	cInfo connectionInfo,
	cfgSnap *proxycfg.ConfigSnapshot,
	listener, cluster string,
	service structs.ServiceName,
) (*envoylistener.FilterChain, error) {

	authFilter, err := makeExtAuthFilter(cInfo.Token)
	if err != nil {
		return nil, err
	}
	sniCluster, err := makeSNIClusterFilter()
	if err != nil {
		return nil, err
	}

	// The cluster name here doesn't matter as the sni_cluster filter will fill it in for us.
	statPrefix := fmt.Sprintf("terminating_gateway_%s_%s_", service.NamespaceOrDefault(), service.Name)
	tcpProxy, err := makeTCPProxyFilter(listener, "", statPrefix)
	if err != nil {
		return nil, err
	}

	return &envoylistener.FilterChain{
		FilterChainMatch: makeSNIFilterChainMatch(cluster),
		Filters: []*envoylistener.Filter{
			authFilter,
			sniCluster,
			tcpProxy,
		},
		TlsContext: &envoyauth.DownstreamTlsContext{
			CommonTlsContext:         makeCommonTLSContextFromLeaf(cfgSnap, cfgSnap.TerminatingGateway.ServiceLeaves[service]),
			RequireClientCertificate: &wrappers.BoolValue{Value: true},
		},
	}, err
}

func (s *Server) makeMeshGatewayListener(name, addr string, port int, cfgSnap *proxycfg.ConfigSnapshot) (*envoy.Listener, error) {
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

	sniClusterChain := &envoylistener.FilterChain{
		Filters: []*envoylistener.Filter{
			sniCluster,
			tcpProxy,
		},
	}

	l := makeListener(name, addr, port)
	l.ListenerFilters = []*envoylistener.ListenerFilter{tlsInspector}

	// TODO (mesh-gateway) - Do we need to create clusters for all the old trust domains as well?
	// We need 1 Filter Chain per datacenter
	datacenters := cfgSnap.MeshGateway.Datacenters()
	for _, dc := range datacenters {
		if dc == cfgSnap.Datacenter {
			continue // skip local
		}
		clusterName := connect.DatacenterSNI(dc, cfgSnap.Roots.TrustDomain)
		filterName := fmt.Sprintf("%s_%s", name, dc)
		dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_remote_")
		if err != nil {
			return nil, err
		}

		l.FilterChains = append(l.FilterChains, &envoylistener.FilterChain{
			FilterChainMatch: &envoylistener.FilterChainMatch{
				ServerNames: []string{fmt.Sprintf("*.%s", clusterName)},
			},
			Filters: []*envoylistener.Filter{
				dcTCPProxy,
			},
		})
	}

	if cfgSnap.ServiceMeta[structs.MetaWANFederationKey] == "1" && cfgSnap.ServerSNIFn != nil {
		for _, dc := range datacenters {
			if dc == cfgSnap.Datacenter {
				continue // skip local
			}
			clusterName := cfgSnap.ServerSNIFn(dc, "")
			filterName := fmt.Sprintf("%s_%s", name, dc)
			dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_remote_")
			if err != nil {
				return nil, err
			}

			l.FilterChains = append(l.FilterChains, &envoylistener.FilterChain{
				FilterChainMatch: &envoylistener.FilterChainMatch{
					ServerNames: []string{fmt.Sprintf("*.%s", clusterName)},
				},
				Filters: []*envoylistener.Filter{
					dcTCPProxy,
				},
			})
		}

		// Wildcard all flavors to each server.
		for _, srv := range cfgSnap.MeshGateway.ConsulServers {
			clusterName := cfgSnap.ServerSNIFn(cfgSnap.Datacenter, srv.Node.Node)

			filterName := fmt.Sprintf("%s_%s", name, cfgSnap.Datacenter)
			dcTCPProxy, err := makeTCPProxyFilter(filterName, clusterName, "mesh_gateway_local_server_")
			if err != nil {
				return nil, err
			}

			l.FilterChains = append(l.FilterChains, &envoylistener.FilterChain{
				FilterChainMatch: &envoylistener.FilterChainMatch{
					ServerNames: []string{fmt.Sprintf("%s", clusterName)},
				},
				Filters: []*envoylistener.Filter{
					dcTCPProxy,
				},
			})
		}
	}

	// This needs to get tacked on at the end as it has no
	// matching and will act as a catch all
	l.FilterChains = append(l.FilterChains, sniClusterChain)

	return l, nil
}

func (s *Server) makeUpstreamListenerForDiscoveryChain(
	u *structs.Upstream,
	address string,
	chain *structs.CompiledDiscoveryChain,
	cfgSnap *proxycfg.ConfigSnapshot,
	tlsContext *envoyauth.DownstreamTlsContext,
) (proto.Message, error) {
	if address == "" {
		address = "127.0.0.1"
	}
	upstreamID := u.Identifier()
	l := makeListener(upstreamID, address, u.LocalBindPort)

	cfg := getAndModifyUpstreamConfigForListener(s.Logger, u, chain)
	if cfg.ListenerJSON != "" {
		return makeListenerFromUserConfig(cfg.ListenerJSON)
	}

	useRDS := true
	clusterName := ""
	if chain == nil || chain.IsDefault() {
		dc := u.Datacenter
		if dc == "" {
			dc = cfgSnap.Datacenter
		}
		sni := connect.UpstreamSNI(u, "", dc, cfgSnap.Roots.TrustDomain)

		useRDS = false
		clusterName = CustomizeClusterName(sni, chain)

	} else if cfg.Protocol == "tcp" {
		startNode := chain.Nodes[chain.StartNode]
		if startNode == nil {
			return nil, fmt.Errorf("missing first node in compiled discovery chain for: %s", chain.ServiceName)
		} else if startNode.Type != structs.DiscoveryGraphNodeTypeResolver {
			return nil, fmt.Errorf("unexpected first node in discovery chain using protocol=%q: %s", cfg.Protocol, startNode.Type)
		}
		targetID := startNode.Resolver.Target
		target := chain.Targets[targetID]

		useRDS = false
		clusterName = CustomizeClusterName(target.Name, chain)
	}

	filter, err := makeListenerFilter(
		useRDS, cfg.Protocol, upstreamID, clusterName, "upstream_", "", false)
	if err != nil {
		return nil, err
	}

	l.FilterChains = []*envoylistener.FilterChain{
		{
			Filters: []*envoylistener.Filter{
				filter,
			},
			TlsContext: tlsContext,
		},
	}
	return l, nil
}

func getAndModifyUpstreamConfigForListener(logger hclog.Logger, u *structs.Upstream, chain *structs.CompiledDiscoveryChain) UpstreamConfig {
	var (
		cfg UpstreamConfig
		err error
	)

	if chain == nil || chain.IsDefault() {
		cfg, err = ParseUpstreamConfig(u.Config)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			logger.Warn("failed to parse", "upstream", u.Identifier(), "error", err)
		}
	} else {
		// Use NoDefaults here so that we can set the protocol to the chain
		// protocol if necessary
		cfg, err = ParseUpstreamConfigNoDefaults(u.Config)
		if err != nil {
			// Don't hard fail on a config typo, just warn. The parse func returns
			// default config if there is an error so it's safe to continue.
			logger.Warn("failed to parse", "upstream", u.Identifier(), "error", err)
		}

		if cfg.ListenerJSON != "" {
			logger.Warn("ignoring escape hatch setting because already configured for",
				"discovery chain", chain.ServiceName, "upstream", u.Identifier(), "config", "envoy_listener_json")

			// Remove from config struct so we don't use it later on
			cfg.ListenerJSON = ""
		}

		proto := cfg.Protocol
		if proto == "" {
			proto = chain.Protocol
		}

		if proto == "" {
			proto = "tcp"
		}

		// set back on the config so that we can use it from return value
		cfg.Protocol = proto
	}

	return cfg
}

func makeListenerFilter(
	useRDS bool,
	protocol, filterName, cluster, statPrefix, routePath string, ingress bool) (*envoylistener.Filter, error) {

	switch protocol {
	case "grpc":
		return makeHTTPFilter(useRDS, filterName, cluster, statPrefix, routePath, ingress, true, true)
	case "http2":
		return makeHTTPFilter(useRDS, filterName, cluster, statPrefix, routePath, ingress, false, true)
	case "http":
		return makeHTTPFilter(useRDS, filterName, cluster, statPrefix, routePath, ingress, false, false)
	case "tcp":
		fallthrough
	default:
		if useRDS {
			return nil, fmt.Errorf("RDS is not compatible with the tcp proxy filter")
		} else if cluster == "" {
			return nil, fmt.Errorf("cluster name is required for a tcp proxy filter")
		}
		return makeTCPProxyFilter(filterName, cluster, statPrefix)
	}
}

func makeTLSInspectorListenerFilter() (*envoylistener.ListenerFilter, error) {
	return &envoylistener.ListenerFilter{Name: wellknown.TlsInspector}, nil
}

func makeSNIFilterChainMatch(sniMatch string) *envoylistener.FilterChainMatch {
	return &envoylistener.FilterChainMatch{
		ServerNames: []string{sniMatch},
	}
}

func makeSNIClusterFilter() (*envoylistener.Filter, error) {
	// This filter has no config which is why we are not calling make
	return &envoylistener.Filter{Name: "envoy.filters.network.sni_cluster"}, nil
}

func makeTCPProxyFilter(filterName, cluster, statPrefix string) (*envoylistener.Filter, error) {
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
	filterName, cluster, statPrefix, routePath string,
	ingress, grpc, http2 bool,
) (*envoylistener.Filter, error) {
	op := envoyhttp.HttpConnectionManager_Tracing_INGRESS
	if !ingress {
		op = envoyhttp.HttpConnectionManager_Tracing_EGRESS
	}
	proto := "http"
	if grpc {
		proto = "grpc"
	}

	cfg := &envoyhttp.HttpConnectionManager{
		StatPrefix: makeStatPrefix(proto, statPrefix, filterName),
		CodecType:  envoyhttp.HttpConnectionManager_AUTO,
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
			return nil, fmt.Errorf("cannot specify cluster name when using RDS")
		}
		cfg.RouteSpecifier = &envoyhttp.HttpConnectionManager_Rds{
			Rds: &envoyhttp.Rds{
				RouteConfigName: filterName,
				ConfigSource: &envoycore.ConfigSource{
					ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
						Ads: &envoycore.AggregatedConfigSource{},
					},
				},
			},
		}
	} else {
		if cluster == "" {
			return nil, fmt.Errorf("must specify cluster name when not using RDS")
		}
		route := &envoyroute.Route{
			Match: &envoyroute.RouteMatch{
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
		}
		// If a path is provided, do not match on a catch-all prefix
		if routePath != "" {
			route.Match.PathSpecifier = &envoyroute.RouteMatch_Path{Path: routePath}
		}

		cfg.RouteSpecifier = &envoyhttp.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy.RouteConfiguration{
				Name: filterName,
				VirtualHosts: []*envoyroute.VirtualHost{
					{
						Name:    filterName,
						Domains: []string{"*"},
						Routes: []*envoyroute.Route{
							route,
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
		cfg.HttpFilters = append([]*envoyhttp.HttpFilter{{
			Name:       "envoy.grpc_http1_bridge",
			ConfigType: &envoyhttp.HttpFilter_Config{Config: &pbstruct.Struct{}},
		}}, cfg.HttpFilters...)
	}

	return makeFilter("envoy.http_connection_manager", cfg)
}

func makeExtAuthFilter(token string) (*envoylistener.Filter, error) {
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

func makeFilter(name string, cfg proto.Message) (*envoylistener.Filter, error) {
	// Ridiculous dance to make that struct into pbstruct.Struct by... encoding it
	// as JSON and decoding again!!
	cfgStruct, err := conversion.MessageToStruct(cfg)
	if err != nil {
		return nil, err
	}

	return &envoylistener.Filter{
		Name:       name,
		ConfigType: &envoylistener.Filter_Config{Config: cfgStruct},
	}, nil
}

func makeCommonTLSContextFromLeaf(cfgSnap *proxycfg.ConfigSnapshot, leaf *structs.IssuedCert) *envoyauth.CommonTlsContext {
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
						InlineString: leaf.CertPEM,
					},
				},
				PrivateKey: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_InlineString{
						InlineString: leaf.PrivateKeyPEM,
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

func makeCommonTLSContextFromFiles(caFile, certFile, keyFile string) *envoyauth.CommonTlsContext {
	ctx := envoyauth.CommonTlsContext{
		TlsParams: &envoyauth.TlsParameters{},
	}

	// Verify certificate of peer if caFile is specified
	if caFile != "" {
		ctx.ValidationContextType = &envoyauth.CommonTlsContext_ValidationContext{
			ValidationContext: &envoyauth.CertificateValidationContext{
				TrustedCa: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_Filename{
						Filename: caFile,
					},
				},
			},
		}
	}

	// Present certificate for mTLS if cert and key files are specified
	if certFile != "" && keyFile != "" {
		ctx.TlsCertificates = []*envoyauth.TlsCertificate{
			{
				CertificateChain: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_Filename{
						Filename: certFile,
					},
				},
				PrivateKey: &envoycore.DataSource{
					Specifier: &envoycore.DataSource_Filename{
						Filename: keyFile,
					},
				},
			},
		}
	}

	return &ctx
}

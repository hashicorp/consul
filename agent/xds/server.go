package xds

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_discovery_v2 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
)

var StatsGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"xds", "server", "streams"},
		Help: "Measures the number of active xDS streams handled by the server split by protocol version.",
	},
}

// ADSStream is a shorter way of referring to this thing...
type ADSStream = envoy_discovery_v3.AggregatedDiscoveryService_StreamAggregatedResourcesServer
type ADSStream_v2 = envoy_discovery_v2.AggregatedDiscoveryService_StreamAggregatedResourcesServer

const (
	// Resource types in xDS v3. These are copied from
	// envoyproxy/go-control-plane/pkg/resource/v3/resource.go since we don't need any of
	// the rest of that package.
	apiTypePrefix = "type.googleapis.com/"

	// EndpointType is the TypeURL for Endpoint discovery responses.
	EndpointType    = apiTypePrefix + "envoy.config.endpoint.v3.ClusterLoadAssignment"
	EndpointType_v2 = apiTypePrefix + "envoy.api.v2.ClusterLoadAssignment"

	// ClusterType is the TypeURL for Cluster discovery responses.
	ClusterType    = apiTypePrefix + "envoy.config.cluster.v3.Cluster"
	ClusterType_v2 = apiTypePrefix + "envoy.api.v2.Cluster"

	// RouteType is the TypeURL for Route discovery responses.
	RouteType    = apiTypePrefix + "envoy.config.route.v3.RouteConfiguration"
	RouteType_v2 = apiTypePrefix + "envoy.api.v2.RouteConfiguration"

	// ListenerType is the TypeURL for Listener discovery responses.
	ListenerType    = apiTypePrefix + "envoy.config.listener.v3.Listener"
	ListenerType_v2 = apiTypePrefix + "envoy.api.v2.Listener"

	// PublicListenerName is the name we give the public listener in Envoy config.
	PublicListenerName = "public_listener"

	// OutboundListenerName is the name we give the outbound Envoy listener when transparent proxy mode is enabled.
	OutboundListenerName = "outbound_listener"

	// LocalAppClusterName is the name we give the local application "cluster" in
	// Envoy config. Note that all cluster names may collide with service names
	// since we want cluster names and service names to match to enable nice
	// metrics correlation without massaging prefixes on cluster names.
	//
	// We should probably make this more unlikely to collide however changing it
	// potentially breaks upgrade compatibility without restarting all Envoy's as
	// it will no longer match their existing cluster name. Changing this will
	// affect metrics output so could break dashboards (for local app traffic).
	//
	// We should probably just make it configurable if anyone actually has
	// services named "local_app" in the future.
	LocalAppClusterName = "local_app"

	// LocalAgentClusterName is the name we give the local agent "cluster" in
	// Envoy config. Note that all cluster names may collide with service names
	// since we want cluster names and service names to match to enable nice
	// metrics correlation without massaging prefixes on cluster names.
	//
	// We should probably make this more unlikely to collied however changing it
	// potentially breaks upgrade compatibility without restarting all Envoy's as
	// it will no longer match their existing cluster name. Changing this will
	// affect metrics output so could break dashboards (for local agent traffic).
	//
	// We should probably just make it configurable if anyone actually has
	// services named "local_agent" in the future.
	LocalAgentClusterName = "local_agent"

	// OriginalDestinationClusterName is the name we give to the passthrough
	// cluster which redirects transparently-proxied requests to their original
	// destination outside the mesh. This cluster prevents Consul from blocking
	// connections to destinations outside of the catalog when in transparent
	// proxy mode.
	OriginalDestinationClusterName = "original-destination"

	// DefaultAuthCheckFrequency is the default value for
	// Server.AuthCheckFrequency to use when the zero value is provided.
	DefaultAuthCheckFrequency = 5 * time.Minute
)

// ACLResolverFunc is a shim to resolve ACLs. Since ACL enforcement is so far
// entirely agent-local and all uses private methods this allows a simple shim
// to be written in the agent package to allow resolving without tightly
// coupling this to the agent.
type ACLResolverFunc func(id string) (acl.Authorizer, error)

// ServiceChecks is the interface the agent needs to expose
// for the xDS server to fetch a service's HTTP check definitions
type HTTPCheckFetcher interface {
	ServiceHTTPBasedChecks(serviceID structs.ServiceID) []structs.CheckType
}

// ConfigFetcher is the interface the agent needs to expose
// for the xDS server to fetch agent config, currently only one field is fetched
type ConfigFetcher interface {
	AdvertiseAddrLAN() string
}

// ConfigManager is the interface xds.Server requires to consume proxy config
// updates. It's satisfied normally by the agent's proxycfg.Manager, but allows
// easier testing without several layers of mocked cache, local state and
// proxycfg.Manager.
type ConfigManager interface {
	Watch(proxyID structs.ServiceID) (<-chan *proxycfg.ConfigSnapshot, proxycfg.CancelFunc)
}

// Server represents a gRPC server that can handle xDS requests from Envoy. All
// of it's public members must be set before the gRPC server is started.
//
// A full description of the XDS protocol can be found at
// https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
type Server struct {
	Logger       hclog.Logger
	CfgMgr       ConfigManager
	ResolveToken ACLResolverFunc
	CheckFetcher HTTPCheckFetcher
	CfgFetcher   ConfigFetcher

	// AuthCheckFrequency is how often we should re-check the credentials used
	// during a long-lived gRPC Stream after it has been initially established.
	// This is only used during idle periods of stream interactions (i.e. when
	// there has been no recent DiscoveryRequest).
	AuthCheckFrequency time.Duration

	DisableV2Protocol bool

	activeStreams activeStreamCounters
}

// activeStreamCounters simply encapsulates two counters accessed atomically to
// ensure alignment is correct.
type activeStreamCounters struct {
	xDSv3 uint64
	xDSv2 uint64
}

func (c *activeStreamCounters) Increment(xdsVersion string) func() {
	var counter *uint64
	switch xdsVersion {
	case "v3":
		counter = &c.xDSv3
	case "v2":
		counter = &c.xDSv2
	default:
		return func() {}
	}

	labels := []metrics.Label{{Name: "version", Value: xdsVersion}}

	count := atomic.AddUint64(counter, 1)
	metrics.SetGaugeWithLabels([]string{"xds", "server", "streams"}, float32(count), labels)
	return func() {
		count := atomic.AddUint64(counter, ^uint64(0))
		metrics.SetGaugeWithLabels([]string{"xds", "server", "streams"}, float32(count), labels)
	}
}

func NewServer(
	logger hclog.Logger,
	cfgMgr ConfigManager,
	resolveToken ACLResolverFunc,
	checkFetcher HTTPCheckFetcher,
	cfgFetcher ConfigFetcher,
) *Server {
	return &Server{
		Logger:             logger,
		CfgMgr:             cfgMgr,
		ResolveToken:       resolveToken,
		CheckFetcher:       checkFetcher,
		CfgFetcher:         cfgFetcher,
		AuthCheckFrequency: DefaultAuthCheckFrequency,
	}
}

// StreamAggregatedResources implements
// envoy_discovery_v3.AggregatedDiscoveryServiceServer. This is the ADS endpoint which is
// the only xDS API we directly support for now.
//
// Deprecated: use DeltaAggregatedResources instead
func (s *Server) StreamAggregatedResources(stream ADSStream) error {
	return errors.New("not implemented")
}

// Deprecated: remove when xDS v2 is no longer supported
func (s *Server) streamAggregatedResources(stream ADSStream) error {
	defer s.activeStreams.Increment("v2")()

	// Note: despite dealing entirely in v3 protobufs, this function is
	// exclusively used from the xDS v2 shim RPC handler, so the logging below
	// will refer to it as "v2".

	// a channel for receiving incoming requests
	reqCh := make(chan *envoy_discovery_v3.DiscoveryRequest)
	reqStop := int32(0)
	go func() {
		for {
			req, err := stream.Recv()
			if atomic.LoadInt32(&reqStop) != 0 {
				return
			}
			if err != nil {
				close(reqCh)
				return
			}
			reqCh <- req
		}
	}()

	err := s.process(stream, reqCh)
	if err != nil {
		s.Logger.Error("Error handling ADS stream", "xdsVersion", "v2", "error", err)
	}

	// prevents writing to a closed channel if send failed on blocked recv
	atomic.StoreInt32(&reqStop, 1)

	return err
}

const (
	stateInit int = iota
	statePendingInitialConfig
	stateRunning
)

// Deprecated: remove when xDS v2 is no longer supported
func (s *Server) process(stream ADSStream, reqCh <-chan *envoy_discovery_v3.DiscoveryRequest) error {
	// xDS requires a unique nonce to correlate response/request pairs
	var nonce uint64

	// xDS works with versions of configs. Internally we don't have a consistent
	// version. We could hash the config since versions don't have to be
	// ordered as far as I can tell, but it is cheaper to increment a counter
	// every time we observe a new config since the upstream proxycfg package only
	// delivers updates when there are actual changes.
	var configVersion uint64

	// Loop state
	var (
		cfgSnap     *proxycfg.ConfigSnapshot
		req         *envoy_discovery_v3.DiscoveryRequest
		node        *envoy_config_core_v3.Node
		ok          bool
		stateCh     <-chan *proxycfg.ConfigSnapshot
		watchCancel func()
		proxyID     structs.ServiceID
	)

	generator := newResourceGenerator(
		s.Logger.Named(logging.XDS).With("xdsVersion", "v2"),
		s.CheckFetcher,
		s.CfgFetcher,
		false,
	)

	// need to run a small state machine to get through initial authentication.
	var state = stateInit

	// Configure handlers for each type of request
	handlers := map[string]*xDSType{
		EndpointType: {
			generator: generator,
			typeURL:   EndpointType,
			stream:    stream,
		},
		ClusterType: {
			generator: generator,
			typeURL:   ClusterType,
			stream:    stream,
			allowEmptyFn: func(cfgSnap *proxycfg.ConfigSnapshot) bool {
				// Mesh, Ingress, and Terminating gateways are allowed to inform CDS of
				// no clusters.
				return cfgSnap.Kind == structs.ServiceKindMeshGateway ||
					cfgSnap.Kind == structs.ServiceKindTerminatingGateway ||
					cfgSnap.Kind == structs.ServiceKindIngressGateway
			},
		},
		RouteType: {
			generator: generator,
			typeURL:   RouteType,
			stream:    stream,
			allowEmptyFn: func(cfgSnap *proxycfg.ConfigSnapshot) bool {
				return cfgSnap.Kind == structs.ServiceKindIngressGateway
			},
		},
		ListenerType: {
			generator: generator,
			typeURL:   ListenerType,
			stream:    stream,
			allowEmptyFn: func(cfgSnap *proxycfg.ConfigSnapshot) bool {
				return cfgSnap.Kind == structs.ServiceKindIngressGateway
			},
		},
	}

	var authTimer <-chan time.Time
	extendAuthTimer := func() {
		authTimer = time.After(s.AuthCheckFrequency)
	}

	checkStreamACLs := func(cfgSnap *proxycfg.ConfigSnapshot) error {
		return s.authorize(stream.Context(), cfgSnap)
	}

	for {
		select {
		case <-authTimer:
			// It's been too long since a Discovery{Request,Response} so recheck ACLs.
			if err := checkStreamACLs(cfgSnap); err != nil {
				return err
			}
			extendAuthTimer()

		case req, ok = <-reqCh:
			if !ok {
				// reqCh is closed when stream.Recv errors which is how we detect client
				// going away. AFAICT the stream.Context() is only canceled once the
				// RPC method returns which it can't until we return from this one so
				// there's no point in blocking on that.
				return nil
			}

			generator.logTraceRequest("SOTW xDS v2", req)

			if req.TypeUrl == "" {
				return status.Errorf(codes.InvalidArgument, "type URL is required for ADS")
			}

			if node == nil && req.Node != nil {
				node = req.Node
				var err error
				generator.ProxyFeatures, err = determineSupportedProxyFeatures(req.Node)
				if err != nil {
					return status.Errorf(codes.InvalidArgument, err.Error())
				}
			}

			if handler, ok := handlers[req.TypeUrl]; ok {
				handler.Recv(req, node)
			}
		case cfgSnap = <-stateCh:
			// We got a new config, update the version counter
			configVersion++
		}

		// Trigger state machine
		switch state {
		case stateInit:
			if req == nil {
				// This can't happen (tm) since stateCh is nil until after the first req
				// is received but lets not panic about it.
				continue
			}
			// Start authentication process, we need the proxyID
			proxyID = structs.NewServiceID(req.Node.Id, parseEnterpriseMeta(req.Node))

			// Start watching config for that proxy
			stateCh, watchCancel = s.CfgMgr.Watch(proxyID)
			// Note that in this case we _intend_ the defer to only be triggered when
			// this whole process method ends (i.e. when streaming RPC aborts) not at
			// the end of the current loop iteration. We have to do it in the loop
			// here since we can't start watching until we get to this state in the
			// state machine.
			defer watchCancel()

			generator.Logger.Trace("watching proxy, pending initial proxycfg snapshot",
				"service_id", proxyID.String())

			// Now wait for the config so we can check ACL
			state = statePendingInitialConfig
		case statePendingInitialConfig:
			if cfgSnap == nil {
				// Nothing we can do until we get the initial config
				continue
			}

			// Got config, try to authenticate next.
			state = stateRunning

			// Upgrade the logger based on Kind.
			switch cfgSnap.Kind {
			case structs.ServiceKindConnectProxy:
			case structs.ServiceKindTerminatingGateway:
				generator.Logger = generator.Logger.Named(logging.TerminatingGateway)
			case structs.ServiceKindMeshGateway:
				generator.Logger = generator.Logger.Named(logging.MeshGateway)
			case structs.ServiceKindIngressGateway:
				generator.Logger = generator.Logger.Named(logging.IngressGateway)
			}

			generator.Logger.Trace("Got initial config snapshot",
				"service_id", cfgSnap.ProxyID.String())

			// Lets actually process the config we just got or we'll mis responding
			fallthrough
		case stateRunning:
			// Check ACLs on every Discovery{Request,Response}.
			if err := checkStreamACLs(cfgSnap); err != nil {
				return err
			}
			// For the first time through the state machine, this is when the
			// timer is first started.
			extendAuthTimer()

			generator.Logger.Trace("Invoking all xDS resource handlers and sending new data if there is any",
				"service_id", cfgSnap.ProxyID.String())

			// See if any handlers need to have the current (possibly new) config
			// sent. Note the order here is actually significant so we can't just
			// range the map which has no determined order. It's important because:
			//
			//  1. Envoy needs to see a consistent snapshot to avoid potentially
			//     dropping traffic due to inconsistencies. This is the
			//     main win of ADS after all - we get to control this order.
			//  2. Non-determinsic order of complex protobuf responses which are
			//     compared for non-exact JSON equivalence makes the tests uber-messy
			//     to handle
			for _, typeURL := range []string{ClusterType, EndpointType, RouteType, ListenerType} {
				handler := handlers[typeURL]
				if err := handler.SendIfNew(cfgSnap, configVersion, &nonce); err != nil {
					return status.Errorf(codes.Unavailable,
						"failed to send reply for type %q: %v",
						typeURL, err)
				}
			}
		}
	}
}

// Deprecated: remove when xDS v2 is no longer supported
type xDSType struct {
	generator *ResourceGenerator
	typeURL   string
	stream    ADSStream
	req       *envoy_discovery_v3.DiscoveryRequest
	node      *envoy_config_core_v3.Node
	lastNonce string
	// lastVersion is the version that was last sent to the proxy. It is needed
	// because we don't want to send the same version more than once.
	// req.VersionInfo may be an older version than the most recent once sent in
	// two cases: 1) if the ACK wasn't received yet and `req` still points to the
	// previous request we already responded to and 2) if the proxy rejected the
	// last version we sent with a Nack then req.VersionInfo will be the older
	// version it's hanging on to.
	lastVersion  uint64
	allowEmptyFn func(cfgSnap *proxycfg.ConfigSnapshot) bool
}

func (t *xDSType) Recv(req *envoy_discovery_v3.DiscoveryRequest, node *envoy_config_core_v3.Node) {
	if t.lastNonce == "" || t.lastNonce == req.GetResponseNonce() {
		t.req = req
		t.node = node
	}
}

func (t *xDSType) SendIfNew(cfgSnap *proxycfg.ConfigSnapshot, version uint64, nonce *uint64) error {
	if t.req == nil {
		return nil
	}
	if t.lastVersion >= version {
		// Already sent this version
		return nil
	}

	resources, err := t.generator.resourcesFromSnapshot(t.typeURL, cfgSnap)
	if err != nil {
		return err
	}

	allowEmpty := t.allowEmptyFn != nil && t.allowEmptyFn(cfgSnap)

	// Zero length resource responses should be ignored and are the result of no
	// data yet. Notice that this caused a bug originally where we had zero
	// healthy endpoints for an upstream that would cause Envoy to hang waiting
	// for the EDS response. This is fixed though by ensuring we send an explicit
	// empty LoadAssignment resource for the cluster rather than allowing junky
	// empty resources.
	if len(resources) == 0 && !allowEmpty {
		// Nothing to send yet
		return nil
	}

	// Note we only increment nonce when we actually send - not important for
	// correctness but makes tests much simpler when we skip a type like Routes
	// with nothing to send.
	*nonce++
	nonceStr := fmt.Sprintf("%08x", *nonce)
	versionStr := fmt.Sprintf("%08x", version)

	resp, err := createResponse(t.typeURL, versionStr, nonceStr, resources)
	if err != nil {
		return err
	}

	t.generator.logTraceResponse("SOTW xDS v2", resp)

	err = t.stream.Send(resp)
	if err != nil {
		return err
	}
	t.lastVersion = version
	t.lastNonce = nonceStr
	return nil
}

func tokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	toks, ok := md["x-consul-token"]
	if ok && len(toks) > 0 {
		return toks[0]
	}
	return ""
}

// newPanicHandler returns a recovery.RecoveryHandlerFuncContext closure function
// to handle panic in GRPC server's handlers.
func newPanicHandler(logger hclog.Logger) recovery.RecoveryHandlerFuncContext {
	return func(ctx context.Context, p interface{}) (err error) {
		// Log the panic and the stack trace of the Goroutine that caused the panic.
		stacktrace := hclog.Stacktrace()
		logger.Error("panic serving grpc request",
			"panic", p,
			"stack", stacktrace,
		)

		return status.Errorf(codes.Internal, "grpc: panic serving request: %v", p)
	}
}

// NewGRPCServer creates a grpc.Server, registers the Server, and then returns
// the grpc.Server.
func NewGRPCServer(s *Server, tlsConfigurator *tlsutil.Configurator) *grpc.Server {
  recoveryOpts := []recovery.Option{
		recovery.WithRecoveryHandlerContext(newPanicHandler(s.Logger)),
	}

	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(2048),
		middleware.WithUnaryServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.UnaryServerInterceptor(recoveryOpts...),
		),
		middleware.WithStreamServerChain(
			// Add middlware interceptors to recover in case of panics.
			recovery.StreamServerInterceptor(recoveryOpts...),
		),
	}
	if tlsConfigurator != nil {
		if tlsConfigurator.Cert() != nil {
			creds := credentials.NewTLS(tlsConfigurator.IncomingXDSConfig())
			opts = append(opts, grpc.Creds(creds))
		}
	}
	srv := grpc.NewServer(opts...)
	envoy_discovery_v3.RegisterAggregatedDiscoveryServiceServer(srv, s)

	if !s.DisableV2Protocol {
		envoy_discovery_v2.RegisterAggregatedDiscoveryServiceServer(srv, &adsServerV2Shim{srv: s})
	}
	return srv
}

// authorize the xDS request using the token stored in ctx. This authorization is
// a bit different from most interfaces. Instead of explicitly authorizing or
// filtering each piece of data in the response, the request is authorized
// by checking the token has `service:write` for the service ID of the destination
// service (for kind=ConnectProxy), or the gateway service (for other kinds).
// This authorization strategy requires that agent/proxycfg only fetches data
// using a token with the same permissions, and that it stores the data by
// proxy ID. We assume that any data in the snapshot was already filtered,
// which allows this authorization to be a shallow authorization check
// for all the data in a ConfigSnapshot.
func (s *Server) authorize(ctx context.Context, cfgSnap *proxycfg.ConfigSnapshot) error {
	if cfgSnap == nil {
		return status.Errorf(codes.Unauthenticated, "unauthenticated: no config snapshot")
	}

	authz, err := s.ResolveToken(tokenFromContext(ctx))
	if acl.IsErrNotFound(err) {
		return status.Errorf(codes.Unauthenticated, "unauthenticated: %v", err)
	} else if acl.IsErrPermissionDenied(err) {
		return status.Errorf(codes.PermissionDenied, "permission denied: %v", err)
	} else if err != nil {
		return status.Errorf(codes.Internal, "error resolving acl token: %v", err)
	}

	var authzContext acl.AuthorizerContext
	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		cfgSnap.ProxyID.EnterpriseMeta.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(cfgSnap.Proxy.DestinationServiceName, &authzContext) != acl.Allow {
			return status.Errorf(codes.PermissionDenied, "permission denied")
		}
	case structs.ServiceKindMeshGateway, structs.ServiceKindTerminatingGateway, structs.ServiceKindIngressGateway:
		cfgSnap.ProxyID.EnterpriseMeta.FillAuthzContext(&authzContext)
		if authz.ServiceWrite(cfgSnap.Service, &authzContext) != acl.Allow {
			return status.Errorf(codes.PermissionDenied, "permission denied")
		}
	default:
		return status.Errorf(codes.Internal, "Invalid service kind")
	}

	// Authed OK!
	return nil
}

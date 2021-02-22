package xds

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/consul/logging"
	"sync/atomic"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoydisco "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
)

// ADSStream is a shorter way of referring to this thing...
type ADSStream = envoydisco.AggregatedDiscoveryService_StreamAggregatedResourcesServer

const (
	// Resource types in xDS v2. These are copied from
	// envoyproxy/go-control-plane/pkg/cache/resource.go since we don't need any of
	// the rest of that package.
	typePrefix = "type.googleapis.com/envoy.api.v2."

	// EndpointType is the TypeURL for Endpoint discovery responses.
	EndpointType = typePrefix + "ClusterLoadAssignment"

	// ClusterType is the TypeURL for Cluster discovery responses.
	ClusterType = typePrefix + "Cluster"

	// RouteType is the TypeURL for Route discovery responses.
	RouteType = typePrefix + "RouteConfiguration"

	// ListenerType is the TypeURL for Listener discovery responses.
	ListenerType = typePrefix + "Listener"

	// PublicListenerName is the name we give the public listener in Envoy config.
	PublicListenerName = "public_listener"

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
	// AuthCheckFrequency is how often we should re-check the credentials used
	// during a long-lived gRPC Stream after it has been initially established.
	// This is only used during idle periods of stream interactions (i.e. when
	// there has been no recent DiscoveryRequest).
	AuthCheckFrequency time.Duration
	CheckFetcher       HTTPCheckFetcher
	CfgFetcher         ConfigFetcher
}

// StreamAggregatedResources implements
// envoydisco.AggregatedDiscoveryServiceServer. This is the ADS endpoint which is
// the only xDS API we directly support for now.
func (s *Server) StreamAggregatedResources(stream ADSStream) error {
	// a channel for receiving incoming requests
	reqCh := make(chan *envoy.DiscoveryRequest)
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
		s.Logger.Error("Error handling ADS stream", "error", err)
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

func (s *Server) process(stream ADSStream, reqCh <-chan *envoy.DiscoveryRequest) error {
	logger := s.Logger.Named(logging.XDS)

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
		cfgSnap       *proxycfg.ConfigSnapshot
		req           *envoy.DiscoveryRequest
		node          *envoycore.Node
		proxyFeatures supportedProxyFeatures
		ok            bool
		stateCh       <-chan *proxycfg.ConfigSnapshot
		watchCancel   func()
		proxyID       structs.ServiceID
	)

	// need to run a small state machine to get through initial authentication.
	var state = stateInit

	// Configure handlers for each type of request
	handlers := map[string]*xDSType{
		EndpointType: {
			typeURL:   EndpointType,
			resources: s.endpointsFromSnapshot,
			stream:    stream,
		},
		ClusterType: {
			typeURL:   ClusterType,
			resources: s.clustersFromSnapshot,
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
			typeURL:   RouteType,
			resources: s.routesFromSnapshot,
			stream:    stream,
			allowEmptyFn: func(cfgSnap *proxycfg.ConfigSnapshot) bool {
				return cfgSnap.Kind == structs.ServiceKindIngressGateway
			},
		},
		ListenerType: {
			typeURL:   ListenerType,
			resources: s.listenersFromSnapshot,
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
		if cfgSnap == nil {
			return status.Errorf(codes.Unauthenticated, "unauthenticated: no config snapshot")
		}

		rule, err := s.ResolveToken(tokenFromContext(stream.Context()))

		if acl.IsErrNotFound(err) {
			return status.Errorf(codes.Unauthenticated, "unauthenticated: %v", err)
		} else if acl.IsErrPermissionDenied(err) {
			return status.Errorf(codes.PermissionDenied, "permission denied: %v", err)
		} else if err != nil {
			return err
		}

		var authzContext acl.AuthorizerContext
		switch cfgSnap.Kind {
		case structs.ServiceKindConnectProxy:
			cfgSnap.ProxyID.EnterpriseMeta.FillAuthzContext(&authzContext)
			if rule != nil && rule.ServiceWrite(cfgSnap.Proxy.DestinationServiceName, &authzContext) != acl.Allow {
				return status.Errorf(codes.PermissionDenied, "permission denied")
			}
		case structs.ServiceKindMeshGateway, structs.ServiceKindTerminatingGateway, structs.ServiceKindIngressGateway:
			cfgSnap.ProxyID.EnterpriseMeta.FillAuthzContext(&authzContext)
			if rule != nil && rule.ServiceWrite(cfgSnap.Service, &authzContext) != acl.Allow {
				return status.Errorf(codes.PermissionDenied, "permission denied")
			}
		default:
			return status.Errorf(codes.Internal, "Invalid service kind")
		}

		// Authed OK!
		return nil
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
			if req.TypeUrl == "" {
				return status.Errorf(codes.InvalidArgument, "type URL is required for ADS")
			}

			if node == nil && req.Node != nil {
				node = req.Node
				var err error
				proxyFeatures, err = determineSupportedProxyFeatures(req.Node)
				if err != nil {
					return status.Errorf(codes.InvalidArgument, err.Error())
				}
			}

			if handler, ok := handlers[req.TypeUrl]; ok {
				handler.Recv(req, node, proxyFeatures)
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

			logger.Trace("watching proxy, pending initial proxycfg snapshot",
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

			logger.Trace("Got initial config snapshot",
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

			logger.Trace("Invoking all xDS resource handlers and sending new data if there is any",
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
					return err
				}
			}
		}
	}
}

type xDSType struct {
	typeURL       string
	stream        ADSStream
	req           *envoy.DiscoveryRequest
	node          *envoycore.Node
	proxyFeatures supportedProxyFeatures
	lastNonce     string
	// lastVersion is the version that was last sent to the proxy. It is needed
	// because we don't want to send the same version more than once.
	// req.VersionInfo may be an older version than the most recent once sent in
	// two cases: 1) if the ACK wasn't received yet and `req` still points to the
	// previous request we already responded to and 2) if the proxy rejected the
	// last version we sent with a Nack then req.VersionInfo will be the older
	// version it's hanging on to.
	lastVersion  uint64
	resources    func(cInfo connectionInfo, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error)
	allowEmptyFn func(cfgSnap *proxycfg.ConfigSnapshot) bool
}

// connectionInfo represents details specific to this connection
type connectionInfo struct {
	Token         string
	ProxyFeatures supportedProxyFeatures
}

func (t *xDSType) Recv(req *envoy.DiscoveryRequest, node *envoycore.Node, proxyFeatures supportedProxyFeatures) {
	if t.lastNonce == "" || t.lastNonce == req.GetResponseNonce() {
		t.req = req
		t.node = node
		t.proxyFeatures = proxyFeatures
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

	cInfo := connectionInfo{
		Token:         tokenFromContext(t.stream.Context()),
		ProxyFeatures: t.proxyFeatures,
	}
	resources, err := t.resources(cInfo, cfgSnap)
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

// DeltaAggregatedResources implements envoydisco.AggregatedDiscoveryServiceServer
func (s *Server) DeltaAggregatedResources(_ envoydisco.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return errors.New("not implemented")
}

// GRPCServer returns a server instance that can handle xDS requests.
func (s *Server) GRPCServer(tlsConfigurator *tlsutil.Configurator) (*grpc.Server, error) {
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(2048),
	}
	if tlsConfigurator != nil {
		if tlsConfigurator.Cert() != nil {
			creds := credentials.NewTLS(tlsConfigurator.IncomingGRPCConfig())
			opts = append(opts, grpc.Creds(creds))
		}
	}
	srv := grpc.NewServer(opts...)
	envoydisco.RegisterAggregatedDiscoveryServiceServer(srv, s)

	return srv, nil
}

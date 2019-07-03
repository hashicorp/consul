package xds

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauthz "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2"
	envoyauthzalpha "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2alpha"
	envoydisco "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/gogo/googleapis/google/rpc"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
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

// ConnectAuthz is the interface the agent needs to expose to be able to re-use
// the authorization logic between both APIs.
type ConnectAuthz interface {
	// ConnectAuthorize is implemented by Agent.ConnectAuthorize
	ConnectAuthorize(token string, req *structs.ConnectAuthorizeRequest) (authz bool, reason string, m *cache.ResultMeta, err error)
}

// ConfigManager is the interface xds.Server requires to consume proxy config
// updates. It's satisfied normally by the agent's proxycfg.Manager, but allows
// easier testing without several layers of mocked cache, local state and
// proxycfg.Manager.
type ConfigManager interface {
	Watch(proxyID string) (<-chan *proxycfg.ConfigSnapshot, proxycfg.CancelFunc)
}

// Server represents a gRPC server that can handle both XDS and ext_authz
// requests from Envoy. All of it's public members must be set before the gRPC
// server is started.
//
// A full description of the XDS protocol can be found at
// https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
type Server struct {
	Logger       *log.Logger
	CfgMgr       ConfigManager
	Authz        ConnectAuthz
	ResolveToken ACLResolverFunc
	// AuthCheckFrequency is how often we should re-check the credentials used
	// during a long-lived gRPC Stream after it has been initially established.
	// This is only used during idle periods of stream interactions (i.e. when
	// there has been no recent DiscoveryRequest).
	AuthCheckFrequency time.Duration
}

// Initialize will finish configuring the Server for first use.
func (s *Server) Initialize() {
	if s.AuthCheckFrequency == 0 {
		s.AuthCheckFrequency = DefaultAuthCheckFrequency
	}
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
		s.Logger.Printf("[DEBUG] Error handling ADS stream: %s", err)
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
	// xDS requires a unique nonce to correlate response/request pairs
	var nonce uint64

	// xDS works with versions of configs. Internally we don't have a consistent
	// version. We could just hash the config since versions don't have to be
	// ordered as far as I can tell, but it's cheaper just to increment a counter
	// every time we observe a new config since the upstream proxycfg package only
	// delivers updates when there are actual changes.
	var configVersion uint64

	// Loop state
	var cfgSnap *proxycfg.ConfigSnapshot
	var req *envoy.DiscoveryRequest
	var ok bool
	var stateCh <-chan *proxycfg.ConfigSnapshot
	var watchCancel func()
	var proxyID string

	// need to run a small state machine to get through initial authentication.
	var state = stateInit

	// Configure handlers for each type of request
	handlers := map[string]*xDSType{
		EndpointType: &xDSType{
			typeURL:   EndpointType,
			resources: s.endpointsFromSnapshot,
			stream:    stream,
		},
		ClusterType: &xDSType{
			typeURL:   ClusterType,
			resources: s.clustersFromSnapshot,
			stream:    stream,
		},
		RouteType: &xDSType{
			typeURL:   RouteType,
			resources: routesFromSnapshot,
			stream:    stream,
		},
		ListenerType: &xDSType{
			typeURL:   ListenerType,
			resources: s.listenersFromSnapshot,
			stream:    stream,
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

		token := tokenFromStream(stream)
		rule, err := s.ResolveToken(token)

		if acl.IsErrNotFound(err) {
			return status.Errorf(codes.Unauthenticated, "unauthenticated: %v", err)
		} else if acl.IsErrPermissionDenied(err) {
			return status.Errorf(codes.PermissionDenied, "permission denied: %v", err)
		} else if err != nil {
			return err
		}

		switch cfgSnap.Kind {
		case structs.ServiceKindConnectProxy:
			if rule != nil && !rule.ServiceWrite(cfgSnap.Proxy.DestinationServiceName, nil) {
				return status.Errorf(codes.PermissionDenied, "permission denied")
			}
		case structs.ServiceKindMeshGateway:
			// TODO (mesh-gateway) - figure out what ACLs to check for the Gateways
			if rule != nil && !rule.ServiceWrite(cfgSnap.Service, nil) {
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
			if handler, ok := handlers[req.TypeUrl]; ok {
				handler.Recv(req)
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
			proxyID = req.Node.Id

			// Start watching config for that proxy
			stateCh, watchCancel = s.CfgMgr.Watch(proxyID)
			// Note that in this case we _intend_ the defer to only be triggered when
			// this whole process method ends (i.e. when streaming RPC aborts) not at
			// the end of the current loop iteration. We have to do it in the loop
			// here since we can't start watching until we get to this state in the
			// state machine.
			defer watchCancel()

			// Now wait for the config so we can check ACL
			state = statePendingInitialConfig
		case statePendingInitialConfig:
			if cfgSnap == nil {
				// Nothing we can do until we get the initial config
				continue
			}

			// Got config, try to authenticate next.
			state = stateRunning

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
	typeURL   string
	stream    ADSStream
	req       *envoy.DiscoveryRequest
	lastNonce string
	// lastVersion is the version that was last sent to the proxy. It is needed
	// because we don't want to send the same version more than once.
	// req.VersionInfo may be an older version than the most recent once sent in
	// two cases: 1) if the ACK wasn't received yet and `req` still points to the
	// previous request we already responded to and 2) if the proxy rejected the
	// last version we sent with a Nack then req.VersionInfo will be the older
	// version it's hanging on to.
	lastVersion uint64
	resources   func(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error)
}

func (t *xDSType) Recv(req *envoy.DiscoveryRequest) {
	if t.lastNonce == "" || t.lastNonce == req.GetResponseNonce() {
		t.req = req
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
	resources, err := t.resources(cfgSnap, tokenFromStream(t.stream))
	if err != nil {
		return err
	}
	// Zero length resource responses should be ignored and are the result of no
	// data yet. Notice that this caused a bug originally where we had zero
	// healthy endpoints for an upstream that would cause Envoy to hang waiting
	// for the EDS response. This is fixed though by ensuring we send an explicit
	// empty LoadAssignment resource for the cluster rather than allowing junky
	// empty resources.
	if len(resources) == 0 {
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

func tokenFromStream(stream ADSStream) string {
	return tokenFromContext(stream.Context())
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

func deniedResponse(reason string) (*envoyauthz.CheckResponse, error) {
	return &envoyauthz.CheckResponse{
		Status: &rpc.Status{
			Code:    int32(rpc.PERMISSION_DENIED),
			Message: "Denied: " + reason,
		},
	}, nil
}

// Check implements envoyauthz.AuthorizationServer.
func (s *Server) Check(ctx context.Context, r *envoyauthz.CheckRequest) (*envoyauthz.CheckResponse, error) {
	// Sanity checks
	if r.Attributes == nil || r.Attributes.Source == nil || r.Attributes.Destination == nil {
		return nil, status.Error(codes.InvalidArgument, "source and destination attributes are required")
	}
	if r.Attributes.Source.Principal == "" || r.Attributes.Destination.Principal == "" {
		return nil, status.Error(codes.InvalidArgument, "source and destination Principal are required")
	}

	// Parse destination to know the target service
	dest, err := connect.ParseCertURIFromString(r.Attributes.Destination.Principal)
	if err != nil {
		s.Logger.Printf("[DEBUG] grpc: Connect AuthZ DENIED: bad destination URI: src=%s dest=%s",
			r.Attributes.Source.Principal, r.Attributes.Destination.Principal)
		// Treat this as an auth error since Envoy has sent something it considers
		// valid, it's just not an identity we trust.
		return deniedResponse("Destination Principal is not a valid Connect identity")
	}

	destID, ok := dest.(*connect.SpiffeIDService)
	if !ok {
		s.Logger.Printf("[DEBUG] grpc: Connect AuthZ DENIED: bad destination service ID: src=%s dest=%s",
			r.Attributes.Source.Principal, r.Attributes.Destination.Principal)
		return deniedResponse("Destination Principal is not a valid Service identity")
	}

	// For now we don't validate the trust domain of the _destination_ at all -
	// the HTTP Authorize endpoint just accepts a target _service_ and it's
	// implicit that the request is for the correct cluster. We might want to
	// reconsider this later but plumbing in additional machinery to check the
	// clusterID here is not really necessary for now unless Envoys are badly
	// configured. Our threat model _requires_ correctly configured and well
	// behaved proxies given that they have ACLs to fetch certs and so can do
	// whatever they want including not authorizing traffic at all or routing it
	// do a different service than they auth'd against.

	// Create an authz request
	req := &structs.ConnectAuthorizeRequest{
		Target:        destID.Service,
		ClientCertURI: r.Attributes.Source.Principal,
		// TODO(banks): need Envoy to support sending cert serial/hash to enforce
		// revocation later.
	}
	token := tokenFromContext(ctx)
	authed, reason, _, err := s.Authz.ConnectAuthorize(token, req)
	if err != nil {
		if err == acl.ErrPermissionDenied {
			s.Logger.Printf("[DEBUG] grpc: Connect AuthZ failed ACL check: %s: src=%s dest=%s",
				err, r.Attributes.Source.Principal, r.Attributes.Destination.Principal)
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		s.Logger.Printf("[DEBUG] grpc: Connect AuthZ failed: %s: src=%s dest=%s",
			err, r.Attributes.Source.Principal, r.Attributes.Destination.Principal)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !authed {
		s.Logger.Printf("[DEBUG] grpc: Connect AuthZ DENIED: src=%s dest=%s reason=%s",
			r.Attributes.Source.Principal, r.Attributes.Destination.Principal, reason)
		return deniedResponse(reason)
	}

	s.Logger.Printf("[DEBUG] grpc: Connect AuthZ ALLOWED: src=%s dest=%s reason=%s",
		r.Attributes.Source.Principal, r.Attributes.Destination.Principal, reason)
	return &envoyauthz.CheckResponse{
		Status: &rpc.Status{
			Code:    int32(rpc.OK),
			Message: "ALLOWED: " + reason,
		},
	}, nil
}

// GRPCServer returns a server instance that can handle XDS and ext_authz
// requests.
func (s *Server) GRPCServer(certFile, keyFile string) (*grpc.Server, error) {
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(2048),
	}
	if certFile != "" && keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.Creds(creds))
	}
	srv := grpc.NewServer(opts...)
	envoydisco.RegisterAggregatedDiscoveryServiceServer(srv, s)

	// Envoy 1.10 changed the package for ext_authz from v2alpha to v2. We still
	// need to be compatible with 1.9.1 and earlier which only uses v2alpha. While
	// there is a deprecated compatibility shim option in 1.10, we want to support
	// first class. Fortunately they are wire-compatible so we can just register a
	// single service implementation (using the new v2 package definitions) but
	// using the old v2alpha regiatration function which just exports it on the
	// old path as well.
	envoyauthz.RegisterAuthorizationServer(srv, s)
	envoyauthzalpha.RegisterAuthorizationServer(srv, s)

	return srv, nil
}

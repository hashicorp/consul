package xds

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	authz "github.com/envoyproxy/go-control-plane/envoy/service/auth/v2alpha"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/gogo/googleapis/google/rpc"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

// ADSStream is a shorter way of referring to this thing...
type ADSStream = discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer

// Resource types in xDS v2. These are copied from
// envoyproxy/go-control-plane/pkg/cache/resource.go since we don't need any of
// the rest of that package.
const (
	typePrefix   = "type.googleapis.com/envoy.api.v2."
	EndpointType = typePrefix + "ClusterLoadAssignment"
	ClusterType  = typePrefix + "Cluster"
	RouteType    = typePrefix + "RouteConfiguration"
	ListenerType = typePrefix + "Listener"

	LocalAppClusterName   = "local_app"
	LocalAgentClusterName = "local_agent"
)

// ACLResolverFunc is a shim to resolve ACLs. Since ACL enforcement is so far
// entirely agent-local and all uses private methods this allows a simple shim
// to be written in the agent package to allow resolving without tightly
// coupling this to the agent.
type ACLResolverFunc func(id string) (acl.ACL, error)

// ConnectAuthz is the interface the agent needs to expose to be able to re-use
// the authorization logic between both APIs.
type ConnectAuthz interface {
	// ConnectAuthorize is implemented by Agent.ConnectAuthorize
	ConnectAuthorize(token string, req *structs.ConnectAuthorizeRequest) (authz bool, reason string, m *cache.ResultMeta, err error)
}

// ConfigManager is the interface the config manager implements to make testing
// the server without a full mocker manager, state and cache. In normal use this
// is satisfied directly by a pointer to the agent's proxycfg.Manager.
type ConfigManager interface {
	Watch(proxyID string) (<-chan *proxycfg.ConfigSnapshot, func())
}

// Server represents a gRPC server that can handle both XDS and ext_authz
// requests from Envoy.
//
// A full description of the XDS protocol can be found at
// https://github.com/envoyproxy/data-plane-api/blob/master/XDS_PROTOCOL.md
type Server struct {
	logger       *log.Logger
	cfgMgr       ConfigManager
	authz        ConnectAuthz
	resolveToken ACLResolverFunc
}

// NewServer creates a usable server instance.
func NewServer(logger *log.Logger, cfgMgr ConfigManager,
	aclFn ACLResolverFunc, authz ConnectAuthz) *Server {
	return &Server{
		logger:       logger,
		cfgMgr:       cfgMgr,
		authz:        authz,
		resolveToken: aclFn,
	}
}

// StreamAggregatedResources implements
// discovery.AggregatedDiscoveryServiceServer. This is the ADS endpoint which is
// the only xDS API we directly support for now.
func (s *Server) StreamAggregatedResources(stream ADSStream) error {
	// a channel for receiving incoming requests
	reqCh := make(chan *v2.DiscoveryRequest)
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
		s.logger.Printf("[DEBUG] Error handling ADS stream: %s", err)
	}

	// prevents writing to a closed channel if send failed on blocked recv
	atomic.StoreInt32(&reqStop, 1)

	return err
}

const (
	stateInit int = iota
	statePendingAuth
	stateRunning
)

func (s *Server) process(stream ADSStream, reqCh <-chan *v2.DiscoveryRequest) error {
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
	var req *v2.DiscoveryRequest
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
			resources: endpointsFromSnapshot,
			stream:    stream,
		},
		ClusterType: &xDSType{
			typeURL:   ClusterType,
			resources: clustersFromSnapshot,
			stream:    stream,
		},
		RouteType: &xDSType{
			typeURL:   RouteType,
			resources: routesFromSnapshot,
			stream:    stream,
		},
		ListenerType: &xDSType{
			typeURL:   ListenerType,
			resources: listenersFromSnapshot,
			stream:    stream,
		},
	}

	for {
		select {
		case <-stream.Context().Done():
			// TODO: Not sure if we should error here, this presumably happens during
			// a normal shutdown where client goes away too.
			return nil
		case req, ok = <-reqCh:
			if !ok {
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
				// is recieved but lets not panic about it.
				continue
			}
			// Start authentication process, we need the proxyID
			proxyID = req.Node.Id

			// Start watching config for that proxy
			stateCh, watchCancel = s.cfgMgr.Watch(proxyID)
			defer watchCancel()

			// Now wait for the config so we can check ACL
			state = statePendingAuth
		case statePendingAuth:
			if cfgSnap == nil {
				// Nothing we can do until we get the initial config
				continue
			}
			// Got config, try to authenticate
			token := tokenFromStream(stream)
			rule, err := s.resolveToken(token)
			if err != nil {
				return err
			}
			if rule != nil && !rule.ServiceWrite(cfgSnap.Proxy.DestinationServiceName, nil) {
				return status.Errorf(codes.PermissionDenied, "permission denied")
			}
			// Authed OK!
			state = stateRunning

			// Lets actually process the config we just got or we'll mis responding
			fallthrough
		case stateRunning:
			// See if any handlers need to have the current (possibly new) config
			// sent. Note the order here is actually significant so we can't just
			// range the map which has no determined order. It's important because:
			//
			//  1. Envoy needs to see a consistent snapshot to avoid potentially
			//     dropping or misrouting traffic due to inconsitencies. This is the
			//     main win of ADS after all - we get to control this order.
			//  2. Non-determinsic order of complex protobug responses which are
			//     compared for non-exact JSON equivalence makes the tests uber-messy
			//     to handle
			for _, typeURL := range []string{ClusterType, EndpointType, RouteType, ListenerType} {
				handler := handlers[typeURL]
				err := handler.SendIfNew(cfgSnap, configVersion, &nonce)
				if err != nil {
					return err
				}
			}
		}
	}
}

type xDSType struct {
	typeURL   string
	stream    ADSStream
	req       *v2.DiscoveryRequest
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

func (t *xDSType) Recv(req *v2.DiscoveryRequest) {
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
	if resources == nil || len(resources) == 0 {
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

// IncrementalAggregatedResources implements discovery.AggregatedDiscoveryServiceServer
func (s *Server) IncrementalAggregatedResources(_ discovery.AggregatedDiscoveryService_IncrementalAggregatedResourcesServer) error {
	return errors.New("not implemented")
}

func deniedResponse(reason string) (*authz.CheckResponse, error) {
	return &authz.CheckResponse{
		Status: &rpc.Status{
			Code:    int32(rpc.PERMISSION_DENIED),
			Message: "Denied: " + reason,
		},
	}, nil
}

// Check implementents authz.AuthorizationServer.
func (s *Server) Check(ctx context.Context, r *authz.CheckRequest) (*authz.CheckResponse, error) {
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
		// Treat this as an auth error since Envoy has sent something it considers
		// valid, it's just not an identity we trust.
		return deniedResponse("Destination Principal is not a valid Connect identitiy")
	}

	destID, ok := dest.(*connect.SpiffeIDService)
	if !ok {
		return deniedResponse("Destination Principal is not a valid Service identitiy")
	}

	// For now we don't validate the trust domain of the _destination_ at all -
	// the HTTP Authorize endpoint just accepts a target _service_ and it's
	// implicit that the request is for the correct cluster. We might want to
	// reconsider this later but plumbing in additional machinery to check the
	// clusterID here is not really necessary for now unless Envoys are badly
	// configured. Our threat model _requires_ correctly configured and well
	// behaved proxies given that they have ACLs to fetch certs and so can do
	// whatever they want including not authorizing traffic at all or routing it
	// do a different service than they authed against.

	// Create an authz request
	req := &structs.ConnectAuthorizeRequest{
		Target:        destID.Service,
		ClientCertURI: r.Attributes.Source.Principal,
		// TODO(banks): need Envoy to support sending cert serial/hash to enforce
		// revocation later.
	}
	token := tokenFromContext(ctx)
	authed, reason, _, err := s.authz.ConnectAuthorize(token, req)
	if err != nil {
		if err == acl.ErrPermissionDenied {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !authed {
		return deniedResponse(reason)
	}

	return &authz.CheckResponse{
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
	discovery.RegisterAggregatedDiscoveryServiceServer(srv, s)
	authz.RegisterAuthorizationServer(srv, s)
	return srv, nil
}

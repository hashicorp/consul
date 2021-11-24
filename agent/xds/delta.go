package xds

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

// ADSDeltaStream is a shorter way of referring to this thing...
type ADSDeltaStream = envoy_discovery_v3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer

// DeltaAggregatedResources implements envoy_discovery_v3.AggregatedDiscoveryServiceServer
func (s *Server) DeltaAggregatedResources(stream ADSDeltaStream) error {
	defer s.activeStreams.Increment("v3")()

	// a channel for receiving incoming requests
	reqCh := make(chan *envoy_discovery_v3.DeltaDiscoveryRequest)
	reqStop := int32(0)
	go func() {
		for {
			req, err := stream.Recv()
			if atomic.LoadInt32(&reqStop) != 0 {
				return
			}
			if err != nil {
				s.Logger.Error("Error receiving new DeltaDiscoveryRequest; closing request channel", "error", err)
				close(reqCh)
				return
			}
			reqCh <- req
		}
	}()

	err := s.processDelta(stream, reqCh)
	if err != nil {
		s.Logger.Error("Error handling ADS delta stream", "xdsVersion", "v3", "error", err)
	}

	// prevents writing to a closed channel if send failed on blocked recv
	atomic.StoreInt32(&reqStop, 1)

	return err
}

const (
	stateDeltaInit int = iota
	stateDeltaPendingInitialConfig
	stateDeltaRunning
)

func (s *Server) processDelta(stream ADSDeltaStream, reqCh <-chan *envoy_discovery_v3.DeltaDiscoveryRequest) error {
	// Loop state
	var (
		cfgSnap     *proxycfg.ConfigSnapshot
		node        *envoy_config_core_v3.Node
		stateCh     <-chan *proxycfg.ConfigSnapshot
		watchCancel func()
		proxyID     structs.ServiceID
		nonce       uint64 // xDS requires a unique nonce to correlate response/request pairs
		ready       bool   // set to true after the first snapshot arrives
	)

	var (
		// resourceMap is the SoTW we are incrementally attempting to sync to envoy.
		//
		// type => name => proto
		resourceMap = emptyIndexedResources()

		// currentVersions is the the xDS versioning represented by Resources.
		//
		// type => name => version (as consul knows right now)
		currentVersions = make(map[string]map[string]string)
	)

	generator := newResourceGenerator(
		s.Logger.Named(logging.XDS).With("xdsVersion", "v3"),
		s.CheckFetcher,
		s.CfgFetcher,
		true,
	)

	// need to run a small state machine to get through initial authentication.
	var state = stateDeltaInit

	// Configure handlers for each type of request we currently care about.
	handlers := map[string]*xDSDeltaType{
		ListenerType: newDeltaType(generator, stream, ListenerType, func(kind structs.ServiceKind) bool {
			return cfgSnap.Kind == structs.ServiceKindIngressGateway
		}),
		RouteType: newDeltaType(generator, stream, RouteType, func(kind structs.ServiceKind) bool {
			return cfgSnap.Kind == structs.ServiceKindIngressGateway
		}),
		ClusterType: newDeltaType(generator, stream, ClusterType, func(kind structs.ServiceKind) bool {
			// Mesh, Ingress, and Terminating gateways are allowed to inform CDS of
			// no clusters.
			return cfgSnap.Kind == structs.ServiceKindMeshGateway ||
				cfgSnap.Kind == structs.ServiceKindTerminatingGateway ||
				cfgSnap.Kind == structs.ServiceKindIngressGateway
		}),
		EndpointType: newDeltaType(generator, stream, EndpointType, nil),
	}

	// Endpoints are stored within a Cluster (and Routes
	// are stored within a Listener) so whenever the
	// enclosing resource is updated the inner resource
	// list is cleared implicitly.
	//
	// When this happens we should update our local
	// representation of envoy state to force an update.
	//
	// see: https://github.com/envoyproxy/envoy/issues/13009
	handlers[ListenerType].childType = handlers[RouteType]
	handlers[ClusterType].childType = handlers[EndpointType]

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

		case req, ok := <-reqCh:
			if !ok {
				// reqCh is closed when stream.Recv errors which is how we detect client
				// going away. AFAICT the stream.Context() is only canceled once the
				// RPC method returns which it can't until we return from this one so
				// there's no point in blocking on that.
				return nil
			}

			generator.logTraceRequest("Incremental xDS v3", req)

			if req.TypeUrl == "" {
				return status.Errorf(codes.InvalidArgument, "type URL is required for ADS")
			}

			if handler, ok := handlers[req.TypeUrl]; ok {
				if handler.Recv(req) {
					generator.Logger.Trace("subscribing to type", "typeUrl", req.TypeUrl)
				}
			}

			if node == nil && req.Node != nil {
				node = req.Node
				var err error
				generator.ProxyFeatures, err = determineSupportedProxyFeatures(req.Node)
				if err != nil {
					return status.Errorf(codes.InvalidArgument, err.Error())
				}
			}

		case cfgSnap = <-stateCh:
			newRes, err := generator.allResourcesFromSnapshot(cfgSnap)
			if err != nil {
				return status.Errorf(codes.Unavailable, "failed to generate all xDS resources from the snapshot: %v", err)
			}

			// index and hash the xDS structures
			newResourceMap := indexResources(generator.Logger, newRes)

			if s.ResourceMapMutateFn != nil {
				s.ResourceMapMutateFn(newResourceMap)
			}

			if err := populateChildIndexMap(newResourceMap); err != nil {
				return status.Errorf(codes.Unavailable, "failed to index xDS resource versions: %v", err)
			}

			newVersions, err := computeResourceVersions(newResourceMap)
			if err != nil {
				return status.Errorf(codes.Unavailable, "failed to compute xDS resource versions: %v", err)
			}

			resourceMap = newResourceMap
			currentVersions = newVersions
			ready = true
		}

		// Trigger state machine
		switch state {
		case stateDeltaInit:
			if node == nil {
				// This can't happen (tm) since stateCh is nil until after the first req
				// is received but lets not panic about it.
				continue
			}
			// Start authentication process, we need the proxyID
			proxyID = structs.NewServiceID(node.Id, parseEnterpriseMeta(node))

			// Start watching config for that proxy
			stateCh, watchCancel = s.CfgMgr.Watch(proxyID)
			// Note that in this case we _intend_ the defer to only be triggered when
			// this whole process method ends (i.e. when streaming RPC aborts) not at
			// the end of the current loop iteration. We have to do it in the loop
			// here since we can't start watching until we get to this state in the
			// state machine.
			defer watchCancel()

			generator.Logger = generator.Logger.With("service_id", proxyID.String()) // enhance future logs

			generator.Logger.Trace("watching proxy, pending initial proxycfg snapshot for xDS")

			// Now wait for the config so we can check ACL
			state = stateDeltaPendingInitialConfig
		case stateDeltaPendingInitialConfig:
			if cfgSnap == nil {
				// Nothing we can do until we get the initial config
				continue
			}

			// Got config, try to authenticate next.
			state = stateDeltaRunning

			// Upgrade the logger
			switch cfgSnap.Kind {
			case structs.ServiceKindConnectProxy:
			case structs.ServiceKindTerminatingGateway:
				generator.Logger = generator.Logger.Named(logging.TerminatingGateway)
			case structs.ServiceKindMeshGateway:
				generator.Logger = generator.Logger.Named(logging.MeshGateway)
			case structs.ServiceKindIngressGateway:
				generator.Logger = generator.Logger.Named(logging.IngressGateway)
			}

			generator.Logger.Trace("Got initial config snapshot")

			// Lets actually process the config we just got or we'll mis responding
			fallthrough
		case stateDeltaRunning:
			// Check ACLs on every Discovery{Request,Response}.
			if err := checkStreamACLs(cfgSnap); err != nil {
				return err
			}
			// For the first time through the state machine, this is when the
			// timer is first started.
			extendAuthTimer()

			if !ready {
				generator.Logger.Trace("Skipping delta computation because we haven't gotten a snapshot yet")
				continue
			}

			var pendingTypes []string
			for typeUrl, handler := range handlers {
				if !handler.registered {
					continue
				}
				if len(handler.pendingUpdates) > 0 {
					pendingTypes = append(pendingTypes, typeUrl)
				}
			}
			if len(pendingTypes) > 0 {
				sort.Strings(pendingTypes)
				generator.Logger.Trace("Skipping delta computation because there are responses in flight",
					"pendingTypeUrls", pendingTypes)
				continue
			}

			generator.Logger.Trace("Invoking all xDS resource handlers and sending changed data if there are any")

			sentType := make(map[string]struct{}) // use this to only do one kind of mutation per type per execution
			for _, op := range xDSUpdateOrder {
				if _, sent := sentType[op.TypeUrl]; sent {
					continue
				}
				err, sent := handlers[op.TypeUrl].SendIfNew(
					cfgSnap.Kind,
					currentVersions[op.TypeUrl],
					resourceMap,
					&nonce,
					op.Upsert,
					op.Remove,
				)
				if err != nil {
					return status.Errorf(codes.Unavailable,
						"failed to send %sreply for type %q: %v",
						op.errorLogNameReplyPrefix(),
						op.TypeUrl, err)
				}
				if sent {
					sentType[op.TypeUrl] = struct{}{}
				}
			}
		}
	}
}

var xDSUpdateOrder = []xDSUpdateOperation{
	// 1. CDS updates (if any) must always be pushed first.
	{TypeUrl: ClusterType, Upsert: true},
	// 2. EDS updates (if any) must arrive after CDS updates for the respective clusters.
	{TypeUrl: EndpointType, Upsert: true},
	// 3. LDS updates must arrive after corresponding CDS/EDS updates.
	{TypeUrl: ListenerType, Upsert: true, Remove: true},
	// 4. RDS updates related to the newly added listeners must arrive after CDS/EDS/LDS updates.
	{TypeUrl: RouteType, Upsert: true, Remove: true},
	// 5. (NOT IMPLEMENTED YET IN CONSUL) VHDS updates (if any) related to the newly added RouteConfigurations must arrive after RDS updates.
	// {},
	// 6. Stale CDS clusters and related EDS endpoints (ones no longer being referenced) can then be removed.
	{TypeUrl: ClusterType, Remove: true},
	{TypeUrl: EndpointType, Remove: true},
	// xDS updates can be pushed independently if no new
	// clusters/routes/listeners are added or if it’s acceptable to
	// temporarily drop traffic during updates. Note that in case of
	// LDS updates, the listeners will be warmed before they receive
	// traffic, i.e. the dependent routes are fetched through RDS if
	// configured. Clusters are warmed when adding/removing/updating
	// clusters. On the other hand, routes are not warmed, i.e., the
	// management plane must ensure that clusters referenced by a route
	// are in place, before pushing the updates for a route.
}

type xDSUpdateOperation struct {
	TypeUrl string
	Upsert  bool
	Remove  bool
}

func (op *xDSUpdateOperation) errorLogNameReplyPrefix() string {
	switch {
	case op.Upsert && op.Remove:
		return "upsert/remove "
	case op.Upsert:
		return "upsert "
	case op.Remove:
		return "remove "
	default:
		return ""
	}
}

type xDSDeltaType struct {
	generator    *ResourceGenerator
	stream       ADSDeltaStream
	typeURL      string
	allowEmptyFn func(kind structs.ServiceKind) bool

	// childType is a type that in Envoy is actually stored within this type.
	// Upserts of THIS type should potentially trigger dependent named
	// resources within the child to be re-configured.
	childType *xDSDeltaType

	// registered indicates if this type has been requested at least once by
	// the proxy
	registered bool

	// wildcard indicates that this type was requested with no preference for
	// specific resource names. subscribe/unsubscribe are ignored.
	wildcard bool

	// sentToEnvoyOnce is true after we've sent one response to envoy.
	sentToEnvoyOnce bool

	// subscriptions is the set of currently subscribed envoy resources.
	// If wildcard == true, this will be empty.
	subscriptions map[string]struct{}

	// resourceVersions is the current view of CONFIRMED/ACKed updates to
	// envoy's view of the loaded resources.
	//
	// name => version
	resourceVersions map[string]string

	// pendingUpdates is a set of un-ACKed updates to the 'resourceVersions'
	// map. Once we get an ACK from envoy we'll update the resourceVersions map
	// and strike the entry from this map.
	//
	// nonce -> name -> {version}
	pendingUpdates map[string]map[string]PendingUpdate
}

func (t *xDSDeltaType) subscribed(name string) bool {
	if t.wildcard {
		return true
	}
	_, subscribed := t.subscriptions[name]
	return subscribed
}

type PendingUpdate struct {
	Remove         bool
	Version        string
	ChildResources []string // optional
}

func newDeltaType(
	generator *ResourceGenerator,
	stream ADSDeltaStream,
	typeUrl string,
	allowEmptyFn func(kind structs.ServiceKind) bool,
) *xDSDeltaType {
	return &xDSDeltaType{
		generator:        generator,
		stream:           stream,
		typeURL:          typeUrl,
		allowEmptyFn:     allowEmptyFn,
		subscriptions:    make(map[string]struct{}),
		resourceVersions: make(map[string]string),
		pendingUpdates:   make(map[string]map[string]PendingUpdate),
	}
}

// Recv handles new discovery requests from envoy.
//
// Returns true the first time a type receives a request.
func (t *xDSDeltaType) Recv(req *envoy_discovery_v3.DeltaDiscoveryRequest) bool {
	if t == nil {
		return false // not something we care about
	}
	logger := t.generator.Logger.With("typeUrl", t.typeURL)

	registeredThisTime := false
	if !t.registered {
		// We are in the wildcard mode if the first request of a particular
		// type has empty subscription list
		t.wildcard = len(req.ResourceNamesSubscribe) == 0
		t.registered = true
		registeredThisTime = true
	}

	/*
		DeltaDiscoveryRequest can be sent in the following situations:

		Initial message in a xDS bidirectional gRPC stream.

		As an ACK or NACK response to a previous DeltaDiscoveryResponse. In
		this case the response_nonce is set to the nonce value in the Response.
		ACK or NACK is determined by the absence or presence of error_detail.

		Spontaneous DeltaDiscoveryRequests from the client. This can be done to
		dynamically add or remove elements from the tracked resource_names set.
		In this case response_nonce must be omitted.

	*/

	/*
		DeltaDiscoveryRequest plays two independent roles. Any
		DeltaDiscoveryRequest can be either or both of:
	*/

	if req.ResponseNonce != "" {
		/*
			[2] (N)ACKing an earlier resource update from the server (using
			response_nonce, with presence of error_detail making it a NACK).
		*/
		if req.ErrorDetail == nil {
			logger.Trace("got ok response from envoy proxy", "nonce", req.ResponseNonce)
			t.ack(req.ResponseNonce)
		} else {
			logger.Error("got error response from envoy proxy", "nonce", req.ResponseNonce,
				"error", status.ErrorProto(req.ErrorDetail))
			t.nack(req.ResponseNonce)
		}
	}

	if registeredThisTime && len(req.InitialResourceVersions) > 0 {
		/*
			Additionally, the first message (for a given type_url) of a
			reconnected gRPC stream has a third role:

			[3] informing the server of the resources (and their versions) that
			the client already possesses, using the initial_resource_versions
			field.
		*/
		logger.Trace("setting initial resource versions for stream",
			"resources", req.InitialResourceVersions)
		t.resourceVersions = req.InitialResourceVersions
		if !t.wildcard {
			for k := range req.InitialResourceVersions {
				t.subscriptions[k] = struct{}{}
			}
		}
	}

	if !t.wildcard {
		/*
			[1] informing the server of what resources the client has
			gained/lost interest in (using resource_names_subscribe and
			resource_names_unsubscribe), or
		*/
		for _, name := range req.ResourceNamesSubscribe {
			// A resource_names_subscribe field may contain resource names that
			// the server believes the client is already subscribed to, and
			// furthermore has the most recent versions of. However, the server
			// must still provide those resources in the response; due to
			// implementation details hidden from the server, the client may
			// have “forgotten” those resources despite apparently remaining
			// subscribed.
			//
			// NOTE: the server must respond with all resources listed in
			// resource_names_subscribe, even if it believes the client has the
			// most recent version of them. The reason: the client may have
			// dropped them, but then regained interest before it had a chance
			// to send the unsubscribe message.
			//
			// We handle that here by ALWAYS wiping the version so the diff
			// decides to send the value.
			_, alreadySubscribed := t.subscriptions[name]
			t.subscriptions[name] = struct{}{}

			// Reset the tracked version so we force a reply.
			if _, alreadyTracked := t.resourceVersions[name]; alreadyTracked {
				t.resourceVersions[name] = ""
			}

			if alreadySubscribed {
				logger.Trace("re-subscribing resource for stream", "resource", name)
			} else {
				logger.Trace("subscribing resource for stream", "resource", name)
			}
		}

		for _, name := range req.ResourceNamesUnsubscribe {
			if _, ok := t.subscriptions[name]; !ok {
				continue
			}
			delete(t.subscriptions, name)
			logger.Trace("unsubscribing resource for stream", "resource", name)
			// NOTE: we'll let the normal differential comparison handle cleaning up resourceVersions
		}
	}

	return registeredThisTime
}

func (t *xDSDeltaType) ack(nonce string) {
	pending, ok := t.pendingUpdates[nonce]
	if !ok {
		return
	}

	for name, obj := range pending {
		if obj.Remove {
			delete(t.resourceVersions, name)
			continue
		}

		t.resourceVersions[name] = obj.Version
		if t.childType != nil {
			// This branch only matters on UPDATE, since we already have
			// mechanisms to clean up orphaned resources.
			for _, childName := range obj.ChildResources {
				if _, exist := t.childType.resourceVersions[childName]; !exist {
					continue
				}
				if !t.subscribed(childName) {
					continue
				}
				t.generator.Logger.Trace(
					"triggering implicit update of resource",
					"typeUrl", t.typeURL,
					"resource", name,
					"childTypeUrl", t.childType.typeURL,
					"childResource", childName,
				)
				// Basically manifest this as a re-subscribe/re-sync
				t.childType.resourceVersions[childName] = ""
			}
		}
	}
	t.sentToEnvoyOnce = true
	delete(t.pendingUpdates, nonce)
}

func (t *xDSDeltaType) nack(nonce string) {
	delete(t.pendingUpdates, nonce)
}

func (t *xDSDeltaType) SendIfNew(
	kind structs.ServiceKind,
	currentVersions map[string]string, // type => name => version (as consul knows right now)
	resourceMap *IndexedResources,
	nonce *uint64,
	upsert, remove bool,
) (error, bool) {
	if t == nil || !t.registered {
		return nil, false
	}
	logger := t.generator.Logger.With("typeUrl", t.typeURL)

	allowEmpty := t.allowEmptyFn != nil && t.allowEmptyFn(kind)

	// Zero length resource responses should be ignored and are the result of no
	// data yet. Notice that this caused a bug originally where we had zero
	// healthy endpoints for an upstream that would cause Envoy to hang waiting
	// for the EDS response. This is fixed though by ensuring we send an explicit
	// empty LoadAssignment resource for the cluster rather than allowing junky
	// empty resources.
	if len(currentVersions) == 0 && !allowEmpty {
		// Nothing to send yet
		return nil, false
	}

	resp, updates, err := t.createDeltaResponse(currentVersions, resourceMap, upsert, remove)
	if err != nil {
		return err, false
	}

	if resp == nil {
		return nil, false
	}

	*nonce++
	resp.Nonce = fmt.Sprintf("%08x", *nonce)

	t.generator.logTraceResponse("Incremental xDS v3", resp)

	logger.Trace("sending response", "nonce", resp.Nonce)
	if err := t.stream.Send(resp); err != nil {
		return err, false
	}
	logger.Trace("sent response", "nonce", resp.Nonce)

	if t.childType != nil {
		// Capture the relevant child resource names on this pending update so
		// we can properly clean up the linked children when this change is
		// ACKed.
		for name, obj := range updates {
			if children, ok := resourceMap.ChildIndex[t.typeURL][name]; ok {
				obj.ChildResources = children
				updates[name] = obj
			}
		}
	}
	t.pendingUpdates[resp.Nonce] = updates

	return nil, true
}

func (t *xDSDeltaType) createDeltaResponse(
	currentVersions map[string]string, // name => version (as consul knows right now)
	resourceMap *IndexedResources,
	upsert, remove bool,
) (*envoy_discovery_v3.DeltaDiscoveryResponse, map[string]PendingUpdate, error) {
	// compute difference
	var (
		hasRelevantUpdates = false
		updates            = make(map[string]PendingUpdate)
	)

	if t.wildcard {
		// First find things that need updating or deleting
		for name, envoyVers := range t.resourceVersions {
			currVers, ok := currentVersions[name]
			if !ok {
				if remove {
					hasRelevantUpdates = true
				}
				updates[name] = PendingUpdate{Remove: true}
			} else if currVers != envoyVers {
				if upsert {
					hasRelevantUpdates = true
				}
				updates[name] = PendingUpdate{Version: currVers}
			}
		}

		// Now find new things
		for name, currVers := range currentVersions {
			if _, known := t.resourceVersions[name]; known {
				continue
			}
			if upsert {
				hasRelevantUpdates = true
			}
			updates[name] = PendingUpdate{Version: currVers}
		}
	} else {
		// First find things that need updating or deleting

		// Walk the list of things currently stored in envoy
		for name, envoyVers := range t.resourceVersions {
			if t.subscribed(name) {
				if currVers, ok := currentVersions[name]; ok {
					if currVers != envoyVers {
						if upsert {
							hasRelevantUpdates = true
						}
						updates[name] = PendingUpdate{Version: currVers}
					}
				}
			}
		}

		// Now find new things not in envoy yet
		for name := range t.subscriptions {
			if _, known := t.resourceVersions[name]; known {
				continue
			}
			if currVers, ok := currentVersions[name]; ok {
				updates[name] = PendingUpdate{Version: currVers}
				if upsert {
					hasRelevantUpdates = true
				}
			}
		}
	}

	if !hasRelevantUpdates && t.sentToEnvoyOnce {
		return nil, nil, nil
	}

	// now turn this into a disco response
	resp := &envoy_discovery_v3.DeltaDiscoveryResponse{
		// TODO(rb): consider putting something in SystemVersionInfo?
		TypeUrl: t.typeURL,
	}
	realUpdates := make(map[string]PendingUpdate)
	for name, obj := range updates {
		if obj.Remove {
			if remove {
				resp.RemovedResources = append(resp.RemovedResources, name)
				realUpdates[name] = PendingUpdate{Remove: true}
			}
		} else if upsert {
			resources, ok := resourceMap.Index[t.typeURL]
			if !ok {
				return nil, nil, fmt.Errorf("unknown type url: %s", t.typeURL)
			}
			res, ok := resources[name]
			if !ok {
				return nil, nil, fmt.Errorf("unknown name for type url %q: %s", t.typeURL, name)
			}
			any, err := ptypes.MarshalAny(res)
			if err != nil {
				return nil, nil, err
			}

			resp.Resources = append(resp.Resources, &envoy_discovery_v3.Resource{
				Name:     name,
				Resource: any,
				Version:  obj.Version,
			})
			realUpdates[name] = obj
		}
	}

	return resp, realUpdates, nil
}

func computeResourceVersions(resourceMap *IndexedResources) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string)
	for typeUrl, resources := range resourceMap.Index {
		m, err := hashResourceMap(resources)
		if err != nil {
			return nil, fmt.Errorf("failed to hash resources for %q: %v", typeUrl, err)
		}
		out[typeUrl] = m
	}
	return out, nil
}

type IndexedResources struct {
	// Index is a map of typeURL => resourceName => resource
	Index map[string]map[string]proto.Message

	// ChildIndex is a map of typeURL => parentResourceName => list of
	// childResourceNames. This only applies if the child and parent do not
	// share a name.
	ChildIndex map[string]map[string][]string
}

func emptyIndexedResources() *IndexedResources {
	return &IndexedResources{
		Index: map[string]map[string]proto.Message{
			ListenerType: make(map[string]proto.Message),
			RouteType:    make(map[string]proto.Message),
			ClusterType:  make(map[string]proto.Message),
			EndpointType: make(map[string]proto.Message),
		},
		ChildIndex: map[string]map[string][]string{
			ListenerType: make(map[string][]string),
			ClusterType:  make(map[string][]string),
		},
	}
}

func populateChildIndexMap(resourceMap *IndexedResources) error {
	// LDS and RDS have a more complicated relationship.
	for name, res := range resourceMap.Index[ListenerType] {
		listener := res.(*envoy_listener_v3.Listener)
		rdsRouteNames, err := extractRdsResourceNames(listener)
		if err != nil {
			return err
		}
		resourceMap.ChildIndex[ListenerType][name] = rdsRouteNames
	}

	// CDS and EDS share exact names.
	for name := range resourceMap.Index[ClusterType] {
		resourceMap.ChildIndex[ClusterType][name] = []string{name}
	}

	return nil
}

func indexResources(logger hclog.Logger, resources map[string][]proto.Message) *IndexedResources {
	data := emptyIndexedResources()

	for typeURL, typeRes := range resources {
		for _, res := range typeRes {
			name := getResourceName(res)
			if name == "" {
				logger.Warn("skipping unexpected xDS type found in delta snapshot", "typeURL", typeURL)
			} else {
				data.Index[typeURL][name] = res
			}
		}
	}

	return data
}

func getResourceName(res proto.Message) string {
	// NOTE: this only covers types that we currently care about for LDS/RDS/CDS/EDS
	switch x := res.(type) {
	case *envoy_listener_v3.Listener: // LDS
		return x.Name
	case *envoy_route_v3.RouteConfiguration: // RDS
		return x.Name
	case *envoy_cluster_v3.Cluster: // CDS
		return x.Name
	case *envoy_endpoint_v3.ClusterLoadAssignment: // EDS
		return x.ClusterName
	default:
		return ""
	}
}

func hashResourceMap(resources map[string]proto.Message) (map[string]string, error) {
	m := make(map[string]string)
	for name, res := range resources {
		h, err := hashResource(res)
		if err != nil {
			return nil, fmt.Errorf("failed to hash resource %q: %v", name, err)
		}
		m[name] = h
	}
	return m, nil
}

// hashResource will take a resource and create a SHA256 hash sum out of the marshaled bytes
func hashResource(res proto.Message) (string, error) {
	h := sha256.New()
	buffer := proto.NewBuffer(nil)
	buffer.SetDeterministic(true)

	err := buffer.Marshal(res)
	if err != nil {
		return "", err
	}
	h.Write(buffer.Bytes())
	buffer.Reset()

	return hex.EncodeToString(h.Sum(nil)), nil
}

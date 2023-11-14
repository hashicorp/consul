// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib/retry"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/autopilotevents"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
	"github.com/hashicorp/consul/proto/private/pbservice"
)

type MaterializedViewStore interface {
	Get(ctx context.Context, req submatview.Request) (submatview.Result, error)
	Notify(ctx context.Context, req submatview.Request, cID string, ch chan<- cache.UpdateEvent) error
}

type SubscriptionBackend interface {
	Subscriber
}

// subscriptionManager handlers requests to subscribe to events from an events publisher.
type subscriptionManager struct {
	logger               hclog.Logger
	config               Config
	trustDomain          string
	viewStore            MaterializedViewStore
	backend              SubscriptionBackend
	getStore             func() StateStore
	serviceSubReady      <-chan struct{}
	trustBundlesSubReady <-chan struct{}
	serverAddrsSubReady  <-chan struct{}
}

// TODO(peering): Maybe centralize so that there is a single manager per datacenter, rather than per peering.
func newSubscriptionManager(
	ctx context.Context,
	logger hclog.Logger,
	config Config,
	trustDomain string,
	backend SubscriptionBackend,
	getStore func() StateStore,
	remoteSubTracker *resourceSubscriptionTracker,
) *subscriptionManager {
	logger = logger.Named("subscriptions")
	store := submatview.NewStore(logger.Named("viewstore"))
	go store.Run(ctx)

	return &subscriptionManager{
		logger:               logger,
		config:               config,
		trustDomain:          trustDomain,
		viewStore:            store,
		backend:              backend,
		getStore:             getStore,
		serviceSubReady:      remoteSubTracker.SubscribedChan(pbpeerstream.TypeURLExportedService),
		trustBundlesSubReady: remoteSubTracker.SubscribedChan(pbpeerstream.TypeURLPeeringTrustBundle),
		serverAddrsSubReady:  remoteSubTracker.SubscribedChan(pbpeerstream.TypeURLPeeringServerAddresses),
	}
}

// subscribe returns a channel that will contain updates to exported service instances for a given peer.
func (m *subscriptionManager) subscribe(ctx context.Context, peerID, peerName, partition string) <-chan cache.UpdateEvent {
	var (
		updateCh       = make(chan cache.UpdateEvent, 1)
		publicUpdateCh = make(chan cache.UpdateEvent, 1)
	)

	state := newSubscriptionState(peerName, partition)
	state.publicUpdateCh = publicUpdateCh
	state.updateCh = updateCh

	// Wrap our bare state store queries in goroutines that emit events.
	go m.notifyExportedServicesForPeerID(ctx, state, peerID)
	go m.notifyServerAddrUpdates(ctx, state.updateCh)
	if m.config.ConnectEnabled {
		go m.notifyMeshGatewaysForPartition(ctx, state, state.partition)
		// If connect is enabled, watch for updates to CA roots.
		go m.notifyRootCAUpdatesForPartition(ctx, state.updateCh, state.partition)
	}

	// This goroutine is the only one allowed to manipulate protected
	// subscriptionManager fields.
	go m.handleEvents(ctx, state, updateCh)

	return publicUpdateCh
}

func (m *subscriptionManager) handleEvents(ctx context.Context, state *subscriptionState, updateCh <-chan cache.UpdateEvent) {
	for {
		// TODO(peering): exponential backoff

		select {
		case <-ctx.Done():
			return
		case update := <-updateCh:
			if err := m.handleEvent(ctx, state, update); err != nil {
				m.logger.Error("Failed to handle update from watch",
					"id", update.CorrelationID, "error", err,
				)
				continue
			}
		}
	}
}

func (m *subscriptionManager) handleEvent(ctx context.Context, state *subscriptionState, u cache.UpdateEvent) error {
	if u.Err != nil {
		return fmt.Errorf("received error event: %w", u.Err)
	}

	switch {
	case u.CorrelationID == subExportedServiceList:
		// Everything starts with the exported service list coming from
		// our state store watchset loop.
		evt, ok := u.Result.(*structs.ExportedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		state.exportList = evt

		pending := &pendingPayload{}
		m.syncNormalServices(ctx, state, evt.Services)
		if m.config.ConnectEnabled {
			m.syncDiscoveryChains(state, pending, evt.DiscoChains)
		}

		err := pending.Add(
			exportedServiceListID,
			subExportedServiceList,
			pbpeerstream.ExportedServiceListFromStruct(evt),
		)
		if err != nil {
			return err
		}

		state.sendPendingEvents(ctx, m.logger, pending)

		// cleanup event versions too
		state.cleanupEventVersions(m.logger)

	case strings.HasPrefix(u.CorrelationID, subExportedService):
		csn, ok := u.Result.(*pbservice.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		// TODO(peering): is it safe to edit these protobufs in place?

		// Clear this raft index before exporting.
		csn.Index = 0

		// Ensure that connect things are scrubbed so we don't mix-and-match
		// with the synthetic entries that point to mesh gateways.
		filterConnectReferences(csn)

		// Flatten health checks
		for _, instance := range csn.Nodes {
			instance.Checks = flattenChecks(
				instance.Node.Node,
				instance.Service.ID,
				instance.Service.Service,
				instance.Service.EnterpriseMeta,
				instance.Checks,
			)
		}

		// Scrub raft indexes
		for _, instance := range csn.Nodes {
			instance.Node.RaftIndex = nil
			instance.Service.RaftIndex = nil
			// skip checks since we just generated one from scratch
		}

		id := servicePayloadIDPrefix + strings.TrimPrefix(u.CorrelationID, subExportedService)

		// Just ferry this one directly along to the destination.
		pending := &pendingPayload{}
		if err := pending.Add(id, u.CorrelationID, csn); err != nil {
			return err
		}
		state.sendPendingEvents(ctx, m.logger, pending)

	case strings.HasPrefix(u.CorrelationID, subMeshGateway):
		csn, ok := u.Result.(*pbservice.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		partition := strings.TrimPrefix(u.CorrelationID, subMeshGateway)

		if !m.config.ConnectEnabled {
			return nil // ignore event
		}

		if !acl.EqualPartitions(partition, state.partition) {
			return nil // ignore event
		}

		// Clear this raft index before exporting.
		csn.Index = 0

		// Flatten health checks
		for _, instance := range csn.Nodes {
			instance.Checks = flattenChecks(
				instance.Node.Node,
				instance.Service.ID,
				instance.Service.Service,
				instance.Service.EnterpriseMeta,
				instance.Checks,
			)
		}

		// Scrub raft indexes
		for _, instance := range csn.Nodes {
			instance.Node.RaftIndex = nil
			instance.Service.RaftIndex = nil
			// skip checks since we just generated one from scratch

			// Remove connect things like native mode.
			if instance.Service.Connect != nil || instance.Service.Proxy != nil {
				instance.Service.Connect = nil
				instance.Service.Proxy = nil

				// VirtualIPs assigned in this cluster won't make sense on the importing side
				delete(instance.Service.TaggedAddresses, structs.TaggedAddressVirtualIP)
			}
		}

		state.meshGateway = csn

		pending := &pendingPayload{}

		if state.exportList != nil {
			// Trigger public events for all synthetic discovery chain replies.
			for chainName, info := range state.connectServices {
				m.collectPendingEventForDiscoveryChain(state, pending, chainName, info)
			}
		}

		// TODO(peering): should we ship this down verbatim to the consumer?
		state.sendPendingEvents(ctx, m.logger, pending)

	case u.CorrelationID == subCARoot:
		roots, ok := u.Result.(*pbpeering.PeeringTrustBundle)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		pending := &pendingPayload{}
		if err := pending.Add(caRootsPayloadID, u.CorrelationID, roots); err != nil {
			return err
		}

		state.sendPendingEvents(ctx, m.logger, pending)

	case u.CorrelationID == subServerAddrs:
		addrs, ok := u.Result.(*pbpeering.PeeringServerAddresses)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		pending := &pendingPayload{}
		if err := pending.Add(serverAddrsPayloadID, u.CorrelationID, addrs); err != nil {
			return err
		}

		state.sendPendingEvents(ctx, m.logger, pending)
	default:
		return fmt.Errorf("unknown correlation ID: %s", u.CorrelationID)
	}
	return nil
}

func filterConnectReferences(orig *pbservice.IndexedCheckServiceNodes) {
	newNodes := make([]*pbservice.CheckServiceNode, 0, len(orig.Nodes))
	for i := range orig.Nodes {
		csn := orig.Nodes[i]

		if csn.Service.Kind != string(structs.ServiceKindTypical) {
			continue // skip non-typical services
		}

		if strings.HasSuffix(csn.Service.Service, syntheticProxyNameSuffix) {
			// Skip things that might LOOK like a proxy so we don't get a
			// collision with the ones we generate.
			continue
		}

		// Remove connect things like native mode.
		if csn.Service.Connect != nil || csn.Service.Proxy != nil {
			csn = proto.Clone(csn).(*pbservice.CheckServiceNode)
			csn.Service.Connect = nil
			csn.Service.Proxy = nil

			// VirtualIPs assigned in this cluster won't make sense on the importing side
			delete(csn.Service.TaggedAddresses, structs.TaggedAddressVirtualIP)
		}

		newNodes = append(newNodes, csn)
	}
	orig.Nodes = newNodes
}

func (m *subscriptionManager) notifyRootCAUpdatesForPartition(
	ctx context.Context,
	updateCh chan<- cache.UpdateEvent,
	partition string,
) {
	// Wait until this is subscribed-to.
	select {
	case <-m.trustBundlesSubReady:
	case <-ctx.Done():
		return
	}

	var idx uint64
	// TODO(peering): retry logic; fail past a threshold
	for {
		var err error
		// Typically, this function will block inside `m.subscribeCARoots` and only return on error.
		// Errors are logged and the watch is retried.
		idx, err = m.subscribeCARoots(ctx, idx, updateCh, partition)
		if errors.Is(err, stream.ErrSubForceClosed) {
			m.logger.Trace("subscription force-closed due to an ACL change or snapshot restore, will attempt resume")
		} else if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			m.logger.Warn("failed to subscribe to CA roots, will attempt resume", "error", err.Error())
		} else {
			m.logger.Trace(err.Error())
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

const subCARoot = "roots"

// subscribeCARoots subscribes to state.EventTopicCARoots for changes to CA roots.
// Upon receiving an event it will send the payload in updateCh.
func (m *subscriptionManager) subscribeCARoots(
	ctx context.Context,
	idx uint64,
	updateCh chan<- cache.UpdateEvent,
	partition string,
) (uint64, error) {
	// following code adapted from connectca/watch_roots.go
	sub, err := m.backend.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicCARoots,
		Subject: stream.SubjectNone,
		Token:   "", // using anonymous token for now
		Index:   idx,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to subscribe to CA Roots events: %w", err)
	}
	defer sub.Unsubscribe()

	for {
		event, err := sub.Next(ctx)
		switch {
		case errors.Is(err, stream.ErrSubForceClosed):
			// If the subscription was closed because the state store was abandoned (e.g.
			// following a snapshot restore) reset idx to ensure we don't skip over the
			// new store's events.
			select {
			case <-m.getStore().AbandonCh():
				idx = 0
			default:
			}
			return idx, err
		case errors.Is(err, context.Canceled):
			return 0, err
		case errors.Is(err, context.DeadlineExceeded):
			return 0, err
		case err != nil:
			return idx, fmt.Errorf("failed to read next event: %w", err)
		}

		// Note: this check isn't strictly necessary because the event publishing
		// machinery will ensure the index increases monotonically, but it can be
		// tricky to faithfully reproduce this in tests (e.g. the EventPublisher
		// garbage collects topic buffers and snapshots aggressively when streams
		// disconnect) so this avoids a bunch of confusing setup code.
		if event.Index <= idx {
			continue
		}

		idx = event.Index

		// We do not send framing events (e.g. EndOfSnapshot, NewSnapshotToFollow)
		// because we send a full list of roots on every event, rather than expecting
		// clients to maintain a state-machine in the way they do for service health.
		if event.IsFramingEvent() {
			continue
		}

		payload, ok := event.Payload.(state.EventPayloadCARoots)
		if !ok {
			return 0, fmt.Errorf("unexpected event payload type: %T", payload)
		}

		var rootPems []string
		for _, root := range payload.CARoots {
			rootPems = append(rootPems, root.RootCert)
		}

		updateCh <- cache.UpdateEvent{
			CorrelationID: subCARoot,
			Result: &pbpeering.PeeringTrustBundle{
				TrustDomain:       m.trustDomain,
				RootPEMs:          rootPems,
				ExportedPartition: partition,
				// TODO(peering): revisit decision not to validate datacenter in RBAC
			},
		}
	}
}

func (m *subscriptionManager) syncNormalServices(
	ctx context.Context,
	state *subscriptionState,
	services []structs.ServiceName,
) {
	// seen contains the set of exported service names and is used to reconcile the list of watched services.
	seen := make(map[structs.ServiceName]struct{})

	// Ensure there is a subscription for each service exported to the peer.
	for _, svc := range services {
		seen[svc] = struct{}{}

		if _, ok := state.watchedServices[svc]; ok {
			// Exported service is already being watched, nothing to do.
			continue
		}

		notifyCtx, cancel := context.WithCancel(ctx)
		if err := m.NotifyStandardService(notifyCtx, svc, state.updateCh); err != nil {
			cancel()
			m.logger.Error("failed to subscribe to service", "service", svc.String())
			continue
		}

		state.watchedServices[svc] = cancel
	}

	// For every subscription without an exported service, call the associated cancel fn.
	for svc, cancel := range state.watchedServices {
		if _, ok := seen[svc]; !ok {
			cancel()
			delete(state.watchedServices, svc)
		}
	}
}

func (m *subscriptionManager) syncDiscoveryChains(
	state *subscriptionState,
	pending *pendingPayload,
	chainsByName map[structs.ServiceName]structs.ExportedDiscoveryChainInfo,
) {
	// if it was newly added, then try to emit an UPDATE event
	for chainName, info := range chainsByName {
		if oldInfo, ok := state.connectServices[chainName]; ok && info.Equal(oldInfo) {
			continue
		}

		state.connectServices[chainName] = info

		m.collectPendingEventForDiscoveryChain(state, pending, chainName, info)
	}

	// if it was dropped, try to emit an DELETE event
	for chainName := range state.connectServices {
		if _, ok := chainsByName[chainName]; ok {
			continue
		}

		delete(state.connectServices, chainName)

		if state.meshGateway != nil {
			// Only need to clean this up if we know we may have ever sent it in the first place.
			proxyName := generateProxyNameForDiscoveryChain(chainName)
			err := pending.Add(
				discoveryChainPayloadIDPrefix+chainName.String(),
				subExportedService+proxyName.String(),
				&pbservice.IndexedCheckServiceNodes{},
			)
			if err != nil {
				m.logger.Error("failed to send event for discovery chain", "service", chainName.String(), "error", err)
				continue
			}
		}
	}
}

func (m *subscriptionManager) collectPendingEventForDiscoveryChain(
	state *subscriptionState,
	pending *pendingPayload,
	chainName structs.ServiceName,
	info structs.ExportedDiscoveryChainInfo,
) {
	if _, ok := state.connectServices[chainName]; !ok {
		return // not found
	}

	if state.exportList == nil || state.meshGateway == nil {
		return // skip because we don't have the data to do it yet
	}

	// Emit event with fake data
	proxyName := generateProxyNameForDiscoveryChain(chainName)

	err := pending.Add(
		discoveryChainPayloadIDPrefix+chainName.String(),
		subExportedService+proxyName.String(),
		createDiscoChainHealth(
			state.peerName,
			m.config.Datacenter,
			m.trustDomain,
			chainName,
			info,
			state.meshGateway,
		),
	)
	if err != nil {
		m.logger.Error("failed to send event for discovery chain", "service", chainName.String(), "error", err)
	}
}

func createDiscoChainHealth(
	peerName string,
	datacenter, trustDomain string,
	sn structs.ServiceName,
	info structs.ExportedDiscoveryChainInfo,
	pb *pbservice.IndexedCheckServiceNodes,
) *pbservice.IndexedCheckServiceNodes {
	fakeProxyName := sn.Name + syntheticProxyNameSuffix

	var peerMeta *pbservice.PeeringServiceMeta
	{
		spiffeID := connect.SpiffeIDService{
			Host:       trustDomain,
			Partition:  sn.PartitionOrDefault(),
			Namespace:  sn.NamespaceOrDefault(),
			Datacenter: datacenter,
			Service:    sn.Name,
		}
		mainSpiffeIDString := spiffeID.URI().String()

		sni := connect.PeeredServiceSNI(
			sn.Name,
			sn.NamespaceOrDefault(),
			sn.PartitionOrDefault(),
			peerName,
			trustDomain,
		)

		gwSpiffeID := connect.SpiffeIDMeshGateway{
			Host:       trustDomain,
			Partition:  sn.PartitionOrDefault(),
			Datacenter: datacenter,
		}

		// Create common peer meta.
		//
		// TODO(peering): should this be replicated by service and not by instance?
		peerMeta = &pbservice.PeeringServiceMeta{
			SNI: []string{sni},
			SpiffeID: []string{
				mainSpiffeIDString,
				// Always include the gateway id here to facilitate error-free
				// L4/L7 upgrade/downgrade scenarios.
				gwSpiffeID.URI().String(),
			},
			Protocol: info.Protocol,
		}

		if !structs.IsProtocolHTTPLike(info.Protocol) {
			for _, target := range info.TCPTargets {
				targetSpiffeID := connect.SpiffeIDService{
					Host:       trustDomain,
					Partition:  target.Partition,
					Namespace:  target.Namespace,
					Datacenter: target.Datacenter,
					Service:    target.Service,
				}
				targetSpiffeIDString := targetSpiffeID.URI().String()
				if targetSpiffeIDString != mainSpiffeIDString {
					peerMeta.SpiffeID = append(peerMeta.SpiffeID, targetSpiffeIDString)
				}
			}
		}
	}

	newNodes := make([]*pbservice.CheckServiceNode, 0, len(pb.Nodes))
	for i := range pb.Nodes {
		gwNode := pb.Nodes[i].Node
		gwService := pb.Nodes[i].Service
		gwChecks := pb.Nodes[i].Checks

		pbEntMeta := pbcommon.NewEnterpriseMetaFromStructs(sn.EnterpriseMeta)

		fakeProxyID := fakeProxyName
		destServiceID := sn.Name
		if gwService.ID != "" {
			// This is only going to be relevant if multiple mesh gateways are
			// on the same exporting node.
			fakeProxyID = fmt.Sprintf("%s-instance-%d", fakeProxyName, i)
			destServiceID = fmt.Sprintf("%s-instance-%d", sn.Name, i)
		}

		csn := &pbservice.CheckServiceNode{
			Node: gwNode,
			Service: &pbservice.NodeService{
				Kind:           string(structs.ServiceKindConnectProxy),
				Service:        fakeProxyName,
				ID:             fakeProxyID,
				EnterpriseMeta: pbEntMeta,
				PeerName:       structs.DefaultPeerKeyword,
				Proxy: &pbservice.ConnectProxyConfig{
					DestinationServiceName: sn.Name,
					DestinationServiceID:   destServiceID,
				},
				// direct
				Address:         gwService.Address,
				TaggedAddresses: gwService.TaggedAddresses,
				Port:            gwService.Port,
				SocketPath:      gwService.SocketPath,
				Weights:         gwService.Weights,
				Connect: &pbservice.ServiceConnect{
					PeerMeta: peerMeta,
				},
			},
			Checks: flattenChecks(gwNode.Node, fakeProxyID, fakeProxyName, pbEntMeta, gwChecks),
		}
		newNodes = append(newNodes, csn)
	}

	return &pbservice.IndexedCheckServiceNodes{
		Index: 0,
		Nodes: newNodes,
	}
}

var statusScores = map[string]int{
	// 0 is reserved for unknown
	api.HealthMaint:    1,
	api.HealthCritical: 2,
	api.HealthWarning:  3,
	api.HealthPassing:  4,
}

func getMostImportantStatus(a, b string) string {
	if statusScores[a] < statusScores[b] {
		return a
	}
	return b
}

func flattenChecks(
	nodeName string,
	serviceID string,
	serviceName string,
	entMeta *pbcommon.EnterpriseMeta,
	checks []*pbservice.HealthCheck,
) []*pbservice.HealthCheck {
	if len(checks) == 0 {
		return nil
	}

	// Similar logic to (api.HealthChecks).AggregatedStatus()
	healthStatus := api.HealthPassing
	if len(checks) > 0 {
		for _, chk := range checks {
			id := chk.CheckID
			if id == api.NodeMaint || strings.HasPrefix(id, api.ServiceMaintPrefix) {
				healthStatus = api.HealthMaint
				break // always wins
			}
			healthStatus = getMostImportantStatus(healthStatus, chk.Status)
		}
	}

	if serviceID == "" {
		serviceID = serviceName
	}

	return []*pbservice.HealthCheck{
		{
			CheckID:        serviceID + ":overall-check",
			Name:           "overall-check",
			Status:         healthStatus,
			Node:           nodeName,
			ServiceID:      serviceID,
			ServiceName:    serviceName,
			EnterpriseMeta: entMeta,
			PeerName:       structs.DefaultPeerKeyword,
		},
	}
}

const (
	subExportedServiceList = "exported-service-list"
	subExportedService     = "exported-service:"
	subMeshGateway         = "mesh-gateway:"
)

// NotifyStandardService will notify the given channel when there are updates
// to the requested service of the same name in the catalog.
func (m *subscriptionManager) NotifyStandardService(
	ctx context.Context,
	svc structs.ServiceName,
	updateCh chan<- cache.UpdateEvent,
) error {
	sr := newExportedStandardServiceRequest(m.logger, svc, m.backend)
	return m.viewStore.Notify(ctx, sr, subExportedService+svc.String(), updateCh)
}

// syntheticProxyNameSuffix is the suffix to add to synthetic proxies we
// replicate to route traffic to an exported discovery chain through the mesh
// gateways.
//
// This name was chosen to match existing "sidecar service" generation logic
// and similar logic in the Service Identity synthetic ACL policies.
const syntheticProxyNameSuffix = structs.SidecarProxySuffix

func generateProxyNameForDiscoveryChain(sn structs.ServiceName) structs.ServiceName {
	return structs.NewServiceName(sn.Name+syntheticProxyNameSuffix, &sn.EnterpriseMeta)
}

const subServerAddrs = "server-addrs"

func (m *subscriptionManager) notifyServerAddrUpdates(
	ctx context.Context,
	updateCh chan<- cache.UpdateEvent,
) {
	// Wait until server address updates are subscribed-to.
	select {
	case <-m.serverAddrsSubReady:
	case <-ctx.Done():
		return
	}

	configNotifyCh := m.notifyMeshConfigUpdates(ctx)

	// Intentionally initialized to empty values.
	// These are set after the first mesh config entry update arrives.
	var queryCtx context.Context
	cancel := func() {}

	useGateways := false
	for {
		select {
		case <-ctx.Done():
			cancel()
			return

		case event := <-configNotifyCh:
			entry, ok := event.Result.(*structs.MeshConfigEntry)
			if event.Result != nil && !ok {
				m.logger.Error(fmt.Sprintf("saw unexpected type %T for mesh config entry: falling back to pushing direct server addresses", event.Result))
			}
			if entry != nil && entry.Peering != nil && entry.Peering.PeerThroughMeshGateways {
				useGateways = true
			} else {
				useGateways = false
			}

			// Cancel and re-set watches based on the updated config entry.
			cancel()

			queryCtx, cancel = context.WithCancel(ctx)

			if useGateways {
				go m.notifyServerMeshGatewayAddresses(queryCtx, updateCh)
			} else {
				go m.ensureServerAddrSubscription(queryCtx, updateCh)
			}
		}
	}
}

func (m *subscriptionManager) notifyMeshConfigUpdates(ctx context.Context) <-chan cache.UpdateEvent {
	const meshConfigWatch = "mesh-config-entry"

	notifyCh := make(chan cache.UpdateEvent, 1)
	go m.syncViaBlockingQuery(ctx, meshConfigWatch, func(_ context.Context, store StateStore, ws memdb.WatchSet) (interface{}, error) {
		_, rawEntry, err := store.ConfigEntry(ws, structs.MeshConfig, structs.MeshConfigMesh, acl.DefaultEnterpriseMeta())
		if err != nil {
			return nil, fmt.Errorf("failed to get mesh config entry: %w", err)

		}
		return rawEntry, nil
	}, meshConfigWatch, notifyCh)

	return notifyCh
}

func (m *subscriptionManager) notifyServerMeshGatewayAddresses(ctx context.Context, updateCh chan<- cache.UpdateEvent) {
	m.syncViaBlockingQuery(ctx, "mesh-gateways", func(ctx context.Context, store StateStore, ws memdb.WatchSet) (interface{}, error) {
		_, nodes, err := store.ServiceDump(ws, structs.ServiceKindMeshGateway, true, acl.DefaultEnterpriseMeta(), structs.DefaultPeerKeyword)
		if err != nil {
			return nil, fmt.Errorf("failed to watch mesh gateways services for servers: %w", err)
		}

		var gatewayAddrs []string
		for _, csn := range nodes {
			_, addr, port := csn.BestAddress(true)
			gatewayAddrs = append(gatewayAddrs, ipaddr.FormatAddressPort(addr, port))
		}
		if len(gatewayAddrs) == 0 {
			return nil, errors.New("configured to peer through mesh gateways but no mesh gateways are registered")
		}

		// We may return an empty list if there are no gateway addresses.
		return &pbpeering.PeeringServerAddresses{
			Addresses: gatewayAddrs,
		}, nil
	}, subServerAddrs, updateCh)
}

func (m *subscriptionManager) ensureServerAddrSubscription(ctx context.Context, updateCh chan<- cache.UpdateEvent) {
	waiter := &retry.Waiter{
		MinFailures: 1,
		Factor:      500 * time.Millisecond,
		MaxWait:     60 * time.Second,
		Jitter:      retry.NewJitter(100),
	}

	logger := m.logger.With("queryType", "server-addresses")

	var idx uint64
	for {
		var err error

		idx, err = m.subscribeServerAddrs(ctx, idx, updateCh)
		if err == nil {
			waiter.Reset()
		} else if errors.Is(err, stream.ErrSubForceClosed) {
			logger.Trace("subscription force-closed due to an ACL change or snapshot restore, will attempt resume")

		} else if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("failed to subscribe to server addresses, will attempt resume", "error", err.Error())

		} else if err != nil {
			logger.Trace(err.Error())
			return
		}
		if err := waiter.Wait(ctx); err != nil {
			return
		}
	}
}

func (m *subscriptionManager) subscribeServerAddrs(
	ctx context.Context,
	idx uint64,
	updateCh chan<- cache.UpdateEvent,
) (uint64, error) {
	// following code adapted from serverdiscovery/watch_servers.go
	sub, err := m.backend.Subscribe(&stream.SubscribeRequest{
		Topic:   autopilotevents.EventTopicReadyServers,
		Subject: stream.SubjectNone,
		Token:   "", // using anonymous token for now
		Index:   idx,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to subscribe to ReadyServers events: %w", err)
	}
	defer sub.Unsubscribe()

	for {
		event, err := sub.Next(ctx)
		switch {
		case errors.Is(err, context.Canceled):
			return 0, err
		case err != nil:
			return idx, err
		}

		// We do not send framing events (e.g. EndOfSnapshot, NewSnapshotToFollow)
		// because we send a full list of ready servers on every event, rather than expecting
		// clients to maintain a state-machine in the way they do for service health.
		if event.IsFramingEvent() {
			continue
		}

		// Note: this check isn't strictly necessary because the event publishing
		// machinery will ensure the index increases monotonically, but it can be
		// tricky to faithfully reproduce this in tests (e.g. the EventPublisher
		// garbage collects topic buffers and snapshots aggressively when streams
		// disconnect) so this avoids a bunch of confusing setup code.
		if event.Index <= idx {
			continue
		}

		idx = event.Index

		payload, ok := event.Payload.(autopilotevents.EventPayloadReadyServers)
		if !ok {
			return 0, fmt.Errorf("unexpected event payload type: %T", payload)
		}

		var serverAddrs = make([]string, 0, len(payload))

		for _, srv := range payload {
			if srv.ExtGRPCPort == 0 {
				continue
			}
			addr := srv.Address

			// wan address is preferred
			if v, ok := srv.TaggedAddresses[structs.TaggedAddressWAN]; ok && v != "" {
				addr = v
			}
			grpcAddr := addr + ":" + strconv.Itoa(srv.ExtGRPCPort)
			serverAddrs = append(serverAddrs, grpcAddr)
		}
		if len(serverAddrs) == 0 {
			m.logger.Warn("did not find any server addresses with external gRPC ports to publish")
			continue
		}

		u := cache.UpdateEvent{
			CorrelationID: subServerAddrs,
			Result: &pbpeering.PeeringServerAddresses{
				Addresses: serverAddrs,
			},
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case updateCh <- u:
		}
	}
}

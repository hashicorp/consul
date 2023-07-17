package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

// EventSubjectService is a stream.Subject used to route and receive events for
// a specific service.
type EventSubjectService struct {
	Key            string
	EnterpriseMeta acl.EnterpriseMeta
	PeerName       string

	overrideKey       string
	overrideNamespace string
	overridePartition string
}

// EventPayloadCheckServiceNode is used as the Payload for a stream.Event to
// indicates changes to a CheckServiceNode for service health.
//
// The stream.Payload methods implemented by EventPayloadCheckServiceNode are
// do not mutate the payload, making it safe to use in an Event sent to
// stream.EventPublisher.Publish.
type EventPayloadCheckServiceNode struct {
	Op    pbsubscribe.CatalogOp
	Value *structs.CheckServiceNode
	// key is used to override the key used to filter the payload. It is set for
	// events in the connect topic to specify the name of the underlying service
	// when the change event is for a sidecar or gateway.
	overrideKey       string
	overrideNamespace string
	overridePartition string
}

func (e EventPayloadCheckServiceNode) HasReadPermission(authz acl.Authorizer) bool {
	return e.Value.CanRead(authz) == acl.Allow
}

func (e EventPayloadCheckServiceNode) Subject() stream.Subject {
	return EventSubjectService{
		Key:            e.Value.Service.Service,
		EnterpriseMeta: e.Value.Service.EnterpriseMeta,
		PeerName:       e.Value.Service.PeerName,

		overrideKey:       e.overrideKey,
		overrideNamespace: e.overrideNamespace,
		overridePartition: e.overridePartition,
	}
}

func (e EventPayloadCheckServiceNode) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index: idx,
		Payload: &pbsubscribe.Event_ServiceHealth{
			ServiceHealth: &pbsubscribe.ServiceHealthUpdate{
				Op:               e.Op,
				CheckServiceNode: pbservice.NewCheckServiceNodeFromStructs(e.Value),
			},
		},
	}
}

// EventPayloadServiceListUpdate is used as the Payload for a stream.Event when
// services (not service instances) are registered/deregistered. These events
// are used to materialize the list of services in a datacenter.
type EventPayloadServiceListUpdate struct {
	Op pbsubscribe.CatalogOp

	Name           string
	EnterpriseMeta acl.EnterpriseMeta
	PeerName       string
}

func (e *EventPayloadServiceListUpdate) ToSubscriptionEvent(idx uint64) *pbsubscribe.Event {
	return &pbsubscribe.Event{
		Index: idx,
		Payload: &pbsubscribe.Event_Service{
			Service: &pbsubscribe.ServiceListUpdate{
				Op:             e.Op,
				Name:           e.Name,
				EnterpriseMeta: pbcommon.NewEnterpriseMetaFromStructs(e.EnterpriseMeta),
				PeerName:       e.PeerName,
			},
		},
	}
}

func (e *EventPayloadServiceListUpdate) Subject() stream.Subject { return stream.SubjectNone }

func (e *EventPayloadServiceListUpdate) HasReadPermission(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.EnterpriseMeta.FillAuthzContext(&authzContext)
	return authz.ServiceRead(e.Name, &authzContext) == acl.Allow
}

// serviceHealthSnapshot returns a stream.SnapshotFunc that provides a snapshot
// of stream.Events that describe the current state of a service health query.
func (s *Store) ServiceHealthSnapshot(req stream.SubscribeRequest, buf stream.SnapshotAppender) (index uint64, err error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()

	connect := req.Topic == EventTopicServiceHealthConnect

	subject, ok := req.Subject.(EventSubjectService)
	if !ok {
		return 0, fmt.Errorf("expected SubscribeRequest.Subject to be a: state.EventSubjectService, was a: %T", req.Subject)
	}

	idx, nodes, err := checkServiceNodesTxn(tx, nil, subject.Key, connect, &subject.EnterpriseMeta, subject.PeerName)
	if err != nil {
		return 0, err
	}

	for i := range nodes {
		n := nodes[i]
		event := stream.Event{
			Index: idx,
			Topic: req.Topic,
			Payload: EventPayloadCheckServiceNode{
				Op:    pbsubscribe.CatalogOp_Register,
				Value: &n,
			},
		}

		if !connect {
			// append each event as a separate item so that they can be serialized
			// separately, to prevent the encoding of one massive message.
			buf.Append([]stream.Event{event})
			continue
		}

		events, err := connectEventsByServiceKind(tx, event)
		if err != nil {
			return idx, err
		}
		buf.Append(events)
	}

	return idx, err
}

// TODO: this could use NodeServiceQuery
type nodeServiceTuple struct {
	Node      string
	ServiceID string
	EntMeta   acl.EnterpriseMeta
	PeerName  string
}

func newNodeServiceTupleFromServiceNode(sn *structs.ServiceNode) nodeServiceTuple {
	return nodeServiceTuple{
		Node:      strings.ToLower(sn.Node),
		ServiceID: sn.ServiceID,
		EntMeta:   sn.EnterpriseMeta,
		PeerName:  sn.PeerName,
	}
}

func newNodeServiceTupleFromServiceHealthCheck(hc *structs.HealthCheck) nodeServiceTuple {
	return nodeServiceTuple{
		Node:      strings.ToLower(hc.Node),
		ServiceID: hc.ServiceID,
		EntMeta:   hc.EnterpriseMeta,
		PeerName:  hc.PeerName,
	}
}

type serviceChange struct {
	changeType changeType
	change     memdb.Change
}

type nodeTuple struct {
	Node      string
	Partition string
	PeerName  string
}

var serviceChangeIndirect = serviceChange{changeType: changeIndirect}

// ServiceListUpdateEventsFromChanges returns events representing changes to
// the list of services from the given set of state store changes.
func ServiceListUpdateEventsFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event
	for _, change := range changes.Changes {
		if change.Table != tableKindServiceNames {
			continue
		}

		kindName := changeObject(change).(*KindServiceName)

		// TODO(peering): make this peer-aware.
		payload := &EventPayloadServiceListUpdate{
			Name:           kindName.Service.Name,
			EnterpriseMeta: kindName.Service.EnterpriseMeta,
		}

		if change.Deleted() {
			payload.Op = pbsubscribe.CatalogOp_Deregister
		} else {
			payload.Op = pbsubscribe.CatalogOp_Register
		}

		events = append(events, stream.Event{
			Topic:   EventTopicServiceList,
			Index:   changes.Index,
			Payload: payload,
		})
	}
	return events, nil
}

// ServiceListSnapshot is a stream.SnapshotFunc that returns a snapshot of
// all service names.
func (s *Store) ServiceListSnapshot(_ stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
	index, names, err := s.ServiceNamesOfKind(nil, "")
	if err != nil {
		return 0, err
	}

	if l := len(names); l > 0 {
		events := make([]stream.Event, l)
		for idx, name := range names {
			events[idx] = stream.Event{
				Topic: EventTopicServiceList,
				Index: index,
				Payload: &EventPayloadServiceListUpdate{
					Op:             pbsubscribe.CatalogOp_Register,
					Name:           name.Service.Name,
					EnterpriseMeta: name.Service.EnterpriseMeta,
				},
			}
		}
		buf.Append(events)
	}

	return index, nil
}

// ServiceHealthEventsFromChanges returns all the service and Connect health
// events that should be emitted given a set of changes to the state store.
func ServiceHealthEventsFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event

	var nodeChanges map[nodeTuple]changeType
	var serviceChanges map[nodeServiceTuple]serviceChange
	var termGatewayChanges map[structs.ServiceName]map[structs.ServiceName]serviceChange

	markNode := func(node nodeTuple, typ changeType) {
		if nodeChanges == nil {
			nodeChanges = make(map[nodeTuple]changeType)
		}
		// If the caller has an actual node mutation ensure we store it even if the
		// node is already marked. If the caller is just marking the node dirty
		// without a node change, don't overwrite any existing node change we know
		// about.
		if nodeChanges[node] == changeIndirect {
			nodeChanges[node] = typ
		}
	}
	markService := func(key nodeServiceTuple, svcChange serviceChange) {
		if serviceChanges == nil {
			serviceChanges = make(map[nodeServiceTuple]serviceChange)
		}
		// If the caller has an actual service mutation ensure we store it even if
		// the service is already marked. If the caller is just marking the service
		// dirty without a service change, don't overwrite any existing service change we
		// know about.
		if serviceChanges[key].changeType == changeIndirect {
			serviceChanges[key] = svcChange
		}
	}

	for _, change := range changes.Changes {
		switch change.Table {
		case tableNodes:
			// Node changed in some way, if it's not a delete, we'll need to
			// re-deliver CheckServiceNode results for all services on that node but
			// we mark it anyway because if it _is_ a delete then we need to know that
			// later to avoid trying to deliver events when node level checks mark the
			// node as "changed".
			n := changeObject(change).(*structs.Node)
			tuple := newNodeTupleFromNode(n)
			markNode(tuple, changeTypeFromChange(change))

		case tableServices:
			sn := changeObject(change).(*structs.ServiceNode)
			srvChange := serviceChange{changeType: changeTypeFromChange(change), change: change}
			markService(newNodeServiceTupleFromServiceNode(sn), srvChange)

		case tableChecks:
			// For health we only care about the scope for now to know if it's just
			// affecting a single service or every service on a node. There is a
			// subtle edge case where the check with same ID changes from being node
			// scoped to service scoped or vice versa, in either case we need to treat
			// it as affecting all services on the node.
			switch {
			case change.Updated():
				before := change.Before.(*structs.HealthCheck)
				after := change.After.(*structs.HealthCheck)
				if after.ServiceID == "" || before.ServiceID == "" {
					// check before and/or after is node-scoped
					nt := newNodeTupleFromHealthCheck(after)
					markNode(nt, changeIndirect)
				} else {
					// Check changed which means we just need to emit for the linked
					// service.
					markService(newNodeServiceTupleFromServiceHealthCheck(after), serviceChangeIndirect)

					// Edge case - if the check with same ID was updated to link to a
					// different service ID but the old service with old ID still exists,
					// then the old service instance needs updating too as it has one
					// fewer checks now.
					if before.ServiceID != after.ServiceID {
						markService(newNodeServiceTupleFromServiceHealthCheck(before), serviceChangeIndirect)
					}
				}

			case change.Deleted(), change.Created():
				obj := changeObject(change).(*structs.HealthCheck)
				if obj.ServiceID == "" {
					// Node level check
					nt := newNodeTupleFromHealthCheck(obj)
					markNode(nt, changeIndirect)
				} else {
					markService(newNodeServiceTupleFromServiceHealthCheck(obj), serviceChangeIndirect)
				}
			}
		case tableGatewayServices:
			gs := changeObject(change).(*structs.GatewayService)
			if gs.GatewayKind != structs.ServiceKindTerminatingGateway {
				continue
			}

			gsChange := serviceChange{changeType: changeTypeFromChange(change), change: change}

			if termGatewayChanges == nil {
				termGatewayChanges = make(map[structs.ServiceName]map[structs.ServiceName]serviceChange)
			}

			_, ok := termGatewayChanges[gs.Gateway]
			if !ok {
				termGatewayChanges[gs.Gateway] = map[structs.ServiceName]serviceChange{}
			}

			switch gsChange.changeType {
			case changeUpdate:
				after := gsChange.change.After.(*structs.GatewayService)
				if gsChange.change.Before.(*structs.GatewayService).IsSame(after) {
					continue
				}
				termGatewayChanges[gs.Gateway][gs.Service] = gsChange
			case changeDelete, changeCreate:
				termGatewayChanges[gs.Gateway][gs.Service] = gsChange
			}
		}
	}

	// Now act on those marked nodes/services
	for node, changeType := range nodeChanges {
		if changeType == changeDelete {
			// Node deletions are a no-op here since the state store transaction will
			// have also removed all the service instances which will be handled in
			// the loop below.
			continue
		}
		// Rebuild events for all services on this node
		es, err := newServiceHealthEventsForNode(tx, changes.Index, node.Node,
			structs.WildcardEnterpriseMetaInPartition(node.Partition), node.PeerName)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)
	}

	for tuple, srvChange := range serviceChanges {
		if srvChange.changeType == changeDelete {
			sn := srvChange.change.Before.(*structs.ServiceNode)
			e := newServiceHealthEventDeregister(changes.Index, sn)
			events = append(events, e)
			continue
		}

		// Check if this was a service mutation that changed it's name which
		// requires special handling even if node changed and new events were
		// already published.
		if srvChange.changeType == changeUpdate {
			before := srvChange.change.Before.(*structs.ServiceNode)
			after := srvChange.change.After.(*structs.ServiceNode)

			if before.ServiceName != after.ServiceName {
				// Service was renamed, the code below will ensure the new registrations
				// go out to subscribers to the new service name topic key, but we need
				// to fix up subscribers that were watching the old name by sending
				// deregistrations.
				e := newServiceHealthEventDeregister(changes.Index, before)
				events = append(events, e)
			}

			if e, ok := isConnectProxyDestinationServiceChange(changes.Index, before, after); ok {
				events = append(events, e)
			}
		}

		if _, ok := nodeChanges[tuple.nodeTuple()]; ok {
			// We already rebuilt events for everything on this node, no need to send
			// a duplicate.
			continue
		}
		// Build service event and append it
		e, err := newServiceHealthEventForService(tx, changes.Index, tuple)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	for gatewayName, svcChanges := range termGatewayChanges {
		for serviceName, gsChange := range svcChanges {
			gs := changeObject(gsChange.change).(*structs.GatewayService)

			q := Query{
				Value:          gs.Gateway.Name,
				EnterpriseMeta: gatewayName.EnterpriseMeta,
				PeerName:       structs.TODOPeerKeyword,
			}
			_, nodes, err := serviceNodesTxn(tx, nil, indexService, q)
			if err != nil {
				return nil, err
			}

			// Always send deregister events for deletes/updates.
			if gsChange.changeType != changeCreate {
				for _, sn := range nodes {
					e := newServiceHealthEventDeregister(changes.Index, sn)

					e.Topic = EventTopicServiceHealthConnect
					payload := e.Payload.(EventPayloadCheckServiceNode)
					payload.overrideKey = serviceName.Name
					if gatewayName.EnterpriseMeta.NamespaceOrDefault() != serviceName.EnterpriseMeta.NamespaceOrDefault() {
						payload.overrideNamespace = serviceName.EnterpriseMeta.NamespaceOrDefault()
					}
					if gatewayName.EnterpriseMeta.PartitionOrDefault() != serviceName.EnterpriseMeta.PartitionOrDefault() {
						payload.overridePartition = serviceName.EnterpriseMeta.PartitionOrDefault()
					}
					e.Payload = payload

					events = append(events, e)
				}
			}

			if gsChange.changeType == changeDelete {
				continue
			}

			// Build service events and append them
			for _, sn := range nodes {
				tuple := newNodeServiceTupleFromServiceNode(sn)

				// If we're already sending an event for the service, don't send another.
				if _, ok := serviceChanges[tuple]; ok {
					continue
				}

				e, err := newServiceHealthEventForService(tx, changes.Index, tuple)
				if err != nil {
					return nil, err
				}

				e.Topic = EventTopicServiceHealthConnect
				payload := e.Payload.(EventPayloadCheckServiceNode)
				payload.overrideKey = serviceName.Name
				if gatewayName.EnterpriseMeta.NamespaceOrDefault() != serviceName.EnterpriseMeta.NamespaceOrDefault() {
					payload.overrideNamespace = serviceName.EnterpriseMeta.NamespaceOrDefault()
				}
				if gatewayName.EnterpriseMeta.PartitionOrDefault() != serviceName.EnterpriseMeta.PartitionOrDefault() {
					payload.overridePartition = serviceName.EnterpriseMeta.PartitionOrDefault()
				}
				e.Payload = payload

				events = append(events, e)
			}
		}
	}

	// Duplicate any events that affected connect-enabled instances (proxies or
	// native apps) to the relevant Connect topic.
	connectEvents, err := serviceHealthToConnectEvents(tx, events...)
	if err != nil {
		return nil, err
	}
	events = append(events, connectEvents...)

	return events, nil
}

// isConnectProxyDestinationServiceChange handles the case where a Connect proxy changed
// the service it is proxying. We need to issue a de-registration for the old
// service on the Connect topic. We don't actually need to deregister this sidecar
// service though as it still exists and didn't change its name.
func isConnectProxyDestinationServiceChange(idx uint64, before, after *structs.ServiceNode) (stream.Event, bool) {
	if before.ServiceKind != structs.ServiceKindConnectProxy ||
		before.ServiceProxy.DestinationServiceName == after.ServiceProxy.DestinationServiceName {
		return stream.Event{}, false
	}

	e := newServiceHealthEventDeregister(idx, before)
	e.Topic = EventTopicServiceHealthConnect
	payload := e.Payload.(EventPayloadCheckServiceNode)
	payload.overrideKey = payload.Value.Service.Proxy.DestinationServiceName
	e.Payload = payload
	return e, true
}

type changeType uint8

const (
	// changeIndirect indicates some other object changed which has implications
	// for the target object.
	changeIndirect changeType = iota
	changeDelete
	changeCreate
	changeUpdate
)

func changeTypeFromChange(change memdb.Change) changeType {
	switch {
	case change.Deleted():
		return changeDelete
	case change.Created():
		return changeCreate
	default:
		return changeUpdate
	}
}

// serviceHealthToConnectEvents converts already formatted service health
// registration events into the ones needed to publish to the Connect topic.
// This essentially means filtering out any instances that are not Connect
// enabled and so of no interest to those subscribers but also involves
// switching connection details to be the proxy instead of the actual instance
// in case of a sidecar.
func serviceHealthToConnectEvents(
	tx ReadTxn,
	events ...stream.Event,
) ([]stream.Event, error) {
	var result []stream.Event
	for _, event := range events {
		if event.Topic != EventTopicServiceHealth { // event.Topic == topicServiceHealthConnect
			// Skip non-health or any events already emitted to Connect topic
			continue
		}

		connectEvents, err := connectEventsByServiceKind(tx, event)
		if err != nil {
			return nil, err
		}

		result = append(result, connectEvents...)
	}

	return result, nil
}

func connectEventsByServiceKind(tx ReadTxn, origEvent stream.Event) ([]stream.Event, error) {
	node := getPayloadCheckServiceNode(origEvent.Payload)
	if node.Service == nil {
		return nil, nil
	}

	event := origEvent // shallow copy the event
	event.Topic = EventTopicServiceHealthConnect

	if node.Service.Connect.Native {
		return []stream.Event{event}, nil
	}

	switch node.Service.Kind {
	case structs.ServiceKindConnectProxy:
		payload := event.Payload.(EventPayloadCheckServiceNode)
		payload.overrideKey = node.Service.Proxy.DestinationServiceName
		event.Payload = payload
		return []stream.Event{event}, nil

	case structs.ServiceKindTerminatingGateway:
		var result []stream.Event

		// TODO(peering): handle terminating gateways somehow

		sn := structs.ServiceName{
			Name:           node.Service.Service,
			EnterpriseMeta: node.Service.EnterpriseMeta,
		}
		iter, err := tx.Get(tableGatewayServices, indexGateway, sn)
		if err != nil {
			return nil, err
		}

		// similar to checkServiceNodesTxn -> serviceGatewayNodes
		for obj := iter.Next(); obj != nil; obj = iter.Next() {
			result = append(result, copyEventForService(event, obj.(*structs.GatewayService).Service))
		}
		return result, nil
	default:
		// All other cases are not relevant to the connect topic
	}
	return nil, nil
}

func copyEventForService(event stream.Event, service structs.ServiceName) stream.Event {
	event.Topic = EventTopicServiceHealthConnect
	payload := event.Payload.(EventPayloadCheckServiceNode)
	payload.overrideKey = service.Name
	if payload.Value.Service.EnterpriseMeta.NamespaceOrDefault() != service.EnterpriseMeta.NamespaceOrDefault() {
		payload.overrideNamespace = service.EnterpriseMeta.NamespaceOrDefault()
	}
	if payload.Value.Service.EnterpriseMeta.PartitionOrDefault() != service.EnterpriseMeta.PartitionOrDefault() {
		payload.overridePartition = service.EnterpriseMeta.PartitionOrDefault()
	}

	event.Payload = payload
	return event
}

func getPayloadCheckServiceNode(payload stream.Payload) *structs.CheckServiceNode {
	ep, ok := payload.(EventPayloadCheckServiceNode)
	if !ok {
		return nil
	}
	return ep.Value
}

// newServiceHealthEventsForNode returns health events for all services on the
// given node. This mirrors some of the the logic in the oddly-named
// parseCheckServiceNodes but is more efficient since we know they are all on
// the same node.
func newServiceHealthEventsForNode(tx ReadTxn, idx uint64, node string, entMeta *acl.EnterpriseMeta, peerName string) ([]stream.Event, error) {
	services, err := tx.Get(tableServices, indexNode, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return nil, err
	}

	n, checksFunc, err := getNodeAndChecks(tx, node, entMeta, peerName)
	if err != nil {
		return nil, err
	}

	var events []stream.Event
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)

		event := newServiceHealthEventRegister(idx, n, sn, checksFunc(sn.ServiceID))
		events = append(events, event)
	}

	return events, nil
}

// getNodeAndNodeChecks returns a the node structure and a function that returns
// the full list of checks for a specific service on that node.
func getNodeAndChecks(tx ReadTxn, node string, entMeta *acl.EnterpriseMeta, peerName string) (*structs.Node, serviceChecksFunc, error) {
	// Fetch the node
	nodeRaw, err := tx.First(tableNodes, indexID, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return nil, nil, err
	}
	if nodeRaw == nil {
		return nil, nil, ErrMissingNode
	}
	n := nodeRaw.(*structs.Node)

	iter, err := tx.Get(tableChecks, indexNode, Query{
		Value:          node,
		EnterpriseMeta: *entMeta,
		PeerName:       peerName,
	})
	if err != nil {
		return nil, nil, err
	}

	var nodeChecks structs.HealthChecks
	var svcChecks map[string]structs.HealthChecks

	for check := iter.Next(); check != nil; check = iter.Next() {
		check := check.(*structs.HealthCheck)
		if check.ServiceID == "" {
			nodeChecks = append(nodeChecks, check)
		} else {
			if svcChecks == nil {
				svcChecks = make(map[string]structs.HealthChecks)
			}
			svcChecks[check.ServiceID] = append(svcChecks[check.ServiceID], check)
		}
	}
	serviceChecks := func(serviceID string) structs.HealthChecks {
		// Create a new slice so that append does not modify the array backing nodeChecks.
		result := make(structs.HealthChecks, 0, len(nodeChecks))
		result = append(result, nodeChecks...)
		for _, check := range svcChecks[serviceID] {
			result = append(result, check)
		}
		return result
	}
	return n, serviceChecks, nil
}

type serviceChecksFunc func(serviceID string) structs.HealthChecks

func newServiceHealthEventForService(tx ReadTxn, idx uint64, tuple nodeServiceTuple) (stream.Event, error) {
	n, checksFunc, err := getNodeAndChecks(tx, tuple.Node, &tuple.EntMeta, tuple.PeerName)
	if err != nil {
		return stream.Event{}, err
	}

	svc, err := tx.Get(tableServices, indexID, NodeServiceQuery{
		EnterpriseMeta: tuple.EntMeta,
		Node:           tuple.Node,
		Service:        tuple.ServiceID,
		PeerName:       tuple.PeerName,
	})
	if err != nil {
		return stream.Event{}, err
	}

	raw := svc.Next()
	if raw == nil {
		return stream.Event{}, ErrMissingService
	}

	sn := raw.(*structs.ServiceNode)
	return newServiceHealthEventRegister(idx, n, sn, checksFunc(sn.ServiceID)), nil
}

func newServiceHealthEventRegister(
	idx uint64,
	node *structs.Node,
	sn *structs.ServiceNode,
	checks structs.HealthChecks,
) stream.Event {
	csn := &structs.CheckServiceNode{
		Node:    node,
		Service: sn.ToNodeService(),
		Checks:  checks,
	}
	return stream.Event{
		Topic: EventTopicServiceHealth,
		Index: idx,
		Payload: EventPayloadCheckServiceNode{
			Op:    pbsubscribe.CatalogOp_Register,
			Value: csn,
		},
	}
}

func newServiceHealthEventDeregister(idx uint64, sn *structs.ServiceNode) stream.Event {
	// We actually only need the node name populated in the node part as it's only
	// used as a key to know which service was deregistered so don't bother looking
	// up the node in the DB. Note that while the ServiceNode does have NodeID
	// etc. fields, they are never populated in memdb per the comment on that
	// struct and only filled in when we return copies of the result to users.
	// This is also important because if the service was deleted as part of a
	// whole node deregistering then the node record won't actually exist now
	// anyway and we'd have to plumb it through from the changeset above.

	entMeta := sn.EnterpriseMeta
	entMeta.Normalize()

	csn := &structs.CheckServiceNode{
		Node: &structs.Node{
			Node:      sn.Node,
			Partition: entMeta.PartitionOrEmpty(),
			PeerName:  sn.PeerName,
		},
		Service: sn.ToNodeService(),
	}

	return stream.Event{
		Topic: EventTopicServiceHealth,
		Index: idx,
		Payload: EventPayloadCheckServiceNode{
			Op:    pbsubscribe.CatalogOp_Deregister,
			Value: csn,
		},
	}
}

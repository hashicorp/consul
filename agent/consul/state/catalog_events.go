package state

import (
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

type changeOp int

const (
	OpDelete changeOp = iota
	OpCreate
	OpUpdate
)

type eventPayload struct {
	Op  changeOp
	Obj interface{}
}

// serviceHealthSnapshot returns a stream.SnapshotFunc that provides a snapshot
// of stream.Events that describe the current state of a service health query.
//
// TODO: no tests for this yet
func serviceHealthSnapshot(s *Store, topic topic) stream.SnapshotFunc {
	return func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (index uint64, err error) {
		tx := s.db.Txn(false)
		defer tx.Abort()

		connect := topic == TopicServiceHealthConnect
		// TODO(namespace-streaming): plumb entMeta through from SubscribeRequest
		idx, nodes, err := checkServiceNodesTxn(tx, nil, req.Key, connect, nil)
		if err != nil {
			return 0, err
		}

		for _, n := range nodes {
			event := stream.Event{
				Index: idx,
				Topic: topic,
				Payload: eventPayload{
					Op:  OpCreate,
					Obj: &n,
				},
			}

			if n.Service != nil {
				event.Key = n.Service.Service
			}

			// TODO: could all the events be appended as a single item?
			buf.Append([]stream.Event{event})
		}

		return idx, err
	}
}

type nodeServiceTuple struct {
	Node      string
	ServiceID string
	EntMeta   structs.EnterpriseMeta
}

// ServiceHealthEventsFromChanges returns all the service and Connect health
// events that should be emitted given a set of changes to the state store.
func ServiceHealthEventsFromChanges(tx ReadTxn, changes Changes) ([]stream.Event, error) {
	var events []stream.Event

	var nodeChanges map[string]*memdb.Change
	var serviceChanges map[nodeServiceTuple]*memdb.Change

	markNode := func(node string, nodeChange *memdb.Change) {
		if nodeChanges == nil {
			nodeChanges = make(map[string]*memdb.Change)
		}
		// If the caller has an actual node mutation ensure we store it even if the
		// node is already marked. If the caller is just marking the node dirty
		// without an node change, don't overwrite any existing node change we know
		// about.
		ch := nodeChanges[node]
		if ch == nil {
			nodeChanges[node] = nodeChange
		}
	}
	markService := func(node, service string, entMeta structs.EnterpriseMeta, svcChange *memdb.Change) {
		if serviceChanges == nil {
			serviceChanges = make(map[nodeServiceTuple]*memdb.Change)
		}
		k := nodeServiceTuple{
			Node:      node,
			ServiceID: service,
			EntMeta:   entMeta,
		}
		// If the caller has an actual service mutation ensure we store it even if
		// the service is already marked. If the caller is just marking the service
		// dirty without an node change, don't overwrite any existing node change we
		// know about.
		ch := serviceChanges[k]
		if ch == nil {
			serviceChanges[k] = svcChange
		}
	}

	for _, change := range changes.Changes {
		switch change.Table {
		case "nodes":
			// Node changed in some way, if it's not a delete, we'll need to
			// re-deliver CheckServiceNode results for all services on that node but
			// we mark it anyway because if it _is_ a delete then we need to know that
			// later to avoid trying to deliver events when node level checks mark the
			// node as "changed".
			nRaw := change.After
			if change.After == nil {
				nRaw = change.Before
			}
			n := nRaw.(*structs.Node)
			changeCopy := change
			markNode(n.Node, &changeCopy)

		case "services":
			snRaw := change.After
			if change.After == nil {
				snRaw = change.Before
			}
			sn := snRaw.(*structs.ServiceNode)
			changeCopy := change
			markService(sn.Node, sn.ServiceID, sn.EnterpriseMeta, &changeCopy)

		case "checks":
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
					// Either changed from or to being node-scoped
					markNode(after.Node, nil)
				} else {
					// Check changed which means we just need to emit for the linked
					// service.
					markService(after.Node, after.ServiceID, after.EnterpriseMeta, nil)

					// Edge case - if the check with same ID was updated to link to a
					// different service ID but the old service with old ID still exists,
					// then the old service instance needs updating too as it has one
					// fewer checks now.
					if before.ServiceID != after.ServiceID {
						markService(before.Node, before.ServiceID, before.EnterpriseMeta, nil)
					}
				}

			case change.Deleted():
				before := change.Before.(*structs.HealthCheck)
				if before.ServiceID == "" {
					// Node level check
					markNode(before.Node, nil)
				} else {
					markService(before.Node, before.ServiceID, before.EnterpriseMeta, nil)
				}

			case change.Created():
				after := change.After.(*structs.HealthCheck)
				if after.ServiceID == "" {
					// Node level check
					markNode(after.Node, nil)
				} else {
					markService(after.Node, after.ServiceID, after.EnterpriseMeta, nil)
				}
			}
		}
	}

	// Now act on those marked nodes/services
	for node, change := range nodeChanges {
		// change may be nil if there was a change that _affected_ the node
		// like a change to checks but it didn't actually change the node
		// record itself.
		if change != nil && change.Deleted() {
			// Node deletions are a no-op here since the state store transaction will
			// have also removed all the service instances which will be handled in
			// the loop below.
			continue
		}
		// Rebuild events for all services on this node
		es, err := serviceHealthEventsForNode(tx, changes.Index, node)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)
	}

	for tuple, change := range serviceChanges {
		// change may be nil if there was a change that _affected_ the service
		// like a change to checks but it didn't actually change the service
		// record itself.
		if change != nil && change.Deleted() {
			// Generate delete event for the service instance and append it
			sn := change.Before.(*structs.ServiceNode)
			es, err := serviceHealthDeregEventsForServiceInstance(changes.Index, sn, &tuple.EntMeta)
			if err != nil {
				return nil, err
			}
			events = append(events, es...)
			continue
		}

		// Check if this was a service mutation that changed it's name which
		// requires special handling even if node changed and new events were
		// already published.
		if change != nil && change.Updated() {
			before := change.Before.(*structs.ServiceNode)
			after := change.After.(*structs.ServiceNode)

			if before.ServiceName != after.ServiceName {
				// Service was renamed, the code below will ensure the new registrations
				// go out to subscribers to the new service name topic key, but we need
				// to fix up subscribers that were watching the old name by sending
				// deregistrations.
				es, err := serviceHealthDeregEventsForServiceInstance(changes.Index, before, &tuple.EntMeta)
				if err != nil {
					return nil, err
				}
				events = append(events, es...)
			}

			if before.ServiceKind == structs.ServiceKindConnectProxy &&
				before.ServiceProxy.DestinationServiceName != after.ServiceProxy.DestinationServiceName {
				// Connect proxy changed the service it is representing, need to issue a
				// dereg for the old service on the Connect topic. We don't actually need
				// to deregister this sidecar service though as it still exists and
				// didn't change its name (or if it did that was caught just above). But
				// our mechanism for connect events is to convert them so we generate
				// the regular one, convert it to Connect topic and then discar the
				// original.
				es, err := serviceHealthDeregEventsForServiceInstance(changes.Index, before, &tuple.EntMeta)
				if err != nil {
					return nil, err
				}
				// Don't append es per comment above, but convert it to connect topic
				// events.
				es = serviceHealthToConnectEvents(es)
				events = append(events, es...)
			}
		}

		if _, ok := nodeChanges[tuple.Node]; ok {
			// We already rebuilt events for everything on this node, no need to send
			// a duplicate.
			continue
		}
		// Build service event and append it
		es, err := serviceHealthEventsForServiceInstance(tx, changes.Index, tuple)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)
	}

	// Duplicate any events that affected connect-enabled instances (proxies or
	// native apps) to the relevant Connect topic.
	events = append(events, serviceHealthToConnectEvents(events)...)

	return events, nil
}

// serviceHealthToConnectEvents converts already formatted service health
// registration events into the ones needed to publish to the Connect topic.
// This essentially means filtering out any instances that are not Connect
// enabled and so of no interest to those subscribers but also involves
// switching connection details to be the proxy instead of the actual instance
// in case of a sidecar.
func serviceHealthToConnectEvents(events []stream.Event) []stream.Event {
	serviceHealthConnectEvents := make([]stream.Event, 0, len(events))
	for _, event := range events {
		if event.Topic != TopicServiceHealth {
			// Skip non-health or any events already emitted to Connect topic
			continue
		}
		node := getPayloadCheckServiceNode(event.Payload)
		if node.Service == nil ||
			(node.Service.Kind != structs.ServiceKindConnectProxy && !node.Service.Connect.Native) {
			// Event is not a service instance (i.e. just a node registration)
			// or is not a service that is not connect-enabled in some way.
			continue
		}

		connectEvent := event
		connectEvent.Topic = TopicServiceHealthConnect

		// If this is a proxy, set the key to the destination service name.
		if node.Service.Kind == structs.ServiceKindConnectProxy {
			connectEvent.Key = node.Service.Proxy.DestinationServiceName
		}

		serviceHealthConnectEvents = append(serviceHealthConnectEvents, connectEvent)
	}

	return serviceHealthConnectEvents
}

func getPayloadCheckServiceNode(payload interface{}) *structs.CheckServiceNode {
	ep, ok := payload.(eventPayload)
	if !ok {
		return nil
	}
	csn, ok := ep.Obj.(*structs.CheckServiceNode)
	if !ok {
		return nil
	}
	return csn
}

// serviceHealthEventsForNode returns health events for all services on the
// given node. This mirrors some of the the logic in the oddly-named
// parseCheckServiceNodes but is more efficient since we know they are all on
// the same node.
func serviceHealthEventsForNode(tx ReadTxn, idx uint64, node string) ([]stream.Event, error) {
	// TODO(namespace-streaming): figure out the right EntMeta and mystery arg.
	services, err := catalogServiceListByNode(tx, node, nil, false)
	if err != nil {
		return nil, err
	}

	n, nodeChecks, svcChecks, err := getNodeAndChecks(tx, node)
	if err != nil {
		return nil, err
	}

	var events []stream.Event
	for service := services.Next(); service != nil; service = services.Next() {
		sn := service.(*structs.ServiceNode)

		es, err := serviceHealthEventsForServiceNodeInternal(idx, n, sn, nodeChecks, svcChecks)
		if err != nil {
			return nil, err
		}

		// Append to the results.
		events = append(events, es...)
	}

	return events, nil
}

// getNodeAndNodeChecks returns a specific node and ALL checks on that node
// (both node specific and service-specific). node-level Checks are returned as
// a slice, service-specific checks as a map of slices with the service id as
// the map key.
func getNodeAndChecks(tx ReadTxn, node string) (*structs.Node,
	structs.HealthChecks, map[string]structs.HealthChecks, error) {
	// Fetch the node
	nodeRaw, err := tx.First("nodes", "id", node)
	if err != nil {
		return nil, nil, nil, err
	}
	if nodeRaw == nil {
		return nil, nil, nil, ErrMissingNode
	}
	n := nodeRaw.(*structs.Node)

	// TODO(namespace-streaming): work out what EntMeta is needed here, wildcard?
	iter, err := catalogListChecksByNode(tx, node, nil)
	if err != nil {
		return nil, nil, nil, err
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
	return n, nodeChecks, svcChecks, nil
}

func serviceHealthEventsForServiceInstance(tx ReadTxn, idx uint64, tuple nodeServiceTuple) ([]stream.Event, error) {
	n, nodeChecks, svcChecks, err := getNodeAndChecks(tx, tuple.Node)
	if err != nil {
		return nil, err
	}

	svc, err := getCompoundWithTxn(tx, "services", "id", &tuple.EntMeta, tuple.Node, tuple.ServiceID)
	if err != nil {
		return nil, err
	}

	sn := svc.Next()
	if sn == nil {
		return nil, ErrMissingService
	}

	return serviceHealthEventsForServiceNodeInternal(idx, n, sn.(*structs.ServiceNode), nodeChecks, svcChecks)
}

func serviceHealthEventsForServiceNodeInternal(idx uint64,
	node *structs.Node,
	sn *structs.ServiceNode,
	nodeChecks structs.HealthChecks,
	svcChecks map[string]structs.HealthChecks) ([]stream.Event, error) {

	// Start with a copy of the node checks.
	checks := nodeChecks
	for _, check := range svcChecks[sn.ServiceID] {
		checks = append(checks, check)
	}

	csn := &structs.CheckServiceNode{
		Node:    node,
		Service: sn.ToNodeService(),
		Checks:  checks,
	}
	e := stream.Event{
		Topic: TopicServiceHealth,
		Key:   sn.ServiceName,
		Index: idx,
		Payload: eventPayload{
			Op:  OpCreate,
			Obj: csn,
		},
	}

	// See if we also need to emit a connect event (i.e. if this instance is a
	// connect proxy or connect native app).

	return []stream.Event{e}, nil
}

func serviceHealthDeregEventsForServiceInstance(idx uint64,
	sn *structs.ServiceNode, entMeta *structs.EnterpriseMeta) ([]stream.Event, error) {

	// We actually only need the node name populated in the node part as it's only
	// used as a key to know which service was deregistered so don't bother looking
	// up the node in the DB. Note that while the ServiceNode does have NodeID
	// etc. fields, they are never populated in memdb per the comment on that
	// struct and only filled in when we return copies of the result to users.
	// This is also important because if the service was deleted as part of a
	// whole node deregistering then the node record won't actually exist now
	// anyway and we'd have to plumb it through from the changeset above.
	csn := &structs.CheckServiceNode{
		Node: &structs.Node{
			Node: sn.Node,
		},
		Service: sn.ToNodeService(),
	}

	e := stream.Event{
		Topic: TopicServiceHealth,
		Key:   sn.ServiceName,
		Index: idx,
		Payload: eventPayload{
			Op:  OpDelete,
			Obj: csn,
		},
	}
	return []stream.Event{e}, nil
}

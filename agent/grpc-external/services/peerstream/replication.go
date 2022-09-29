package peerstream

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/googleapis/rpc/code"
	newproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbpeerstream"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbstatus"
	"github.com/hashicorp/consul/types"
)

/*
	TODO(peering):

	At the start of each peering stream establishment (not initiation, but the
	thing that reconnects) we need to do a little bit of light differential
	snapshot correction to initially synchronize the local state store.

	Then if we ever fail to apply a replication message we should either tear
	down the entire connection (and thus force a resync on reconnect) or
	request a resync operation.
*/

// makeServiceResponse handles preparing exported service instance updates to the peer cluster.
// Each cache.UpdateEvent will contain all instances for a service name.
// If there are no instances in the event, we consider that to be a de-registration.
func makeServiceResponse(
	mst *MutableStatus,
	update cache.UpdateEvent,
) (*pbpeerstream.ReplicationMessage_Response, error) {
	serviceName := strings.TrimPrefix(update.CorrelationID, subExportedService)
	sn := structs.ServiceNameFromString(serviceName)
	csn, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
	if !ok {
		return nil, fmt.Errorf("invalid type for service response: %T", update.Result)
	}

	export := &pbpeerstream.ExportedService{
		Nodes: csn.Nodes,
	}

	any, err := anypb.New(export)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	// If no nodes are present then it's due to one of:
	// 1. The service is newly registered or exported and yielded a transient empty update.
	// 2. All instances of the service were de-registered.
	// 3. The service was un-exported.
	//
	// We don't distinguish when these three things occurred, but it's safe to send a DELETE Op in all cases, so we do that.
	// Case #1 is a no-op for the importing peer.
	if len(csn.Nodes) == 0 {
		mst.RemoveExportedService(sn)

		return &pbpeerstream.ReplicationMessage_Response{
			ResourceURL: pbpeerstream.TypeURLExportedService,
			// TODO(peering): Nonce management
			Nonce:      "",
			ResourceID: serviceName,
			Operation:  pbpeerstream.Operation_OPERATION_DELETE,
		}, nil
	}

	mst.TrackExportedService(sn)

	// If there are nodes in the response, we push them as an UPSERT operation.
	return &pbpeerstream.ReplicationMessage_Response{
		ResourceURL: pbpeerstream.TypeURLExportedService,
		// TODO(peering): Nonce management
		Nonce:      "",
		ResourceID: serviceName,
		Operation:  pbpeerstream.Operation_OPERATION_UPSERT,
		Resource:   any,
	}, nil
}

func makeCARootsResponse(
	update cache.UpdateEvent,
) (*pbpeerstream.ReplicationMessage_Response, error) {
	any, _, err := marshalToProtoAny[*pbpeering.PeeringTrustBundle](update.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return &pbpeerstream.ReplicationMessage_Response{
		ResourceURL: pbpeerstream.TypeURLPeeringTrustBundle,
		// TODO(peering): Nonce management
		Nonce:      "",
		ResourceID: "roots",
		Operation:  pbpeerstream.Operation_OPERATION_UPSERT,
		Resource:   any,
	}, nil
}

func makeServerAddrsResponse(
	update cache.UpdateEvent,
) (*pbpeerstream.ReplicationMessage_Response, error) {
	any, _, err := marshalToProtoAny[*pbpeering.PeeringServerAddresses](update.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return &pbpeerstream.ReplicationMessage_Response{
		ResourceURL: pbpeerstream.TypeURLPeeringServerAddresses,
		// TODO(peering): Nonce management
		Nonce:      "",
		ResourceID: "server-addrs",
		Operation:  pbpeerstream.Operation_OPERATION_UPSERT,
		Resource:   any,
	}, nil
}

// marshalToProtoAny takes any input and returns:
// the protobuf.Any type, the asserted T type, and any errors
// during marshalling or type assertion.
// `in` MUST be of type T or it returns an error.
func marshalToProtoAny[T newproto.Message](in any) (*anypb.Any, T, error) {
	typ, ok := in.(T)
	if !ok {
		var outType T
		return nil, typ, fmt.Errorf("input type is not %T: %T", outType, in)
	}
	any, err := anypb.New(typ)
	if err != nil {
		return nil, typ, err
	}
	return any, typ, nil
}

func (s *Server) processResponse(
	peerName string,
	partition string,
	mutableStatus *MutableStatus,
	resp *pbpeerstream.ReplicationMessage_Response,
) (*pbpeerstream.ReplicationMessage, error) {
	if !pbpeerstream.KnownTypeURL(resp.ResourceURL) {
		err := fmt.Errorf("received response for unknown resource type %q", resp.ResourceURL)
		return makeNACKReply(
			resp.ResourceURL,
			resp.Nonce,
			code.Code_INVALID_ARGUMENT,
			err.Error(),
		), err
	}

	switch resp.Operation {
	case pbpeerstream.Operation_OPERATION_UPSERT:
		if resp.Resource == nil {
			err := fmt.Errorf("received upsert response with no content")
			return makeNACKReply(
				resp.ResourceURL,
				resp.Nonce,
				code.Code_INVALID_ARGUMENT,
				err.Error(),
			), err
		}

		if err := s.handleUpsert(peerName, partition, mutableStatus, resp.ResourceURL, resp.ResourceID, resp.Resource); err != nil {
			return makeNACKReply(
				resp.ResourceURL,
				resp.Nonce,
				code.Code_INTERNAL,
				fmt.Sprintf("upsert error, ResourceURL: %q, ResourceID: %q: %v", resp.ResourceURL, resp.ResourceID, err),
			), fmt.Errorf("upsert error: %w", err)
		}

		return makeACKReply(resp.ResourceURL, resp.Nonce), nil

	case pbpeerstream.Operation_OPERATION_DELETE:
		if err := s.handleDelete(peerName, partition, mutableStatus, resp.ResourceURL, resp.ResourceID); err != nil {
			return makeNACKReply(
				resp.ResourceURL,
				resp.Nonce,
				code.Code_INTERNAL,
				fmt.Sprintf("delete error, ResourceURL: %q, ResourceID: %q: %v", resp.ResourceURL, resp.ResourceID, err),
			), fmt.Errorf("delete error: %w", err)
		}
		return makeACKReply(resp.ResourceURL, resp.Nonce), nil

	default:
		var errMsg string
		if op := pbpeerstream.Operation_name[int32(resp.Operation)]; op != "" {
			errMsg = fmt.Sprintf("unsupported operation: %q", op)
		} else {
			errMsg = fmt.Sprintf("unsupported operation: %d", resp.Operation)
		}
		return makeNACKReply(
			resp.ResourceURL,
			resp.Nonce,
			code.Code_INVALID_ARGUMENT,
			errMsg,
		), errors.New(errMsg)
	}
}

func (s *Server) handleUpsert(
	peerName string,
	partition string,
	mutableStatus *MutableStatus,
	resourceURL string,
	resourceID string,
	resource *anypb.Any,
) error {
	if resource.TypeUrl != resourceURL {
		return fmt.Errorf("mismatched resourceURL %q and Any typeUrl %q", resourceURL, resource.TypeUrl)
	}

	switch resourceURL {
	case pbpeerstream.TypeURLExportedService:
		sn := structs.ServiceNameFromString(resourceID)
		sn.OverridePartition(partition)

		export := &pbpeerstream.ExportedService{}
		if err := resource.UnmarshalTo(export); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		err := s.handleUpdateService(peerName, partition, sn, export)
		if err != nil {
			return fmt.Errorf("did not increment imported services count for service=%q: %w", sn.String(), err)
		}

		mutableStatus.TrackImportedService(sn)

		return nil

	case pbpeerstream.TypeURLPeeringTrustBundle:
		roots := &pbpeering.PeeringTrustBundle{}
		if err := resource.UnmarshalTo(roots); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		return s.handleUpsertRoots(peerName, partition, roots)

	case pbpeerstream.TypeURLPeeringServerAddresses:
		addrs := &pbpeering.PeeringServerAddresses{}
		if err := resource.UnmarshalTo(addrs); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		return s.handleUpsertServerAddrs(peerName, partition, addrs)
	default:
		return fmt.Errorf("unexpected resourceURL: %s", resourceURL)
	}
}

// handleUpdateService handles both deletion and upsert events for a service.
//
//	On an UPSERT event:
//		- All nodes, services, checks in the input pbNodes are re-applied through Raft.
//		- Any nodes, services, or checks in the catalog that were not in the input pbNodes get deleted.
//
//	On a DELETE event:
//		- A reconciliation against nil or empty input pbNodes leads to deleting all stored catalog resources
//		  associated with the service name.
func (s *Server) handleUpdateService(
	peerName string,
	partition string,
	sn structs.ServiceName,
	export *pbpeerstream.ExportedService,
) error {
	// Capture instances in the state store for reconciliation later.
	_, storedInstances, err := s.GetStore().CheckServiceNodes(nil, sn.Name, &sn.EnterpriseMeta, peerName)
	if err != nil {
		return fmt.Errorf("failed to read imported services: %w", err)
	}

	structsNodes, err := export.CheckServiceNodesToStruct()
	if err != nil {
		return fmt.Errorf("failed to convert protobuf instances to structs: %w", err)
	}

	// Normalize the data into a convenient form for operation.
	snap := newHealthSnapshot(structsNodes, partition, peerName)

	for _, nodeSnap := range snap.Nodes {
		// First register the node
		req := nodeSnap.Node.ToRegisterRequest()
		if err := s.Backend.CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register node: %w", err)
		}

		// Then register all services on that node
		for _, svcSnap := range nodeSnap.Services {
			req.Service = svcSnap.Service
			if err := s.Backend.CatalogRegister(&req); err != nil {
				return fmt.Errorf("failed to register service: %w", err)
			}
		}
		req.Service = nil

		// Then register all checks on that node
		var chks structs.HealthChecks
		for _, svcSnap := range nodeSnap.Services {
			for _, c := range svcSnap.Checks {
				chks = append(chks, c)
			}
		}

		req.Checks = chks
		if err := s.Backend.CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register check: %w", err)
		}
	}

	//
	// Now that the data received has been stored in the state store, the rest of this
	// function is responsible for cleaning up data in the catalog that wasn't in the snapshot.
	//

	// nodeCheckTuple uniquely identifies a node check in the catalog.
	// The partition is not needed because we are only operating on one partition's catalog.
	type nodeCheckTuple struct {
		checkID types.CheckID
		node    string
	}

	var (
		// unusedNodes tracks node names that were not present in the latest response.
		// Missing nodes are not assumed to be deleted because there may be other service names
		// registered on them.
		// Inside we also track a map of node checks associated with the node.
		unusedNodes = make(map[string]struct{})

		// deletedNodeChecks tracks node checks that were not present in the latest response.
		// A single node check will be attached to all service instances of a node, so this
		// deduplication prevents issuing multiple deregistrations for a single check.
		deletedNodeChecks = make(map[nodeCheckTuple]struct{})
	)
	for _, csn := range storedInstances {
		if _, ok := snap.Nodes[csn.Node.Node]; !ok {
			unusedNodes[csn.Node.Node] = struct{}{}

			// Since the node is not in the snapshot we can know the associated service
			// instance is not in the snapshot either, since a service instance can't
			// exist without a node.
			// This will also delete all service checks.
			err := s.Backend.CatalogDeregister(&structs.DeregisterRequest{
				Node:           csn.Node.Node,
				ServiceID:      csn.Service.ID,
				EnterpriseMeta: csn.Service.EnterpriseMeta,
				PeerName:       peerName,
			})
			if err != nil {
				return fmt.Errorf("failed to deregister service %q: %w", csn.Service.CompoundServiceID(), err)
			}

			// We can't know if a node check was deleted from the exporting cluster
			// (but not the node itself) if the node wasn't in the snapshot,
			// so we do not loop over checks here.
			// If the unusedNode gets deleted below that will also delete node checks.
			continue
		}

		// Delete the service instance if not in the snapshot.
		sid := csn.Service.CompoundServiceID()
		if _, ok := snap.Nodes[csn.Node.Node].Services[sid]; !ok {
			err := s.Backend.CatalogDeregister(&structs.DeregisterRequest{
				Node:           csn.Node.Node,
				ServiceID:      csn.Service.ID,
				EnterpriseMeta: csn.Service.EnterpriseMeta,
				PeerName:       peerName,
			})
			if err != nil {
				ident := fmt.Sprintf("partition:%s/peer:%s/node:%s/ns:%s/service_id:%s",
					csn.Service.PartitionOrDefault(), peerName, csn.Node.Node, csn.Service.NamespaceOrDefault(), csn.Service.ID)
				return fmt.Errorf("failed to deregister service %q: %w", ident, err)
			}

			// When a service is deleted all associated checks also get deleted as a side effect.
			continue
		}

		// Reconcile checks.
		for _, chk := range csn.Checks {
			if _, ok := snap.Nodes[csn.Node.Node].Services[sid].Checks[chk.CheckID]; !ok {
				// Checks without a ServiceID are node checks.
				// If the node exists but the check does not then the check was deleted.
				if chk.ServiceID == "" {
					// Deduplicate node checks to avoid deregistering a check multiple times.
					tuple := nodeCheckTuple{
						checkID: chk.CheckID,
						node:    chk.Node,
					}
					deletedNodeChecks[tuple] = struct{}{}
					continue
				}

				// If the check isn't a node check then it's a service check.
				// Service checks that were not present can be deleted immediately because
				// checks for a given service ID will only be attached to a single CheckServiceNode.
				err := s.Backend.CatalogDeregister(&structs.DeregisterRequest{
					Node:           chk.Node,
					CheckID:        chk.CheckID,
					EnterpriseMeta: chk.EnterpriseMeta,
					PeerName:       peerName,
				})
				if err != nil {
					ident := fmt.Sprintf("partition:%s/peer:%s/node:%s/ns:%s/check_id:%s",
						chk.PartitionOrDefault(), peerName, chk.Node, chk.NamespaceOrDefault(), chk.CheckID)
					return fmt.Errorf("failed to deregister check %q: %w", ident, err)
				}
			}
		}
	}

	// Delete all deduplicated node checks.
	for chk := range deletedNodeChecks {
		nodeMeta := structs.NodeEnterpriseMetaInPartition(sn.PartitionOrDefault())
		err := s.Backend.CatalogDeregister(&structs.DeregisterRequest{
			Node:           chk.node,
			CheckID:        chk.checkID,
			EnterpriseMeta: *nodeMeta,
			PeerName:       peerName,
		})
		if err != nil {
			ident := fmt.Sprintf("partition:%s/peer:%s/node:%s/check_id:%s", nodeMeta.PartitionOrDefault(), peerName, chk.node, chk.checkID)
			return fmt.Errorf("failed to deregister node check %q: %w", ident, err)
		}
	}

	// Delete any nodes that do not have any other services registered on them.
	for node := range unusedNodes {
		nodeMeta := structs.NodeEnterpriseMetaInPartition(sn.PartitionOrDefault())
		_, ns, err := s.GetStore().NodeServices(nil, node, nodeMeta, peerName)
		if err != nil {
			return fmt.Errorf("failed to query services on node: %w", err)
		}
		if ns != nil && len(ns.Services) >= 1 {
			// At least one service is still registered on this node, so we keep it.
			continue
		}

		// All services on the node were deleted, so the node is also cleaned up.
		err = s.Backend.CatalogDeregister(&structs.DeregisterRequest{
			Node:           node,
			PeerName:       peerName,
			EnterpriseMeta: *nodeMeta,
		})
		if err != nil {
			ident := fmt.Sprintf("partition:%s/peer:%s/node:%s", nodeMeta.PartitionOrDefault(), peerName, node)
			return fmt.Errorf("failed to deregister node %q: %w", ident, err)
		}
	}
	return nil
}

func (s *Server) handleUpsertRoots(
	peerName string,
	partition string,
	trustBundle *pbpeering.PeeringTrustBundle,
) error {
	// We override the partition and peer name so that the trust bundle gets stored
	// in the importing partition with a reference to the peer it was imported from.
	trustBundle.Partition = partition
	trustBundle.PeerName = peerName
	req := &pbpeering.PeeringTrustBundleWriteRequest{
		PeeringTrustBundle: trustBundle,
	}
	return s.Backend.PeeringTrustBundleWrite(req)
}

func (s *Server) handleUpsertServerAddrs(
	peerName string,
	partition string,
	addrs *pbpeering.PeeringServerAddresses,
) error {
	q := state.Query{
		Value:          peerName,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(partition),
	}
	_, existing, err := s.GetStore().PeeringRead(nil, q)
	if err != nil {
		return fmt.Errorf("failed to read peering: %w", err)
	}
	if existing == nil || !existing.IsActive() {
		return fmt.Errorf("peering does not exist or has been marked for deletion")
	}

	// Clone to avoid mutating the existing data
	p := proto.Clone(existing).(*pbpeering.Peering)
	p.PeerServerAddresses = addrs.GetAddresses()

	req := &pbpeering.PeeringWriteRequest{
		Peering: p,
	}
	return s.Backend.PeeringWrite(req)
}

func (s *Server) handleDelete(
	peerName string,
	partition string,
	mutableStatus *MutableStatus,
	resourceURL string,
	resourceID string,
) error {
	switch resourceURL {
	case pbpeerstream.TypeURLExportedService:
		sn := structs.ServiceNameFromString(resourceID)
		sn.OverridePartition(partition)

		err := s.handleUpdateService(peerName, partition, sn, nil)
		if err != nil {
			return err
		}

		mutableStatus.RemoveImportedService(sn)

		return nil

	default:
		return fmt.Errorf("unexpected resourceURL: %s", resourceURL)
	}
}

func makeACKReply(resourceURL, nonce string) *pbpeerstream.ReplicationMessage {
	return makeReplicationRequest(&pbpeerstream.ReplicationMessage_Request{
		ResourceURL:   resourceURL,
		ResponseNonce: nonce,
	})
}

func makeNACKReply(resourceURL, nonce string, errCode code.Code, errMsg string) *pbpeerstream.ReplicationMessage {
	var rpcErr *pbstatus.Status
	if errCode != code.Code_OK || errMsg != "" {
		rpcErr = &pbstatus.Status{
			Code:    int32(errCode),
			Message: errMsg,
		}
	}

	return makeReplicationRequest(&pbpeerstream.ReplicationMessage_Request{
		ResourceURL:   resourceURL,
		ResponseNonce: nonce,
		Error:         rpcErr,
	})
}

// makeReplicationRequest is a convenience method to make a Request-type ReplicationMessage.
func makeReplicationRequest(req *pbpeerstream.ReplicationMessage_Request) *pbpeerstream.ReplicationMessage {
	return &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Request_{
			Request: req,
		},
	}
}

// makeReplicationResponse is a convenience method to make a Response-type ReplicationMessage.
func makeReplicationResponse(resp *pbpeerstream.ReplicationMessage_Response) *pbpeerstream.ReplicationMessage {
	return &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Response_{
			Response: resp,
		},
	}
}

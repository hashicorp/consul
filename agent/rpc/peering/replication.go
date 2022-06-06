package peering

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbstatus"
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
	logger hclog.Logger,
	update cache.UpdateEvent,
) *pbpeering.ReplicationMessage {
	any, csn, err := marshalToProtoAny[*pbservice.IndexedCheckServiceNodes](update.Result)
	if err != nil {
		// Log the error and skip this response to avoid locking up peering due to a bad update event.
		logger.Error("failed to marshal", "error", err)
		return nil
	}

	var serviceName string
	if strings.HasPrefix(update.CorrelationID, subExportedService) {
		serviceName = strings.TrimPrefix(update.CorrelationID, subExportedService)
	} else {
		serviceName = strings.TrimPrefix(update.CorrelationID, subExportedProxyService) + syntheticProxyNameSuffix
	}

	// If no nodes are present then it's due to one of:
	// 1. The service is newly registered or exported and yielded a transient empty update.
	// 2. All instances of the service were de-registered.
	// 3. The service was un-exported.
	//
	// We don't distinguish when these three things occurred, but it's safe to send a DELETE Op in all cases, so we do that.
	// Case #1 is a no-op for the importing peer.
	if len(csn.Nodes) == 0 {
		resp := &pbpeering.ReplicationMessage{
			Payload: &pbpeering.ReplicationMessage_Response_{
				Response: &pbpeering.ReplicationMessage_Response{
					ResourceURL: pbpeering.TypeURLService,
					// TODO(peering): Nonce management
					Nonce:      "",
					ResourceID: serviceName,
					Operation:  pbpeering.ReplicationMessage_Response_DELETE,
				},
			},
		}
		return resp
	}

	// If there are nodes in the response, we push them as an UPSERT operation.
	resp := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Response_{
			Response: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLService,
				// TODO(peering): Nonce management
				Nonce:      "",
				ResourceID: serviceName,
				Operation:  pbpeering.ReplicationMessage_Response_UPSERT,
				Resource:   any,
			},
		},
	}
	return resp
}

func makeCARootsResponse(
	logger hclog.Logger,
	update cache.UpdateEvent,
) *pbpeering.ReplicationMessage {
	any, _, err := marshalToProtoAny[*pbpeering.PeeringTrustBundle](update.Result)
	if err != nil {
		// Log the error and skip this response to avoid locking up peering due to a bad update event.
		logger.Error("failed to marshal", "error", err)
		return nil
	}

	resp := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Response_{
			Response: &pbpeering.ReplicationMessage_Response{
				ResourceURL: pbpeering.TypeURLRoots,
				// TODO(peering): Nonce management
				Nonce:      "",
				ResourceID: "roots",
				Operation:  pbpeering.ReplicationMessage_Response_UPSERT,
				Resource:   any,
			},
		},
	}
	return resp
}

// marshalToProtoAny takes any input and returns:
// the protobuf.Any type, the asserted T type, and any errors
// during marshalling or type assertion.
// `in` MUST be of type T or it returns an error.
func marshalToProtoAny[T proto.Message](in any) (*anypb.Any, T, error) {
	typ, ok := in.(T)
	if !ok {
		var outType T
		return nil, typ, fmt.Errorf("input type is not %T: %T", outType, in)
	}
	any, err := ptypes.MarshalAny(typ)
	if err != nil {
		return nil, typ, err
	}
	return any, typ, nil
}

func (s *Service) processResponse(
	peerName string,
	partition string,
	resp *pbpeering.ReplicationMessage_Response,
) (*pbpeering.ReplicationMessage, error) {
	if !pbpeering.KnownTypeURL(resp.ResourceURL) {
		err := fmt.Errorf("received response for unknown resource type %q", resp.ResourceURL)
		return makeReply(
			resp.ResourceURL,
			resp.Nonce,
			code.Code_INVALID_ARGUMENT,
			err.Error(),
		), err
	}

	switch resp.Operation {
	case pbpeering.ReplicationMessage_Response_UPSERT:
		if resp.Resource == nil {
			err := fmt.Errorf("received upsert response with no content")
			return makeReply(
				resp.ResourceURL,
				resp.Nonce,
				code.Code_INVALID_ARGUMENT,
				err.Error(),
			), err
		}

		if err := s.handleUpsert(peerName, partition, resp.ResourceURL, resp.ResourceID, resp.Resource); err != nil {
			return makeReply(
				resp.ResourceURL,
				resp.Nonce,
				code.Code_INTERNAL,
				fmt.Sprintf("upsert error, ResourceURL: %q, ResourceID: %q: %v", resp.ResourceURL, resp.ResourceID, err),
			), fmt.Errorf("upsert error: %w", err)
		}

		return makeReply(resp.ResourceURL, resp.Nonce, code.Code_OK, ""), nil

	case pbpeering.ReplicationMessage_Response_DELETE:
		if err := s.handleDelete(peerName, partition, resp.ResourceURL, resp.ResourceID); err != nil {
			return makeReply(
				resp.ResourceURL,
				resp.Nonce,
				code.Code_INTERNAL,
				fmt.Sprintf("delete error, ResourceURL: %q, ResourceID: %q: %v", resp.ResourceURL, resp.ResourceID, err),
			), fmt.Errorf("delete error: %w", err)
		}
		return makeReply(resp.ResourceURL, resp.Nonce, code.Code_OK, ""), nil

	default:
		var errMsg string
		if op := pbpeering.ReplicationMessage_Response_Operation_name[int32(resp.Operation)]; op != "" {
			errMsg = fmt.Sprintf("unsupported operation: %q", op)
		} else {
			errMsg = fmt.Sprintf("unsupported operation: %d", resp.Operation)
		}
		return makeReply(
			resp.ResourceURL,
			resp.Nonce,
			code.Code_INVALID_ARGUMENT,
			errMsg,
		), errors.New(errMsg)
	}
}

func (s *Service) handleUpsert(
	peerName string,
	partition string,
	resourceURL string,
	resourceID string,
	resource *anypb.Any,
) error {
	switch resourceURL {
	case pbpeering.TypeURLService:
		sn := structs.ServiceNameFromString(resourceID)
		sn.OverridePartition(partition)

		csn := &pbservice.IndexedCheckServiceNodes{}
		if err := ptypes.UnmarshalAny(resource, csn); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		return s.handleUpsertService(peerName, partition, sn, csn)

	case pbpeering.TypeURLRoots:
		roots := &pbpeering.PeeringTrustBundle{}
		if err := ptypes.UnmarshalAny(resource, roots); err != nil {
			return fmt.Errorf("failed to unmarshal resource: %w", err)
		}

		return s.handleUpsertRoots(peerName, partition, roots)

	default:
		return fmt.Errorf("unexpected resourceURL: %s", resourceURL)
	}
}

func (s *Service) handleUpsertService(
	peerName string,
	partition string,
	sn structs.ServiceName,
	csn *pbservice.IndexedCheckServiceNodes,
) error {
	if csn == nil || len(csn.Nodes) == 0 {
		return s.handleDeleteService(peerName, partition, sn)
	}

	// Convert exported data into structs format.
	structsNodes := make([]structs.CheckServiceNode, 0, len(csn.Nodes))
	for _, pb := range csn.Nodes {
		instance, err := pbservice.CheckServiceNodeToStructs(pb)
		if err != nil {
			return fmt.Errorf("failed to convert instance: %w", err)
		}
		structsNodes = append(structsNodes, *instance)
	}

	// Normalize the data into a convenient form for operation.
	snap := newHealthSnapshot(structsNodes, partition, peerName)

	for _, nodeSnap := range snap.Nodes {
		// First register the node
		req := nodeSnap.Node.ToRegisterRequest()
		if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register node: %w", err)
		}

		// Then register all services on that node
		for _, svcSnap := range nodeSnap.Services {
			req.Service = svcSnap.Service
			if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
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
		if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register check: %w", err)
		}
	}

	// TODO(peering): cleanup and deregister existing data that is now missing safely somehow

	return nil
}

func (s *Service) handleUpsertRoots(
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
	return s.Backend.Apply().PeeringTrustBundleWrite(req)
}

func (s *Service) handleDelete(
	peerName string,
	partition string,
	resourceURL string,
	resourceID string,
) error {
	switch resourceURL {
	case pbpeering.TypeURLService:
		sn := structs.ServiceNameFromString(resourceID)
		sn.OverridePartition(partition)
		return s.handleDeleteService(peerName, partition, sn)

	default:
		return fmt.Errorf("unexpected resourceURL: %s", resourceURL)
	}
}

func (s *Service) handleDeleteService(
	peerName string,
	partition string,
	sn structs.ServiceName,
) error {
	// Deregister: ServiceID == DeleteService ANd checks
	// Deregister: ServiceID(empty) CheckID(empty) == DeleteNode

	// TODO(peering): implement
	return nil
}

func makeReply(resourceURL, nonce string, errCode code.Code, errMsg string) *pbpeering.ReplicationMessage {
	var rpcErr *pbstatus.Status
	if errCode != code.Code_OK || errMsg != "" {
		rpcErr = &pbstatus.Status{
			Code:    int32(errCode),
			Message: errMsg,
		}
	}

	// TODO: shouldn't this be response?
	return &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				ResourceURL: resourceURL,
				Nonce:       nonce,
				Error:       rpcErr,
			},
		},
	}
}

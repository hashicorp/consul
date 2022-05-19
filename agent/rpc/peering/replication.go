package peering

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/proto/pbservice"
	"github.com/hashicorp/consul/proto/pbstatus"
	"github.com/hashicorp/consul/types"
)

// pushService response handles sending exported service instance updates to the peer cluster.
// Each cache.UpdateEvent will contain all instances for a service name.
// If there are no instances in the event, we consider that to be a de-registration.
func pushServiceResponse(logger hclog.Logger, stream BidirectionalStream, status *lockableStreamStatus, update cache.UpdateEvent) error {
	csn, ok := update.Result.(*pbservice.IndexedCheckServiceNodes)
	if !ok {
		logger.Error(fmt.Sprintf("invalid type for response: %T, expected *pbservice.IndexedCheckServiceNodes", update.Result))

		// Skip this update to avoid locking up peering due to a bad service update.
		return nil
	}
	serviceName := strings.TrimPrefix(update.CorrelationID, subExportedService)

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
		logTraceSend(logger, resp)
		if err := stream.Send(resp); err != nil {
			status.trackSendError(err.Error())
			return fmt.Errorf("failed to send to stream: %v", err)
		}
		return nil
	}

	// If there are nodes in the response, we push them as an UPSERT operation.
	any, err := ptypes.MarshalAny(csn)
	if err != nil {
		// Log the error and skip this response to avoid locking up peering due to a bad update event.
		logger.Error("failed to marshal service endpoints", "error", err)
		return nil
	}
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
	logTraceSend(logger, resp)
	if err := stream.Send(resp); err != nil {
		status.trackSendError(err.Error())
		return fmt.Errorf("failed to send to stream: %v", err)
	}
	return nil
}

func (s *Service) processResponse(peerName string, partition string, resp *pbpeering.ReplicationMessage_Response) (*pbpeering.ReplicationMessage, error) {
	var (
		err     error
		errCode code.Code
		errMsg  string
	)

	if resp.ResourceURL != pbpeering.TypeURLService {
		errCode = code.Code_INVALID_ARGUMENT
		err = fmt.Errorf("received response for unknown resource type %q", resp.ResourceURL)
		return makeReply(resp.ResourceURL, resp.Nonce, errCode, err.Error()), err
	}

	switch resp.Operation {
	case pbpeering.ReplicationMessage_Response_UPSERT:
		if resp.Resource == nil {
			break
		}
		err = s.handleUpsert(peerName, partition, resp.ResourceURL, resp.ResourceID, resp.Resource)
		if err != nil {
			errCode = code.Code_INTERNAL
			errMsg = err.Error()
		}

	case pbpeering.ReplicationMessage_Response_DELETE:
		err = handleDelete(resp.ResourceURL, resp.ResourceID)
		if err != nil {
			errCode = code.Code_INTERNAL
			errMsg = err.Error()
		}

	default:
		errCode = code.Code_INVALID_ARGUMENT

		op := pbpeering.ReplicationMessage_Response_Operation_name[int32(resp.Operation)]
		if op == "" {
			op = strconv.FormatInt(int64(resp.Operation), 10)
		}
		errMsg = fmt.Sprintf("unsupported operation: %q", op)

		err = errors.New(errMsg)
	}

	return makeReply(resp.ResourceURL, resp.Nonce, errCode, errMsg), err
}

func (s *Service) handleUpsert(peerName string, partition string, resourceURL string, resourceID string, resource *anypb.Any) error {
	csn := &pbservice.IndexedCheckServiceNodes{}
	err := ptypes.UnmarshalAny(resource, csn)
	if err != nil {
		return fmt.Errorf("failed to unmarshal resource, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
	}
	if csn == nil || len(csn.Nodes) == 0 {
		return nil
	}

	type checkTuple struct {
		checkID   types.CheckID
		serviceID string
		nodeID    types.NodeID

		acl.EnterpriseMeta
	}

	var (
		nodes    = make(map[types.NodeID]*structs.Node)
		services = make(map[types.NodeID][]*structs.NodeService)
		checks   = make(map[types.NodeID]map[checkTuple]*structs.HealthCheck)
	)

	for _, pbinstance := range csn.Nodes {
		instance, err := pbservice.CheckServiceNodeToStructs(pbinstance)
		if err != nil {
			return fmt.Errorf("failed to convert instance, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
		}

		nodes[instance.Node.ID] = instance.Node
		services[instance.Node.ID] = append(services[instance.Node.ID], instance.Service)

		if _, ok := checks[instance.Node.ID]; !ok {
			checks[instance.Node.ID] = make(map[checkTuple]*structs.HealthCheck)
		}
		for _, c := range instance.Checks {
			tuple := checkTuple{
				checkID:        c.CheckID,
				serviceID:      c.ServiceID,
				nodeID:         instance.Node.ID,
				EnterpriseMeta: c.EnterpriseMeta,
			}
			checks[instance.Node.ID][tuple] = c
		}
	}

	for nodeID, node := range nodes {
		// For all nodes, services, and checks we override the peer name and partition to be
		// the local partition and local name for the peer.
		node.PeerName, node.Partition = peerName, partition

		// First register the node
		req := node.ToRegisterRequest()
		if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
		}

		// Then register all services on that node
		for _, svc := range services[nodeID] {
			svc.PeerName = peerName
			svc.OverridePartition(partition)

			req.Service = svc
			if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
				return fmt.Errorf("failed to register, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
			}
		}
		req.Service = nil

		// Then register all checks on that node
		var chks structs.HealthChecks
		for _, c := range checks[nodeID] {
			c.PeerName = peerName
			c.OverridePartition(partition)

			chks = append(chks, c)
		}

		req.Checks = chks
		if err := s.Backend.Apply().CatalogRegister(&req); err != nil {
			return fmt.Errorf("failed to register, ResourceURL: %q, ResourceID: %q, err: %w", resourceURL, resourceID, err)
		}
	}
	return nil
}

func handleDelete(resourceURL string, resourceID string) error {
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

	msg := &pbpeering.ReplicationMessage{
		Payload: &pbpeering.ReplicationMessage_Request_{
			Request: &pbpeering.ReplicationMessage_Request{
				ResourceURL: resourceURL,
				Nonce:       nonce,
				Error:       rpcErr,
			},
		},
	}
	return msg
}

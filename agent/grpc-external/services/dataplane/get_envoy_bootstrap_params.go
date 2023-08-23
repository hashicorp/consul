package dataplane

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	acl "github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	external "github.com/hashicorp/consul/agent/grpc-external"
	structs "github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

func (s *Server) GetEnvoyBootstrapParams(ctx context.Context, req *pbdataplane.GetEnvoyBootstrapParamsRequest) (*pbdataplane.GetEnvoyBootstrapParamsResponse, error) {
	logger := s.Logger.Named("get-envoy-bootstrap-params").With("service_id", req.GetServiceId(), "request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	token := external.TokenFromContext(ctx)
	var authzContext acl.AuthorizerContext
	entMeta := acl.NewEnterpriseMetaWithPartition(req.GetPartition(), req.GetNamespace())
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(token, &entMeta, &authzContext)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	store := s.GetStore()

	_, svc, err := store.ServiceNode(req.GetNodeId(), req.GetNodeName(), req.GetServiceId(), &entMeta, structs.DefaultPeerKeyword)
	if err != nil {
		logger.Error("Error looking up service", "error", err)
		if errors.Is(err, state.ErrNodeNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		} else if strings.Contains(err.Error(), "Node ID or name required") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		} else {
			return nil, status.Error(codes.Internal, "Failure looking up service")
		}
	}
	if svc == nil {
		return nil, status.Error(codes.NotFound, "Service not found")
	}

	if err := authz.ToAllowAuthorizer().ServiceReadAllowed(svc.ServiceName, &authzContext); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	// Build out the response

	resp := &pbdataplane.GetEnvoyBootstrapParamsResponse{
		Service:     svc.ServiceProxy.DestinationServiceName,
		Partition:   svc.EnterpriseMeta.PartitionOrDefault(),
		Namespace:   svc.EnterpriseMeta.NamespaceOrDefault(),
		Datacenter:  s.Datacenter,
		ServiceKind: convertToResponseServiceKind(svc.ServiceKind),
	}

	bootstrapConfig, err := structpb.NewStruct(svc.ServiceProxy.Config)
	if err != nil {
		logger.Error("Error creating the envoy boostrap params config", "error", err)
		return nil, status.Error(codes.Unknown, "Error creating the envoy boostrap params config")
	}
	resp.Config = bootstrapConfig

	return resp, nil
}

func convertToResponseServiceKind(serviceKind structs.ServiceKind) (respKind pbdataplane.ServiceKind) {
	switch serviceKind {
	case structs.ServiceKindConnectProxy:
		respKind = pbdataplane.ServiceKind_SERVICE_KIND_CONNECT_PROXY
	case structs.ServiceKindMeshGateway:
		respKind = pbdataplane.ServiceKind_SERVICE_KIND_MESH_GATEWAY
	case structs.ServiceKindTerminatingGateway:
		respKind = pbdataplane.ServiceKind_SERVICE_KIND_TERMINATING_GATEWAY
	case structs.ServiceKindIngressGateway:
		respKind = pbdataplane.ServiceKind_SERVICE_KIND_INGRESS_GATEWAY
	case structs.ServiceKindTypical:
		respKind = pbdataplane.ServiceKind_SERVICE_KIND_TYPICAL
	}
	return
}

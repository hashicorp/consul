package dataplane

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/state"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

func (s *Server) GetEnvoyBootstrapParams(ctx context.Context, req *pbdataplane.GetEnvoyBootstrapParamsRequest) (*pbdataplane.GetEnvoyBootstrapParamsResponse, error) {
	logger := s.Logger.Named("get-envoy-bootstrap-params").With("service_id", req.GetServiceId(), "request_id", external.TraceID())

	logger.Trace("Started processing request")
	defer logger.Trace("Finished processing request")

	options, err := external.QueryOptionsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var authzContext acl.AuthorizerContext
	entMeta := acl.NewEnterpriseMetaWithPartition(req.GetPartition(), req.GetNamespace())
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(options.Token, &entMeta, &authzContext)
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
	var serviceName string
	if svc.ServiceKind == structs.ServiceKindConnectProxy {
		serviceName = svc.ServiceProxy.DestinationServiceName
	} else {
		serviceName = svc.ServiceName
	}

	resp := &pbdataplane.GetEnvoyBootstrapParamsResponse{
		Service:     serviceName,
		Partition:   svc.EnterpriseMeta.PartitionOrDefault(),
		Namespace:   svc.EnterpriseMeta.NamespaceOrDefault(),
		Datacenter:  s.Datacenter,
		ServiceKind: convertToResponseServiceKind(svc.ServiceKind),
		NodeName:    svc.Node,
		NodeId:      string(svc.ID),
	}

	// This is awkward because it's designed for different requests, but
	// this fakes the ServiceSpecificRequest so that we can reuse code.
	_, ns, err := configentry.MergeNodeServiceWithCentralConfig(
		nil,
		store,
		&structs.ServiceSpecificRequest{
			Datacenter:   s.Datacenter,
			QueryOptions: options,
		},
		svc.ToNodeService(),
		logger,
	)
	if err != nil {
		logger.Error("Error merging with central config", "error", err)
		return nil, status.Errorf(codes.Unknown, "Error merging central config: %v", err)
	}

	bootstrapConfig, err := structpb.NewStruct(ns.Proxy.Config)
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

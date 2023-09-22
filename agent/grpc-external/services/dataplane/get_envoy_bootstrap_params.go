// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dataplane

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/state"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/accesslogs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

func (s *Server) GetEnvoyBootstrapParams(ctx context.Context, req *pbdataplane.GetEnvoyBootstrapParamsRequest) (*pbdataplane.GetEnvoyBootstrapParamsResponse, error) {
	proxyID := req.ProxyId
	if req.GetServiceId() != "" {
		proxyID = req.GetServiceId()
	}
	logger := s.Logger.Named("get-envoy-bootstrap-params").With("proxy_id", proxyID, "request_id", external.TraceID())

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

	if s.EnableV2 {
		// Get the workload.
		workloadId := &pbresource.ID{
			Name: proxyID,
			Tenancy: &pbresource.Tenancy{
				Namespace: req.Namespace,
				Partition: req.Partition,
			},
			Type: pbcatalog.WorkloadType,
		}
		workloadRsp, err := s.ResourceAPIClient.Read(ctx, &pbresource.ReadRequest{
			Id: workloadId,
		})
		if err != nil {
			// This error should already include the gRPC status code and so we don't need to wrap it
			// in status.Error.
			logger.Error("Error looking up workload", "error", err)
			return nil, err
		}
		var workload pbcatalog.Workload
		err = workloadRsp.Resource.Data.UnmarshalTo(&workload)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to parse workload data")
		}

		// Only workloads that have an associated identity can ask for proxy bootstrap parameters.
		if workload.Identity == "" {
			return nil, status.Errorf(codes.InvalidArgument, "workload %q doesn't have identity associated with it", req.ProxyId)
		}

		// todo (ishustava): ACL enforcement ensuring there's identity:write permissions.

		// Get all proxy configurations for this workload. Currently we're only looking
		// for proxy configurations in the same tenancy as the workload.
		// todo (ishustava): we need to support wildcard proxy configurations as well.

		proxyCfgList, err := s.ResourceAPIClient.List(ctx, &pbresource.ListRequest{
			Tenancy: workloadRsp.Resource.Id.GetTenancy(),
			Type:    pbmesh.ProxyConfigurationType,
		})
		if err != nil {
			logger.Error("Error looking up proxyConfiguration", "error", err)
			return nil, err
		}

		// Collect and merge proxy configs.
		// todo (ishustava): sorting and conflict resolution.
		bootstrapCfg := &pbmesh.BootstrapConfig{}
		dynamicCfg := &pbmesh.DynamicConfig{}
		for _, cfgResource := range proxyCfgList.Resources {
			var proxyCfg pbmesh.ProxyConfiguration
			err = cfgResource.Data.UnmarshalTo(&proxyCfg)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to parse proxy configuration data: %q", cfgResource.Id.Name)
			}
			if isWorkloadSelected(req.ProxyId, proxyCfg.Workloads) {
				proto.Merge(bootstrapCfg, proxyCfg.BootstrapConfig)
				proto.Merge(dynamicCfg, proxyCfg.DynamicConfig)
			}
		}

		accessLogs := makeAccessLogs(dynamicCfg.GetAccessLogs(), logger)

		return &pbdataplane.GetEnvoyBootstrapParamsResponse{
			Identity:        workload.Identity,
			Partition:       workloadRsp.Resource.Id.Tenancy.Partition,
			Namespace:       workloadRsp.Resource.Id.Tenancy.Namespace,
			BootstrapConfig: bootstrapCfg,
			Datacenter:      s.Datacenter,
			NodeName:        workload.NodeName,
			AccessLogs:      accessLogs,
		}, nil
	}

	// The remainder of this file focuses on v1 implementation of this endpoint.

	store := s.GetStore()

	_, svc, err := store.ServiceNode(req.GetNodeId(), req.GetNodeName(), proxyID, &entMeta, structs.DefaultPeerKeyword)
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

	_, ns, err := configentry.MergeNodeServiceWithCentralConfig(
		nil,
		store,
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

	// Inspect access logging
	// This is non-essential, and don't want to return an error unless there is a more serious issue
	var accessLogs []string
	if ns != nil {
		accessLogs = makeAccessLogs(&ns.Proxy.AccessLogs, logger)
	}

	// Build out the response
	var serviceName string
	if svc.ServiceKind == structs.ServiceKindConnectProxy {
		serviceName = svc.ServiceProxy.DestinationServiceName
	} else {
		serviceName = svc.ServiceName
	}

	return &pbdataplane.GetEnvoyBootstrapParamsResponse{
		Identity:   serviceName,
		Service:    serviceName,
		Partition:  svc.EnterpriseMeta.PartitionOrDefault(),
		Namespace:  svc.EnterpriseMeta.NamespaceOrDefault(),
		Config:     bootstrapConfig,
		Datacenter: s.Datacenter,
		NodeName:   svc.Node,
		AccessLogs: accessLogs,
	}, nil
}

func makeAccessLogs(logs structs.AccessLogs, logger hclog.Logger) []string {
	var accessLogs []string
	if logs.GetEnabled() {
		envoyLoggers, err := accesslogs.MakeAccessLogs(logs, false)
		if err != nil {
			logger.Warn("Error creating the envoy access log config", "error", err)
		}

		accessLogs = make([]string, 0, len(envoyLoggers))

		for _, msg := range envoyLoggers {
			logConfig, err := protojson.Marshal(msg)
			if err != nil {
				logger.Warn("Error marshaling the envoy access log config", "error", err)
			}
			accessLogs = append(accessLogs, string(logConfig))
		}
	}

	return accessLogs
}

func isWorkloadSelected(name string, selector *pbcatalog.WorkloadSelector) bool {
	for _, prefix := range selector.Prefixes {
		if strings.Contains(name, prefix) {
			return true
		}
	}

	for _, selectorName := range selector.Names {
		if name == selectorName {
			return true
		}
	}

	return false
}

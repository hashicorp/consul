// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"context"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/blockingquery"
	"github.com/hashicorp/consul/agent/consul/state"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
)

// Server implements pbconfigentry.ConfigEntryService to provide RPC operations related to
// configentries
type Server struct {
	Config
}

type Config struct {
	Backend    Backend
	Logger     hclog.Logger
	ForwardRPC func(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error)
	FSMServer  blockingquery.FSMServer
}

type Backend interface {
	EnterpriseCheckPartitions(partition string) error

	ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error)
}

func NewServer(cfg Config) *Server {
	external.RequireNotNil(cfg.Backend, "Backend")
	external.RequireNotNil(cfg.Logger, "Logger")
	external.RequireNotNil(cfg.FSMServer, "FSMServer")

	return &Server{
		Config: cfg,
	}
}

var _ pbconfigentry.ConfigEntryServiceServer = (*Server)(nil)

type readRequest struct {
	structs.QueryOptions
	structs.DCSpecificRequest
}

func (s *Server) Register(grpcServer grpc.ServiceRegistrar) {
	pbconfigentry.RegisterConfigEntryServiceServer(grpcServer, s)
}

func (s *Server) GetResolvedExportedServices(
	ctx context.Context,
	req *pbconfigentry.GetResolvedExportedServicesRequest,
) (*pbconfigentry.GetResolvedExportedServicesResponse, error) {

	if err := s.Backend.EnterpriseCheckPartitions(req.Partition); err != nil {
		return nil, grpcstatus.Error(codes.InvalidArgument, err.Error())
	}

	options, err := external.QueryOptionsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var resp *pbconfigentry.GetResolvedExportedServicesResponse
	var emptyDCSpecificRequest structs.DCSpecificRequest

	handled, err := s.ForwardRPC(&readRequest{options, emptyDCSpecificRequest}, func(conn *grpc.ClientConn) error {
		var err error
		resp, err = pbconfigentry.NewConfigEntryServiceClient(conn).GetResolvedExportedServices(ctx, req)
		return err
	})
	if handled || err != nil {
		return resp, err
	}

	defer metrics.MeasureSince([]string{"configentry", "get_resolved_exported_services"}, time.Now())

	var authzCtx acl.AuthorizerContext
	entMeta := structs.DefaultEnterpriseMetaInPartition(req.Partition)

	authz, err := s.Backend.ResolveTokenAndDefaultMeta(options.Token, entMeta, &authzCtx)
	if err != nil {
		return nil, err
	}

	if err := authz.ToAllowAuthorizer().MeshReadAllowed(&authzCtx); err != nil {
		return nil, err
	}

	res := &pbconfigentry.GetResolvedExportedServicesResponse{}
	meta := structs.QueryMeta{}
	err = blockingquery.Query(s.FSMServer, &options, &meta, func(ws memdb.WatchSet, store *state.Store) error {
		idx, exportedSvcs, err := store.ResolvedExportedServices(ws, entMeta)
		if err != nil {
			return err
		}

		meta.SetIndex(idx)

		res.Services = exportedSvcs
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error executing exported services blocking query: %w", err)
	}

	header, err := external.GRPCMetadataFromQueryMeta(meta)
	if err != nil {
		return nil, fmt.Errorf("could not convert query metadata to gRPC header")
	}
	if err := grpc.SendHeader(ctx, header); err != nil {
		return nil, fmt.Errorf("could not send gRPC header")
	}

	return res, nil
}

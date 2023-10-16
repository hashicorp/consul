// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/rpc/operator"
	"github.com/hashicorp/consul/proto/private/pboperator"
	"github.com/hashicorp/raft"
)

type OperatorBackend struct {
	srv *Server
}

// NewOperatorBackend returns a operator.Backend implementation that is bound to the given server.
func NewOperatorBackend(srv *Server) *OperatorBackend {
	return &OperatorBackend{
		srv: srv,
	}
}

func (op *OperatorBackend) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error) {
	res, err := op.srv.ResolveTokenAndDefaultMeta(token, entMeta, authzCtx)
	if err != nil {
		return resolver.Result{}, err
	}
	if err := op.srv.validateEnterpriseToken(res.ACLIdentity); err != nil {
		return resolver.Result{}, err
	}
	return res, err
}

func (op *OperatorBackend) TransferLeader(_ context.Context, request *pboperator.TransferLeaderRequest) (*pboperator.TransferLeaderResponse, error) {
	reply := new(pboperator.TransferLeaderResponse)
	err := op.srv.attemptLeadershipTransfer(raft.ServerID(request.ID))
	reply.Success = err == nil
	return reply, err
}

var _ operator.Backend = (*OperatorBackend)(nil)

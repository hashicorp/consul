package consul

import (
	"context"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/rpc/operator"
	"github.com/hashicorp/consul/proto/pboperator"
	"github.com/hashicorp/raft"
	"sync"
)

type OperatorBackend struct {
	// TODO(peering): accept a smaller interface; maybe just funcs from the server that we actually need: DC, IsLeader, etc
	srv *Server

	leaderAddrLock sync.RWMutex
	leaderAddr     string
}

// NewOperatorBackend returns a peering.Backend implementation that is bound to the given server.
func NewOperatorBackend(srv *Server) *OperatorBackend {
	return &OperatorBackend{
		srv: srv,
	}
}

func (op OperatorBackend) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzCtx *acl.AuthorizerContext) (resolver.Result, error) {
	return op.srv.ResolveTokenAndDefaultMeta(token, entMeta, authzCtx)
}

func (op OperatorBackend) TransferLeader(ctx context.Context, request *pboperator.TransferLeaderRequest) (*pboperator.TransferLeaderResponse, error) {
	reply := new(pboperator.TransferLeaderResponse)
	err := op.srv.attemptLeadershipTransfer(raft.ServerID(request.ID))
	reply.Success = err == nil
	return reply, err
}

var _ operator.Backend = (*OperatorBackend)(nil)

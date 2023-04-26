// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-internal/services/subscribe"
	"github.com/hashicorp/consul/agent/structs"
)

type subscribeBackend struct {
	srv      *Server
	connPool GRPCClientConner
}

// TODO: refactor Resolve methods to an ACLBackend that can be used by all
// the endpoints.
func (s subscribeBackend) ResolveTokenAndDefaultMeta(
	token string,
	entMeta *acl.EnterpriseMeta,
	authzContext *acl.AuthorizerContext,
) (acl.Authorizer, error) {
	return s.srv.ResolveTokenAndDefaultMeta(token, entMeta, authzContext)
}

var _ subscribe.Backend = (*subscribeBackend)(nil)

func (s subscribeBackend) Forward(info structs.RPCInfo, f func(*grpc.ClientConn) error) (handled bool, err error) {
	return s.srv.ForwardGRPC(s.connPool, info, f)
}

func (s subscribeBackend) Subscribe(req *stream.SubscribeRequest) (*stream.Subscription, error) {
	return s.srv.publisher.Subscribe(req)
}

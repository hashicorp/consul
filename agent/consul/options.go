// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"google.golang.org/grpc"

	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
)

type Deps struct {
	EventPublisher   *stream.EventPublisher
	Logger           hclog.InterceptLogger
	TLSConfigurator  *tlsutil.Configurator
	Tokens           *token.Store
	Router           *router.Router
	ConnPool         *pool.ConnPool
	GRPCConnPool     GRPCClientConner
	LeaderForwarder  LeaderForwarder
	XDSStreamLimiter *limiter.SessionLimiter
	// GetNetRPCInterceptorFunc, if not nil, sets the net/rpc rpc.ServerServiceCallInterceptor on
	// the server side to record metrics around the RPC requests. If nil, no interceptor is added to
	// the rpc server.
	GetNetRPCInterceptorFunc func(recorder *middleware.RequestRecorder) rpc.ServerServiceCallInterceptor
	// NewRequestRecorderFunc provides a middleware.RequestRecorder for the server to use; it cannot be nil
	NewRequestRecorderFunc func(logger hclog.Logger, isLeader func() bool, localDC string) *middleware.RequestRecorder

	// HCP contains the dependencies required when integrating with the HashiCorp Cloud Platform
	HCP hcp.Deps

	Experiments []string

	EnterpriseDeps
}

type GRPCClientConner interface {
	ClientConn(datacenter string) (*grpc.ClientConn, error)
	ClientConnLeader() (*grpc.ClientConn, error)
	SetGatewayResolver(func(string) string)
}

type LeaderForwarder interface {
	// UpdateLeaderAddr updates the leader address in the local DC's resolver.
	UpdateLeaderAddr(datacenter, addr string)
}

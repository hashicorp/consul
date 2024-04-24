// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/lib/stringslice"

	"github.com/hashicorp/consul-net-rpc/net/rpc"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/tlsutil"
)

type Deps struct {
	LeafCertManager  *leafcert.Manager
	EventPublisher   *stream.EventPublisher
	Logger           hclog.InterceptLogger
	TLSConfigurator  *tlsutil.Configurator
	Tokens           *token.Store
	Router           *router.Router
	ConnPool         *pool.ConnPool
	GRPCConnPool     GRPCClientConner
	LeaderForwarder  LeaderForwarder
	XDSStreamLimiter *limiter.SessionLimiter
	Registry         resource.Registry
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

// UseV1DNS returns true if "v1dns" is present in the Experiments
// array of the agent config. It is ignored if the v2 resource APIs are enabled.
func (d Deps) UseV1DNS() bool {
	if stringslice.Contains(d.Experiments, V1DNSExperimentName) && !d.UseV2Resources() {
		return true
	}
	return false
}

// UseV2Resources returns true if "resource-apis" is present in the Experiments
// array of the agent config.
func (d Deps) UseV2Resources() bool {
	if stringslice.Contains(d.Experiments, CatalogResourceExperimentName) {
		return true
	}
	return false
}

// UseV2Tenancy returns true if "v2tenancy" is present in the Experiments
// array of the agent config.
func (d Deps) UseV2Tenancy() bool {
	if stringslice.Contains(d.Experiments, V2TenancyExperimentName) {
		return true
	}
	return false
}

// HCPAllowV2Resources returns true if "hcp-v2-resource-apis" is present in the Experiments
// array of the agent config.
func (d Deps) HCPAllowV2Resources() bool {
	if stringslice.Contains(d.Experiments, HCPAllowV2ResourceAPIs) {
		return true
	}
	return false
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

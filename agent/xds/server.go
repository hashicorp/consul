// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/xds/configfetcher"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	proxysnapshot "github.com/hashicorp/consul/internal/mesh/proxy-snapshot"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	StatsGauges = []prometheus.GaugeDefinition{
		{
			Name: []string{"xds", "server", "streams"},
			Help: "Measures the number of active xDS streams handled by the server split by protocol version.",
		},
		{
			Name: []string{"xds", "server", "streamsUnauthenticated"},
			Help: "Counts the number of active xDS streams handled by the server that are unauthenticated because ACLs are not enabled or ACL tokens were missing.",
		},
	}
	StatsCounters = []prometheus.CounterDefinition{
		{
			Name: []string{"xds", "server", "streamDrained"},
			Help: "Counts the number of xDS streams that are drained when rebalancing the load between servers.",
		},
	}
	StatsSummaries = []prometheus.SummaryDefinition{
		{
			Name: []string{"xds", "server", "streamStart"},
			Help: "Measures the time in milliseconds after an xDS stream is opened until xDS resources are first generated for the stream.",
		},
	}
)

// ADSStream is a shorter way of referring to this thing...
type ADSStream = envoy_discovery_v3.AggregatedDiscoveryService_StreamAggregatedResourcesServer

const (
	// LocalAgentClusterName is the name we give the local agent "cluster" in
	// Envoy config. Note that all cluster names may collide with service names
	// since we want cluster names and service names to match to enable nice
	// metrics correlation without massaging prefixes on cluster names.
	//
	// We should probably make this more unlikely to collied however changing it
	// potentially breaks upgrade compatibility without restarting all Envoy's as
	// it will no longer match their existing cluster name. Changing this will
	// affect metrics output so could break dashboards (for local agent traffic).
	//
	// We should probably just make it configurable if anyone actually has
	// services named "local_agent" in the future.
	LocalAgentClusterName = "local_agent"

	// DefaultAuthCheckFrequency is the default value for
	// Server.AuthCheckFrequency to use when the zero value is provided.
	DefaultAuthCheckFrequency = 5 * time.Minute
)

// ACLResolverFunc is a shim to resolve ACLs. Since ACL enforcement is so far
// entirely agent-local and all uses private methods this allows a simple shim
// to be written in the agent package to allow resolving without tightly
// coupling this to the agent.
type ACLResolverFunc func(id string) (acl.Authorizer, error)

// ProxyConfigSource is the interface xds.Server requires to consume proxy
// config updates.
type ProxyWatcher interface {
	Watch(proxyID *pbresource.ID, nodeName string, token string) (<-chan proxysnapshot.ProxySnapshot, limiter.SessionTerminatedChan, proxycfg.SrcTerminatedChan, proxysnapshot.CancelFunc, error)
}

// Server represents a gRPC server that can handle xDS requests from Envoy. All
// of it's public members must be set before the gRPC server is started.
//
// A full description of the XDS protocol can be found at
// https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
type Server struct {
	NodeName     string
	Logger       hclog.Logger
	ProxyWatcher ProxyWatcher
	ResolveToken ACLResolverFunc
	CfgFetcher   configfetcher.ConfigFetcher

	// AuthCheckFrequency is how often we should re-check the credentials used
	// during a long-lived gRPC Stream after it has been initially established.
	// This is only used during idle periods of stream interactions (i.e. when
	// there has been no recent DiscoveryRequest).
	AuthCheckFrequency time.Duration

	// ResourceMapMutateFn exclusively exists for testing purposes.
	ResourceMapMutateFn func(resourceMap *xdscommon.IndexedResources)

	activeStreams *activeStreamCounters
}

// activeStreamCounters tracks various stream-related metrics.
// Requires that activeStreamCounters be a pointer field.
type activeStreamCounters struct {
	xDSv3           atomic.Uint64
	unauthenticated atomic.Uint64
}

func (c *activeStreamCounters) Increment(ctx context.Context) func() {
	// If no ACL token is found, increase the gauge.
	o, _ := external.QueryOptionsFromContext(ctx)
	if o.Token == "" {
		unauthn := c.unauthenticated.Add(1)
		metrics.SetGauge([]string{"xds", "server", "streamsUnauthenticated"}, float32(unauthn))
	}

	// Historically there had been a "v2" version.
	labels := []metrics.Label{{Name: "version", Value: "v3"}}
	count := c.xDSv3.Add(1)
	metrics.SetGaugeWithLabels([]string{"xds", "server", "streams"}, float32(count), labels)

	// This closure should be called in a defer to decrement the gauges after the stream is closed.
	return func() {
		if o.Token == "" {
			unauthn := c.unauthenticated.Add(^uint64(0))
			metrics.SetGauge([]string{"xds", "server", "streamsUnauthenticated"}, float32(unauthn))
		}

		count := c.xDSv3.Add(^uint64(0))
		metrics.SetGaugeWithLabels([]string{"xds", "server", "streams"}, float32(count), labels)
	}
}

func NewServer(
	nodeName string,
	logger hclog.Logger,
	proxyWatcher ProxyWatcher,
	resolveTokenSecret ACLResolverFunc,
	cfgFetcher configfetcher.ConfigFetcher,
) *Server {
	return &Server{
		NodeName:           nodeName,
		Logger:             logger,
		ProxyWatcher:       proxyWatcher,
		ResolveToken:       resolveTokenSecret,
		CfgFetcher:         cfgFetcher,
		AuthCheckFrequency: DefaultAuthCheckFrequency,
		activeStreams:      &activeStreamCounters{},
	}
}

// StreamAggregatedResources implements
// envoy_discovery_v3.AggregatedDiscoveryServiceServer. This is the ADS endpoint which is
// the only xDS API we directly support for now.
//
// Deprecated: use DeltaAggregatedResources instead
func (s *Server) StreamAggregatedResources(stream ADSStream) error {
	return errors.New("not implemented")
}

// Register the XDS server handlers to the given gRPC server.
func (s *Server) Register(srv *grpc.Server) {
	envoy_discovery_v3.RegisterAggregatedDiscoveryServiceServer(srv, s)
}

func (s *Server) authenticate(ctx context.Context) (acl.Authorizer, error) {
	options, err := external.QueryOptionsFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error fetching options from context: %v", err)
	}

	authz, err := s.ResolveToken(options.Token)
	if acl.IsErrNotFound(err) {
		return nil, status.Errorf(codes.Unauthenticated, "unauthenticated: %v", err)
	} else if acl.IsErrPermissionDenied(err) {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "error resolving acl token: %v", err)
	}
	return authz, nil
}

// authorize the xDS request using the token stored in ctx. This authorization is
// a bit different from most interfaces. Instead of explicitly authorizing or
// filtering each piece of data in the response, the request is authorized
// by checking the token has `service:write` for the service ID of the destination
// service (for kind=ConnectProxy), or the gateway service (for other kinds).
// This authorization strategy requires that agent/proxycfg only fetches data
// using a token with the same permissions, and that it stores the data by
// proxy ID. We assume that any data in the snapshot was already filtered,
// which allows this authorization to be a shallow authorization check
// for all the data in a ProxySnapshot.
func (s *Server) authorize(ctx context.Context, proxySnapshot proxysnapshot.ProxySnapshot) error {
	if proxySnapshot == nil {
		return status.Errorf(codes.Unauthenticated, "unauthenticated: no config snapshot")
	}

	authz, err := s.authenticate(ctx)
	if err != nil {
		return err
	}

	return proxySnapshot.Authorize(authz)
}

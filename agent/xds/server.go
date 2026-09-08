// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/go-metrics/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/configfetcher"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
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
		{
			Name: []string{"xds", "server", "bootstrapGateTimeout"},
			Help: "Counts the number of times the api-gateway bootstrap completeness gate released an xDS stream's first push on its deadline before all discovery-chain endpoints were assembled.",
		},
		{
			Name: []string{"xds", "server", "apiGatewayFailoverDegraded"},
			Help: "Counts the number of times an api-gateway failover upstream was rendered as a single plain EDS cluster instead of an aggregate because a member's endpoints were not yet assembled. This averts an Envoy worker-startup crash and self-heals once the endpoints arrive.",
		},
	}
	StatsSummaries = []prometheus.SummaryDefinition{
		{
			Name: []string{"xds", "server", "streamStart"},
			Help: "Measures the time in milliseconds after an xDS stream is opened until xDS resources are first generated for the stream.",
		},
		{
			Name: []string{"xds", "server", "bootstrapGateHeld"},
			Help: "Measures the time in milliseconds an api-gateway xDS stream's first push was held by the bootstrap completeness gate before the snapshot's discovery-chain endpoints were assembled.",
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

	// DefaultBootstrapGateTimeout is the default value for
	// Server.BootstrapGateTimeout. It bounds how long an api-gateway xDS stream
	// will hold its first push waiting for the snapshot's discovery-chain
	// endpoints to be assembled (ITCO-15826).
	//
	// The gate is only ever expected to be satisfiable, but it must not be able
	// to wedge a stream if that assumption is ever violated: Consul programs
	// lds_config/cds_config with initial_fetch_timeout: 0s (wait forever), so a
	// permanently-held first push means a gateway that never becomes ready at
	// all. Past this deadline we push what we have — a gateway answering 503s is
	// strictly better than one that never listens.
	DefaultBootstrapGateTimeout = 30 * time.Second
)

// TODO(CSL-11921): remove once the guard is proven in the field
const (
	// EnvBootstrapGateTimeout overrides Server.BootstrapGateTimeout. It accepts
	// any time.ParseDuration value; a negative duration disables the api-gateway
	// cold-start gate entirely.
	//
	// This is a break-glass control, deliberately an environment variable rather
	// than an agent config option: it exists so a misbehaving gate can be
	// neutralized on a running cluster without a binary rollback, not as a knob
	// operators are expected to tune. The default is correct for all known
	// topologies.
	EnvBootstrapGateTimeout = "CONSUL_XDS_APIGATEWAY_BOOTSTRAP_GATE_TIMEOUT"

	// EnvDisableAPIGatewayFailoverGuard disables the api-gateway
	// aggregate-cluster failover guard when set to a value strconv.ParseBool
	// reads as true. See Server.DisableAPIGatewayFailoverGuard, and the
	// break-glass note on EnvBootstrapGateTimeout.
	EnvDisableAPIGatewayFailoverGuard = "CONSUL_XDS_DISABLE_APIGATEWAY_FAILOVER_GUARD"
)

// bootstrapGateTimeoutFromEnv resolves Server.BootstrapGateTimeout from
// EnvBootstrapGateTimeout, falling back to DefaultBootstrapGateTimeout.
//
// An unset or malformed value yields the default rather than the zero value:
// callers read zero as "use the default" anyway, but returning the default
// explicitly keeps a typo from being indistinguishable from an intentional
// override in a log.
func bootstrapGateTimeoutFromEnv(logger hclog.Logger) time.Duration {
	raw, ok := os.LookupEnv(EnvBootstrapGateTimeout)
	if !ok || raw == "" {
		return DefaultBootstrapGateTimeout
	}

	timeout, err := time.ParseDuration(raw)
	if err != nil {
		logger.Warn("ignoring malformed environment variable; using the default api-gateway bootstrap gate timeout",
			"env", EnvBootstrapGateTimeout, "value", raw, "default", DefaultBootstrapGateTimeout, "error", err)
		return DefaultBootstrapGateTimeout
	}

	if timeout < 0 {
		logger.Warn("api-gateway xDS bootstrap gate is DISABLED by environment variable; "+
			"a cold-starting api-gateway Envoy may receive clusters whose endpoints are not assembled yet",
			"env", EnvBootstrapGateTimeout, "value", raw)
	} else {
		logger.Info("api-gateway xDS bootstrap gate timeout overridden by environment variable",
			"env", EnvBootstrapGateTimeout, "timeout", timeout)
	}
	return timeout
}

// disableAPIGatewayFailoverGuardFromEnv resolves
// Server.DisableAPIGatewayFailoverGuard from
// EnvDisableAPIGatewayFailoverGuard.
//
// Anything other than an explicit, parseable true leaves the guard enabled. The
// guard averts an Envoy crash, so a malformed value must fail closed: silently
// disabling it on a typo would reintroduce the fault it exists to prevent.
func disableAPIGatewayFailoverGuardFromEnv(logger hclog.Logger) bool {
	raw, ok := os.LookupEnv(EnvDisableAPIGatewayFailoverGuard)
	if !ok || raw == "" {
		return false
	}

	disabled, err := strconv.ParseBool(raw)
	if err != nil {
		logger.Warn("ignoring malformed environment variable; the api-gateway failover guard remains enabled",
			"env", EnvDisableAPIGatewayFailoverGuard, "value", raw, "error", err)
		return false
	}

	if disabled {
		logger.Warn("api-gateway aggregate-cluster failover guard is DISABLED by environment variable; "+
			"an api-gateway whose failover member endpoints are not assembled may crash its Envoy "+
			"(envoyproxy/envoy#35157)",
			"env", EnvDisableAPIGatewayFailoverGuard)
	}
	return disabled
}

// ACLResolverFunc is a shim to resolve ACLs. Since ACL enforcement is so far
// entirely agent-local and all uses private methods this allows a simple shim
// to be written in the agent package to allow resolving without tightly
// coupling this to the agent.
type ACLResolverFunc func(id string) (acl.Authorizer, error)

// ProxyConfigSource is the interface xds.Server requires to consume proxy
// config updates.
type ProxyWatcher interface {
	Watch(proxyID structs.ServiceID, nodeName string, token string) (<-chan *proxycfg.ConfigSnapshot, limiter.SessionTerminatedChan, proxycfg.SrcTerminatedChan, context.CancelFunc, error)
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

	// BootstrapGateTimeout bounds the api-gateway cold-start completeness gate
	// applied to a stream's first push. Zero means DefaultBootstrapGateTimeout;
	// a negative value disables the gate entirely. NewServer populates it from
	// EnvBootstrapGateTimeout.
	BootstrapGateTimeout time.Duration

	// DisableAPIGatewayFailoverGuard disables the aggregate-cluster guard that
	// rewrites an api-gateway failover chain into a single plain EDS cluster
	// when a failover member's endpoints are not assembled yet
	// (agent/xds/failover_policy.go, ITCO-15826). The guard is the safety net
	// for an aggregate cluster whose members have no EDS assignment, which
	// faults Envoy during worker startup rather than merely degrading
	// (envoyproxy/envoy#35157).
	//
	// This exists purely as an escape hatch in case the guard misbehaves in a
	// topology we have not exercised, so that recovery does not require a
	// binary rollback. NewServer populates it from
	// EnvDisableAPIGatewayFailoverGuard; it is an environment variable rather
	// than an agent config option because the zero value (false) is correct for
	// all known topologies, and disabling it re-exposes the crash the guard
	// exists to prevent.
	DisableAPIGatewayFailoverGuard bool

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

		BootstrapGateTimeout:           bootstrapGateTimeoutFromEnv(logger),
		DisableAPIGatewayFailoverGuard: disableAPIGatewayFailoverGuardFromEnv(logger),
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
// for all the data in a ConfigSnapshot.
func (s *Server) authorize(ctx context.Context, snapshot *proxycfg.ConfigSnapshot) error {
	if snapshot == nil {
		return status.Errorf(codes.Unauthenticated, "unauthenticated: no config snapshot")
	}

	authz, err := s.authenticate(ctx)
	if err != nil {
		return err
	}

	return snapshot.Authorize(authz)
}

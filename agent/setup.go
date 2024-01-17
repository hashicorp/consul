package agent

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/grpclog"

	autoconf "github.com/hashicorp/consul/agent/auto-config"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/consul/usagemetrics"
	"github.com/hashicorp/consul/agent/consul/xdscapacity"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	grpcInt "github.com/hashicorp/consul/agent/grpc-internal"
	"github.com/hashicorp/consul/agent/grpc-internal/resolver"
	grpcWare "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/agent/submatview"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/tlsutil"
)

// TODO: BaseDeps should be renamed in the future once more of Agent.Start
// has been moved out in front of Agent.New, and we can better see the setup
// dependencies.
type BaseDeps struct {
	consul.Deps // TODO: un-embed

	RuntimeConfig *config.RuntimeConfig
	MetricsConfig *lib.MetricsConfig
	AutoConfig    *autoconf.AutoConfig // TODO: use an interface
	Cache         *cache.Cache
	ViewStore     *submatview.Store
	WatchedFiles  []string
}

type ConfigLoader func(source config.Source) (config.LoadResult, error)

func NewBaseDeps(configLoader ConfigLoader, logOut io.Writer) (BaseDeps, error) {
	d := BaseDeps{}
	result, err := configLoader(nil)
	if err != nil {
		return d, err
	}
	d.WatchedFiles = result.WatchedFiles
	cfg := result.RuntimeConfig
	logConf := cfg.Logging
	logConf.Name = logging.Agent
	d.Logger, err = logging.Setup(logConf, logOut)
	if err != nil {
		return d, err
	}

	grpcLogInitOnce.Do(func() {
		grpclog.SetLoggerV2(logging.NewGRPCLogger(cfg.Logging.LogLevel, d.Logger))
	})

	for _, w := range result.Warnings {
		d.Logger.Warn(w)
	}

	cfg.NodeID, err = newNodeIDFromConfig(cfg, d.Logger)
	if err != nil {
		return d, fmt.Errorf("failed to setup node ID: %w", err)
	}

	isServer := result.RuntimeConfig.ServerMode
	gauges, counters, summaries := getPrometheusDefs(cfg.Telemetry, isServer)
	cfg.Telemetry.PrometheusOpts.GaugeDefinitions = gauges
	cfg.Telemetry.PrometheusOpts.CounterDefinitions = counters
	cfg.Telemetry.PrometheusOpts.SummaryDefinitions = summaries

	d.MetricsConfig, err = lib.InitTelemetry(cfg.Telemetry, d.Logger)
	if err != nil {
		return d, fmt.Errorf("failed to initialize telemetry: %w", err)
	}

	d.TLSConfigurator, err = tlsutil.NewConfigurator(cfg.TLS, d.Logger)
	if err != nil {
		return d, err
	}

	d.RuntimeConfig = cfg
	d.Tokens = new(token.Store)

	cfg.Cache.Logger = d.Logger.Named("cache")
	// cache-types are not registered yet, but they won't be used until the components are started.
	d.Cache = cache.New(cfg.Cache)
	d.ViewStore = submatview.NewStore(d.Logger.Named("viewstore"))
	d.ConnPool = newConnPool(cfg, d.Logger, d.TLSConfigurator)

	agentType := "client"
	if cfg.ServerMode {
		agentType = "server"
	}

	builder := resolver.NewServerResolverBuilder(resolver.Config{
		AgentType:  agentType,
		Datacenter: cfg.Datacenter,
		// Set the authority to something sufficiently unique so any usage in
		// tests would be self-isolating in the global resolver map, while also
		// not incurring a huge penalty for non-test code.
		Authority: cfg.Datacenter + "." + string(cfg.NodeID),
	})
	resolver.Register(builder)
	d.GRPCConnPool = grpcInt.NewClientConnPool(grpcInt.ClientConnPoolConfig{
		Servers:               builder,
		SrcAddr:               d.ConnPool.SrcAddr,
		TLSWrapper:            grpcInt.TLSWrapper(d.TLSConfigurator.OutgoingRPCWrapper()),
		ALPNWrapper:           grpcInt.ALPNWrapper(d.TLSConfigurator.OutgoingALPNRPCWrapper()),
		UseTLSForDC:           d.TLSConfigurator.UseTLS,
		DialingFromServer:     cfg.ServerMode,
		DialingFromDatacenter: cfg.Datacenter,
	})
	d.LeaderForwarder = builder

	d.Router = router.NewRouter(d.Logger, cfg.Datacenter, fmt.Sprintf("%s.%s", cfg.NodeName, cfg.Datacenter), builder)

	// this needs to happen prior to creating auto-config as some of the dependencies
	// must also be passed to auto-config
	d, err = initEnterpriseBaseDeps(d, cfg)
	if err != nil {
		return d, err
	}

	acConf := autoconf.Config{
		DirectRPC:        d.ConnPool,
		Logger:           d.Logger,
		Loader:           configLoader,
		ServerProvider:   d.Router,
		TLSConfigurator:  d.TLSConfigurator,
		Cache:            d.Cache,
		Tokens:           d.Tokens,
		EnterpriseConfig: initEnterpriseAutoConfig(d.EnterpriseDeps, cfg),
	}

	d.AutoConfig, err = autoconf.New(acConf)
	if err != nil {
		return d, err
	}

	d.NewRequestRecorderFunc = middleware.NewRequestRecorder
	d.GetNetRPCInterceptorFunc = middleware.GetNetRPCInterceptor

	d.EventPublisher = stream.NewEventPublisher(10 * time.Second)

	d.XDSStreamLimiter = limiter.NewSessionLimiter()
	if cfg.IsCloudEnabled() {
		d.HCP, err = hcp.NewDeps(cfg.Cloud, d.Logger)
		if err != nil {
			return d, err
		}
	}

	return d, nil
}

// grpcLogInitOnce because the test suite will call NewBaseDeps in many tests and
// causes data races when it is re-initialized.
var grpcLogInitOnce sync.Once

func newConnPool(config *config.RuntimeConfig, logger hclog.Logger, tls *tlsutil.Configurator) *pool.ConnPool {
	var rpcSrcAddr *net.TCPAddr
	if !ipaddr.IsAny(config.RPCBindAddr) {
		rpcSrcAddr = &net.TCPAddr{IP: config.RPCBindAddr.IP}
	}

	pool := &pool.ConnPool{
		Server:           config.ServerMode,
		SrcAddr:          rpcSrcAddr,
		Logger:           logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}),
		TLSConfigurator:  tls,
		Datacenter:       config.Datacenter,
		RPCHoldTimeout:   config.RPCHoldTimeout,
		MaxQueryTime:     config.MaxQueryTime,
		DefaultQueryTime: config.DefaultQueryTime,
	}
	pool.SetRPCClientTimeout(config.RPCClientTimeout)
	if config.ServerMode {
		pool.MaxTime = 2 * time.Minute
		pool.MaxStreams = 64
	} else {
		// MaxTime controls how long we keep an idle connection open to a server.
		// 127s was chosen as the first prime above 120s
		// (arbitrarily chose to use a prime) with the intent of reusing
		// connections who are used by once-a-minute cron(8) jobs *and* who
		// use a 60s jitter window (e.g. in vixie cron job execution can
		// drift by up to 59s per job, or 119s for a once-a-minute cron job).
		pool.MaxTime = 127 * time.Second
		pool.MaxStreams = 32
	}
	return pool
}

// getPrometheusDefs reaches into every slice of prometheus defs we've defined in each part of the agent, and appends
// all of our slices into one nice slice of definitions per metric type for the Consul agent to pass to go-metrics.
func getPrometheusDefs(cfg lib.TelemetryConfig, isServer bool) ([]prometheus.GaugeDefinition, []prometheus.CounterDefinition, []prometheus.SummaryDefinition) {
	// TODO: "raft..." metrics come from the raft lib and we should migrate these to a telemetry
	//  package within. In the mean time, we're going to define a few here because they're key to monitoring Consul.
	raftGauges := []prometheus.GaugeDefinition{
		{
			Name: []string{"raft", "fsm", "lastRestoreDuration"},
			Help: "This measures how long the last FSM restore (from disk or leader) took.",
		},
		{
			Name: []string{"raft", "leader", "oldestLogAge"},
			Help: "This measures how old the oldest log in the leader's log store is.",
		},
	}

	serverGauges := []prometheus.GaugeDefinition{
		{
			Name: []string{"server", "isLeader"},
			Help: "Tracks if the server is a leader.",
		},
	}

	// Build slice of slices for all gauge definitions
	var gauges = [][]prometheus.GaugeDefinition{
		cache.Gauges,
		consul.RPCGauges,
		consul.SessionGauges,
		grpcWare.StatsGauges,
		xds.StatsGauges,
		usagemetrics.Gauges,
		consul.ReplicationGauges,
		CertExpirationGauges,
		Gauges,
		raftGauges,
		serverGauges,
	}

	// TODO(ffmmm): conditionally add only leader specific metrics to gauges, counters, summaries, etc
	if isServer {
		gauges = append(gauges,
			consul.AutopilotGauges,
			consul.LeaderCertExpirationGauges,
			consul.LeaderPeeringMetrics,
			xdscapacity.StatsGauges,
		)
	}

	// Flatten definitions
	// NOTE(kit): Do we actually want to create a set here so we can ensure definition names are unique?
	var gaugeDefs []prometheus.GaugeDefinition
	for _, g := range gauges {
		// Set Consul to each definition's namespace
		// TODO(kit): Prepending the service to each definition should be handled by go-metrics
		var withService []prometheus.GaugeDefinition
		for _, gauge := range g {
			gauge.Name = append([]string{cfg.MetricsPrefix}, gauge.Name...)
			withService = append(withService, gauge)
		}
		gaugeDefs = append(gaugeDefs, withService...)
	}

	raftCounters := []prometheus.CounterDefinition{
		// TODO(kit): "raft..." metrics come from the raft lib and we should migrate these to a telemetry
		//  package within. In the mean time, we're going to define a few here because they're key to monitoring Consul.
		{
			Name: []string{"raft", "apply"},
			Help: "This counts the number of Raft transactions occurring over the interval.",
		},
		{
			Name: []string{"raft", "state", "candidate"},
			Help: "This increments whenever a Consul server starts an election.",
		},
		{
			Name: []string{"raft", "state", "leader"},
			Help: "This increments whenever a Consul server becomes a leader.",
		},
	}

	var counters = [][]prometheus.CounterDefinition{
		CatalogCounters,
		cache.Counters,
		consul.ACLCounters,
		consul.CatalogCounters,
		consul.ClientCounters,
		consul.RPCCounters,
		grpcWare.StatsCounters,
		local.StateCounters,
		xds.StatsCounters,
		raftCounters,
	}
	// Flatten definitions
	// NOTE(kit): Do we actually want to create a set here so we can ensure definition names are unique?
	var counterDefs []prometheus.CounterDefinition
	for _, c := range counters {
		// TODO(kit): Prepending the service to each definition should be handled by go-metrics
		var withService []prometheus.CounterDefinition
		for _, counter := range c {
			counter.Name = append([]string{cfg.MetricsPrefix}, counter.Name...)
			withService = append(withService, counter)
		}
		counterDefs = append(counterDefs, withService...)
	}

	raftSummaries := []prometheus.SummaryDefinition{
		// TODO(kit): "raft..." metrics come from the raft lib and we should migrate these to a telemetry
		//  package within. In the mean time, we're going to define a few here because they're key to monitoring Consul.
		{
			Name: []string{"raft", "commitTime"},
			Help: "This measures the time it takes to commit a new entry to the Raft log on the leader.",
		},
		{
			Name: []string{"raft", "leader", "lastContact"},
			Help: "Measures the time since the leader was last able to contact the follower nodes when checking its leader lease.",
		},
		{
			Name: []string{"raft", "snapshot", "persist"},
			Help: "Measures the time it takes raft to write a new snapshot to disk.",
		},
		{
			Name: []string{"raft", "rpc", "installSnapshot"},
			Help: "Measures the time it takes the raft leader to install a snapshot on a follower that is catching up after being down or has just joined the cluster.",
		},
	}

	var summaries = [][]prometheus.SummaryDefinition{
		HTTPSummaries,
		consul.ACLSummaries,
		consul.ACLEndpointSummaries,
		consul.CatalogSummaries,
		consul.FederationStateSummaries,
		consul.IntentionSummaries,
		consul.KVSummaries,
		consul.LeaderSummaries,
		consul.PreparedQuerySummaries,
		consul.RPCSummaries,
		consul.SegmentCESummaries,
		consul.SessionSummaries,
		consul.SessionEndpointSummaries,
		consul.TxnSummaries,
		fsm.CommandsSummaries,
		fsm.SnapshotSummaries,
		raftSummaries,
		xds.StatsSummaries,
	}
	// Flatten definitions
	// NOTE(kit): Do we actually want to create a set here so we can ensure definition names are unique?
	var summaryDefs []prometheus.SummaryDefinition
	for _, s := range summaries {
		// TODO(kit): Prepending the service to each definition should be handled by go-metrics
		var withService []prometheus.SummaryDefinition
		for _, summary := range s {
			summary.Name = append([]string{cfg.MetricsPrefix}, summary.Name...)
			withService = append(withService, summary)
		}
		summaryDefs = append(summaryDefs, withService...)
	}

	return gaugeDefs, counterDefs, summaryDefs
}

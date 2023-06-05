package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcp-scada-provider/capability"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/ae"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/servercert"
	"github.com/hashicorp/consul/agent/dns"
	external "github.com/hashicorp/consul/agent/grpc-external"
	grpcDNS "github.com/hashicorp/consul/agent/grpc-external/services/dns"
	middleware "github.com/hashicorp/consul/agent/grpc-middleware"
	"github.com/hashicorp/consul/agent/hcp/scada"
	libscada "github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	proxycfgglue "github.com/hashicorp/consul/agent/proxycfg-glue"
	catalogproxycfg "github.com/hashicorp/consul/agent/proxycfg-sources/catalog"
	localproxycfg "github.com/hashicorp/consul/agent/proxycfg-sources/local"
	"github.com/hashicorp/consul/agent/rpcclient/health"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/systemd"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/agent/xds"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/file"
	"github.com/hashicorp/consul/lib/mutex"
	"github.com/hashicorp/consul/lib/routine"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
)

const (
	// Path to save agent service definitions
	servicesDir      = "services"
	serviceConfigDir = "services/configs"

	// Path to save agent proxy definitions
	proxyDir = "proxies"

	// Path to save local agent checks
	checksDir     = "checks"
	checkStateDir = "checks/state"

	// Default reasons for node/service maintenance mode
	defaultNodeMaintReason = "Maintenance mode is enabled for this node, " +
		"but no reason was provided. This is a default message."
	defaultServiceMaintReason = "Maintenance mode is enabled for this " +
		"service, but no reason was provided. This is a default message."

	// ID of the roots watch
	rootsWatchID = "roots"

	// ID of the leaf watch
	leafWatchID = "leaf"

	// maxQueryTime is used to bound the limit of a blocking query
	maxQueryTime = 600 * time.Second

	// defaultQueryTime is the amount of time we block waiting for a change
	// if no time is specified. Previously we would wait the maxQueryTime.
	defaultQueryTime = 300 * time.Second
)

var (
	httpAddrRE = regexp.MustCompile(`^(http[s]?://)(\[.*?\]|\[?[\w\-\.]+)(:\d+)?([^?]*)(\?.*)?$`)
	grpcAddrRE = regexp.MustCompile("(.*)((?::)(?:[0-9]+))(.*)$")
)

type configSource int

const (
	ConfigSourceLocal configSource = iota
	ConfigSourceRemote
)

var configSourceToName = map[configSource]string{
	ConfigSourceLocal:  "local",
	ConfigSourceRemote: "remote",
}
var configSourceFromName = map[string]configSource{
	"local":  ConfigSourceLocal,
	"remote": ConfigSourceRemote,
	// If the value is not found in the persisted config file, then use the
	// former default.
	"": ConfigSourceLocal,
}

func (s configSource) String() string {
	return configSourceToName[s]
}

// ConfigSourceFromName will unmarshal the string form of a configSource.
func ConfigSourceFromName(name string) (configSource, bool) {
	s, ok := configSourceFromName[name]
	return s, ok
}

// delegate defines the interface shared by both
// consul.Client and consul.Server.
type delegate interface {
	// Leave is used to prepare for a graceful shutdown.
	Leave() error

	// AgentLocalMember is used to retrieve the LAN member for the local node.
	AgentLocalMember() serf.Member

	// LANMembersInAgentPartition returns the LAN members for this agent's
	// canonical serf pool. For clients this is the only pool that exists. For
	// servers it's the pool in the default segment and the default partition.
	LANMembersInAgentPartition() []serf.Member

	// LANMembers returns the LAN members for one of:
	//
	// - the requested partition
	// - the requested segment
	// - all segments
	//
	// This is limited to segments and partitions that the node is a member of.
	LANMembers(f consul.LANMemberFilter) ([]serf.Member, error)

	// GetLANCoordinate returns the coordinate of the node in the LAN gossip
	// pool.
	//
	// - Clients return a single coordinate for the single gossip pool they are
	//   in (default, segment, or partition).
	//
	// - Servers return one coordinate for their canonical gossip pool (i.e.
	//   default partition/segment) and one per segment they are also ancillary
	//   members of.
	//
	// NOTE: servers do not emit coordinates for partitioned gossip pools they
	// are ancillary members of.
	//
	// NOTE: This assumes coordinates are enabled, so check that before calling.
	GetLANCoordinate() (lib.CoordinateSet, error)

	// JoinLAN is used to have Consul join the inner-DC pool The target address
	// should be another node inside the DC listening on the Serf LAN address
	JoinLAN(addrs []string, entMeta *acl.EnterpriseMeta) (n int, err error)

	// RemoveFailedNode is used to remove a failed node from the cluster.
	RemoveFailedNode(node string, prune bool, entMeta *acl.EnterpriseMeta) error

	// ResolveTokenAndDefaultMeta returns an acl.Authorizer which authorizes
	// actions based on the permissions granted to the token.
	// If either entMeta or authzContext are non-nil they will be populated with the
	// default partition and namespace from the token.
	ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzContext *acl.AuthorizerContext) (resolver.Result, error)

	RPC(method string, args interface{}, reply interface{}) error
	SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer, replyFn structs.SnapshotReplyFn) error
	Shutdown() error
	Stats() map[string]map[string]string
	ReloadConfig(config consul.ReloadableConfig) error
	enterpriseDelegate
}

// notifier is called after a successful JoinLAN.
type notifier interface {
	Notify(string) error
}

// Agent is the long running process that is run on every machine.
// It exposes an RPC interface that is used by the CLI to control the
// agent. The agent runs the query interfaces like HTTP, DNS, and RPC.
// However, it can run in either a client, or server mode. In server
// mode, it runs a full Consul server. In client-only mode, it only forwards
// requests to other Consul servers.
type Agent struct {
	// TODO: remove fields that are already in BaseDeps
	baseDeps BaseDeps

	// config is the agent configuration.
	config *config.RuntimeConfig

	// Used for writing our logs
	logger hclog.InterceptLogger

	// delegate is either a *consul.Server or *consul.Client
	// depending on the configuration
	delegate delegate

	// externalGRPCServer is the gRPC server exposed on dedicated gRPC ports (as
	// opposed to the multiplexed "server" port).
	externalGRPCServer *grpc.Server

	// state stores a local representation of the node,
	// services and checks. Used for anti-entropy.
	State *local.State

	// sync manages the synchronization of the local
	// and the remote state.
	sync *ae.StateSyncer

	// syncMu and syncCh are used to coordinate agent endpoints that are blocking
	// on local state during a config reload.
	syncMu sync.Mutex
	syncCh chan struct{}

	// cache is the in-memory cache for data the Agent requests.
	cache *cache.Cache

	// checkReapAfter maps the check ID to a timeout after which we should
	// reap its associated service
	checkReapAfter map[structs.CheckID]time.Duration

	// checkMonitors maps the check ID to an associated monitor
	checkMonitors map[structs.CheckID]*checks.CheckMonitor

	// checkHTTPs maps the check ID to an associated HTTP check
	checkHTTPs map[structs.CheckID]*checks.CheckHTTP

	// checkH2PINGs maps the check ID to an associated HTTP2 PING check
	checkH2PINGs map[structs.CheckID]*checks.CheckH2PING

	// checkTCPs maps the check ID to an associated TCP check
	checkTCPs map[structs.CheckID]*checks.CheckTCP

	// checkUDPs maps the check ID to an associated UDP check
	checkUDPs map[structs.CheckID]*checks.CheckUDP

	// checkGRPCs maps the check ID to an associated GRPC check
	checkGRPCs map[structs.CheckID]*checks.CheckGRPC

	// checkTTLs maps the check ID to an associated check TTL
	checkTTLs map[structs.CheckID]*checks.CheckTTL

	// checkDockers maps the check ID to an associated Docker Exec based check
	checkDockers map[structs.CheckID]*checks.CheckDocker

	// checkAliases maps the check ID to an associated Alias checks
	checkAliases map[structs.CheckID]*checks.CheckAlias

	// checkOSServices maps the check ID to an associated OS Service check
	checkOSServices map[structs.CheckID]*checks.CheckOSService

	// exposedPorts tracks listener ports for checks exposed through a proxy
	exposedPorts map[string]int

	// stateLock protects the agent state
	stateLock *mutex.Mutex

	// dockerClient is the client for performing docker health checks.
	dockerClient *checks.DockerClient

	// osServiceClient is the client for performing OS service checks.
	osServiceClient *checks.OSServiceClient

	// eventCh is used to receive user events
	eventCh chan serf.UserEvent

	// eventBuf stores the most recent events in a ring buffer
	// using eventIndex as the next index to insert into. This
	// is guarded by eventLock. When an insert happens, the
	// eventNotify group is notified.
	eventBuf    []*UserEvent
	eventIndex  int
	eventLock   sync.RWMutex
	eventNotify NotifyGroup

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	// joinLANNotifier is called after a successful JoinLAN.
	joinLANNotifier notifier

	// retryJoinCh transports errors from the retry join
	// attempts.
	retryJoinCh chan error

	// endpoints maps unique RPC endpoint names to common ones
	// to allow overriding of RPC handlers since the golang
	// net/rpc server does not allow this.
	endpoints     map[string]string
	endpointsLock sync.RWMutex

	// dnsServer provides the DNS API
	dnsServers []*DNSServer

	// apiServers listening for connections. If any of these server goroutines
	// fail, the agent will be shutdown.
	apiServers *apiServers

	// httpHandlers provides direct access to (one of) the HTTPHandlers started by
	// this agent. This is used in tests to test HTTP endpoints without overhead
	// of TCP connections etc.
	//
	// TODO: this is a temporary re-introduction after we removed a list of
	// HTTPServers in favour of apiServers abstraction. Now that HTTPHandlers is
	// stateful and has config reloading though it's not OK to just use a
	// different instance of handlers in tests to the ones that the agent is wired
	// up to since then config reloads won't actually affect the handlers under
	// test while plumbing the external handlers in the TestAgent through bypasses
	// testing that the agent itself is actually reloading the state correctly.
	// Once we move `apiServers` to be a passed-in dependency for NewAgent, we
	// should be able to remove this and have the Test Agent create the
	// HTTPHandlers and pass them in removing the need to pull them back out
	// again.
	httpHandlers *HTTPHandlers

	// wgServers is the wait group for all HTTP and DNS servers
	// TODO: remove once dnsServers are handled by apiServers
	wgServers sync.WaitGroup

	// watchPlans tracks all the currently-running watch plans for the
	// agent.
	watchPlans []*watch.Plan

	// tokens holds ACL tokens initially from the configuration, but can
	// be updated at runtime, so should always be used instead of going to
	// the configuration directly.
	tokens *token.Store

	// proxyConfig is the manager for proxy service (Kind = connect-proxy)
	// configuration state. This ensures all state needed by a proxy registration
	// is maintained in cache and handles pushing updates to that state into XDS
	// server to be pushed out to Envoy.
	proxyConfig *proxycfg.Manager

	// serviceManager is the manager for combining local service registrations with
	// the centrally configured proxy/service defaults.
	serviceManager *ServiceManager

	// tlsConfigurator is the central instance to provide a *tls.Config
	// based on the current consul configuration.
	tlsConfigurator *tlsutil.Configurator

	// certManager manages the lifecycle of the internally-managed server certificate.
	certManager *servercert.CertManager

	// httpConnLimiter is used to limit connections to the HTTP server by client
	// IP.
	httpConnLimiter connlimit.Limiter

	// configReloaders are subcomponents that need to be notified on a reload so
	// they can update their internal state.
	configReloaders []ConfigReloader

	// TODO: pass directly to HTTPHandlers and DNSServer once those are passed
	// into Agent, which will allow us to remove this field.
	rpcClientHealth *health.Client

	rpcClientPeering pbpeering.PeeringServiceClient

	// routineManager is responsible for managing longer running go routines
	// run by the Agent
	routineManager *routine.Manager

	// configFileWatcher is the watcher responsible to report events when a config file
	// changed
	configFileWatcher config.Watcher

	// xdsServer serves the XDS protocol for configuring Envoy proxies.
	xdsServer *xds.Server

	// scadaProvider is set when HashiCorp Cloud Platform integration is configured and exposes the agent's API over
	// an encrypted session to HCP
	scadaProvider scada.Provider

	// enterpriseAgent embeds fields that we only access in consul-enterprise builds
	enterpriseAgent
}

// New process the desired options and creates a new Agent.
// This process will
//   - parse the config given the config Flags
//   - setup logging
//   - using predefined logger given in an option
//     OR
//   - initialize a new logger from the configuration
//     including setting up gRPC logging
//   - initialize telemetry
//   - create a TLS Configurator
//   - build a shared connection pool
//   - create the ServiceManager
//   - setup the NodeID if one isn't provided in the configuration
//   - create the AutoConfig object for future use in fully
//     resolving the configuration
func New(bd BaseDeps) (*Agent, error) {
	a := Agent{
		checkReapAfter:  make(map[structs.CheckID]time.Duration),
		checkMonitors:   make(map[structs.CheckID]*checks.CheckMonitor),
		checkTTLs:       make(map[structs.CheckID]*checks.CheckTTL),
		checkHTTPs:      make(map[structs.CheckID]*checks.CheckHTTP),
		checkH2PINGs:    make(map[structs.CheckID]*checks.CheckH2PING),
		checkTCPs:       make(map[structs.CheckID]*checks.CheckTCP),
		checkUDPs:       make(map[structs.CheckID]*checks.CheckUDP),
		checkGRPCs:      make(map[structs.CheckID]*checks.CheckGRPC),
		checkDockers:    make(map[structs.CheckID]*checks.CheckDocker),
		checkAliases:    make(map[structs.CheckID]*checks.CheckAlias),
		checkOSServices: make(map[structs.CheckID]*checks.CheckOSService),
		eventCh:         make(chan serf.UserEvent, 1024),
		eventBuf:        make([]*UserEvent, 256),
		joinLANNotifier: &systemd.Notifier{},
		retryJoinCh:     make(chan error),
		shutdownCh:      make(chan struct{}),
		endpoints:       make(map[string]string),
		stateLock:       mutex.New(),

		baseDeps:        bd,
		tokens:          bd.Tokens,
		logger:          bd.Logger,
		tlsConfigurator: bd.TLSConfigurator,
		config:          bd.RuntimeConfig,
		cache:           bd.Cache,
		routineManager:  routine.NewManager(bd.Logger),
		scadaProvider:   bd.HCP.Provider,
	}

	// TODO: create rpcClientHealth in BaseDeps once NetRPC is available without Agent
	conn, err := bd.GRPCConnPool.ClientConn(bd.RuntimeConfig.Datacenter)
	if err != nil {
		return nil, err
	}

	a.rpcClientHealth = &health.Client{
		Cache:     bd.Cache,
		NetRPC:    &a,
		CacheName: cachetype.HealthServicesName,
		ViewStore: bd.ViewStore,
		MaterializerDeps: health.MaterializerDeps{
			Conn:   conn,
			Logger: bd.Logger.Named("rpcclient.health"),
		},
		UseStreamingBackend: a.config.UseStreamingBackend,
		QueryOptionDefaults: config.ApplyDefaultQueryOptions(a.config),
	}

	a.rpcClientPeering = pbpeering.NewPeeringServiceClient(conn)

	a.serviceManager = NewServiceManager(&a)

	// We used to do this in the Start method. However it doesn't need to go
	// there any longer. Originally it did because we passed the agent
	// delegate to some of the cache registrations. Now we just
	// pass the agent itself so its safe to move here.
	a.registerCache()

	// TODO: why do we ignore failure to load persisted tokens?
	_ = a.tokens.Load(bd.RuntimeConfig.ACLTokens, a.logger)

	// TODO: pass in a fully populated apiServers into Agent.New
	a.apiServers = NewAPIServers(a.logger)

	for _, f := range []struct {
		Cfg tlsutil.ProtocolConfig
	}{
		{a.baseDeps.RuntimeConfig.TLS.InternalRPC},
		{a.baseDeps.RuntimeConfig.TLS.GRPC},
		{a.baseDeps.RuntimeConfig.TLS.HTTPS},
	} {
		if f.Cfg.KeyFile != "" {
			a.baseDeps.WatchedFiles = append(a.baseDeps.WatchedFiles, f.Cfg.KeyFile)
		}
		if f.Cfg.CertFile != "" {
			a.baseDeps.WatchedFiles = append(a.baseDeps.WatchedFiles, f.Cfg.CertFile)
		}
	}
	if a.baseDeps.RuntimeConfig.AutoReloadConfig && len(a.baseDeps.WatchedFiles) > 0 {
		w, err := config.NewRateLimitedFileWatcher(a.baseDeps.WatchedFiles, a.baseDeps.Logger, a.baseDeps.RuntimeConfig.AutoReloadConfigCoalesceInterval)
		if err != nil {
			return nil, err
		}
		a.configFileWatcher = w
	}

	return &a, nil
}

// GetConfig retrieves the agents config
// TODO make export the config field and get rid of this method
// This is here for now to simplify the work I am doing and make
// reviewing the final PR easier.
func (a *Agent) GetConfig() *config.RuntimeConfig {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.config
}

// LocalConfig takes a config.RuntimeConfig and maps the fields to a local.Config
func LocalConfig(cfg *config.RuntimeConfig) local.Config {
	lc := local.Config{
		AdvertiseAddr:       cfg.AdvertiseAddrLAN.String(),
		CheckUpdateInterval: cfg.CheckUpdateInterval,
		Datacenter:          cfg.Datacenter,
		DiscardCheckOutput:  cfg.DiscardCheckOutput,
		NodeID:              cfg.NodeID,
		NodeName:            cfg.NodeName,
		Partition:           cfg.PartitionOrDefault(),
		TaggedAddresses:     map[string]string{},
	}
	for k, v := range cfg.TaggedAddresses {
		lc.TaggedAddresses[k] = v
	}
	return lc
}

// Start verifies its configuration and runs an agent's various subprocesses.
func (a *Agent) Start(ctx context.Context) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	// This needs to be done early on as it will potentially alter the configuration
	// and then how other bits are brought up
	c, err := a.baseDeps.AutoConfig.InitialConfiguration(ctx)
	if err != nil {
		return err
	}

	// copy over the existing node id, this cannot be
	// changed while running anyways but this prevents
	// breaking some existing behavior. then overwrite
	// the configuration
	c.NodeID = a.config.NodeID
	a.config = c

	if err := a.tlsConfigurator.Update(a.config.TLS); err != nil {
		return fmt.Errorf("Failed to load TLS configurations after applying auto-config settings: %w", err)
	}

	// This needs to happen after the initial auto-config is loaded, because TLS
	// can only be configured on the gRPC server at the point of creation.
	a.externalGRPCServer = external.NewServer(
		a.logger.Named("grpc.external"),
		metrics.Default(),
		a.tlsConfigurator,
	)

	if err := a.startLicenseManager(ctx); err != nil {
		return err
	}

	// create the local state
	a.State = local.NewState(LocalConfig(c), a.logger, a.tokens)

	// create the state synchronization manager which performs
	// regular and on-demand state synchronizations (anti-entropy).
	a.sync = ae.NewStateSyncer(a.State, c.AEInterval, a.shutdownCh, a.logger)

	// create the config for the rpc server/client
	consulCfg, err := newConsulConfig(a.config, a.logger)
	if err != nil {
		return err
	}

	// Setup the user event callback
	consulCfg.UserEventHandler = func(e serf.UserEvent) {
		select {
		case a.eventCh <- e:
		case <-a.shutdownCh:
		}
	}

	// ServerUp is used to inform that a new consul server is now
	// up. This can be used to speed up the sync process if we are blocking
	// waiting to discover a consul server
	consulCfg.ServerUp = a.sync.SyncFull.Trigger

	err = a.initEnterprise(consulCfg)
	if err != nil {
		return fmt.Errorf("failed to start Consul enterprise component: %v", err)
	}

	// Setup either the client or the server.
	if c.ServerMode {
		server, err := consul.NewServer(consulCfg, a.baseDeps.Deps, a.externalGRPCServer)
		if err != nil {
			return fmt.Errorf("Failed to start Consul server: %v", err)
		}
		a.delegate = server

		if a.config.PeeringEnabled && a.config.ConnectEnabled {
			d := servercert.Deps{
				Logger: a.logger.Named("server.cert-manager"),
				Config: servercert.Config{
					Datacenter:  a.config.Datacenter,
					ACLsEnabled: a.config.ACLsEnabled,
				},
				Cache:           a.cache,
				GetStore:        func() servercert.Store { return server.FSM().State() },
				TLSConfigurator: a.tlsConfigurator,
			}
			a.certManager = servercert.NewCertManager(d)
			if err := a.certManager.Start(&lib.StopChannelContext{StopCh: a.shutdownCh}); err != nil {
				return fmt.Errorf("failed to start server cert manager: %w", err)
			}
		}

	} else {
		client, err := consul.NewClient(consulCfg, a.baseDeps.Deps)
		if err != nil {
			return fmt.Errorf("Failed to start Consul client: %v", err)
		}
		a.delegate = client
	}

	// The staggering of the state syncing depends on the cluster size.
	//
	// NOTE: we will use the agent's canonical serf pool for this since that's
	// similarly scoped with the state store side of anti-entropy.
	a.sync.ClusterSize = func() int { return len(a.delegate.LANMembersInAgentPartition()) }

	// link the state with the consul server/client and the state syncer
	// via callbacks. After several attempts this was easier than using
	// channels since the event notification needs to be non-blocking
	// and that should be hidden in the state syncer implementation.
	a.State.Delegate = a.delegate
	a.State.TriggerSyncChanges = a.sync.SyncChanges.Trigger

	if err := a.baseDeps.AutoConfig.Start(&lib.StopChannelContext{StopCh: a.shutdownCh}); err != nil {
		return fmt.Errorf("AutoConf failed to start certificate monitor: %w", err)
	}

	// Load checks/services/metadata.
	emptyCheckSnapshot := map[structs.CheckID]*structs.HealthCheck{}
	if err := a.loadServices(c, emptyCheckSnapshot); err != nil {
		return err
	}
	if err := a.loadChecks(c, nil); err != nil {
		return err
	}
	if err := a.loadMetadata(c); err != nil {
		return err
	}

	var intentionDefaultAllow bool
	switch a.config.ACLResolverSettings.ACLDefaultPolicy {
	case "allow":
		intentionDefaultAllow = true
	case "deny":
		intentionDefaultAllow = false
	default:
		return fmt.Errorf("unexpected ACL default policy value of %q", a.config.ACLResolverSettings.ACLDefaultPolicy)
	}

	go a.baseDeps.ViewStore.Run(&lib.StopChannelContext{StopCh: a.shutdownCh})

	// Start the proxy config manager.
	a.proxyConfig, err = proxycfg.NewManager(proxycfg.ManagerConfig{
		DataSources: a.proxyDataSources(),
		Logger:      a.logger.Named(logging.ProxyConfig),
		Source: &structs.QuerySource{
			Datacenter:    a.config.Datacenter,
			Segment:       a.config.SegmentName,
			Node:          a.config.NodeName,
			NodePartition: a.config.PartitionOrEmpty(),
		},
		DNSConfig: proxycfg.DNSConfig{
			Domain:    a.config.DNSDomain,
			AltDomain: a.config.DNSAltDomain,
		},
		TLSConfigurator:       a.tlsConfigurator,
		IntentionDefaultAllow: intentionDefaultAllow,
		UpdateRateLimit:       a.config.XDSUpdateRateLimit,
	})
	if err != nil {
		return err
	}

	go localproxycfg.Sync(
		&lib.StopChannelContext{StopCh: a.shutdownCh},
		localproxycfg.SyncConfig{
			Manager:         a.proxyConfig,
			State:           a.State,
			Logger:          a.proxyConfig.Logger.Named("agent-state"),
			Tokens:          a.baseDeps.Tokens,
			NodeName:        a.config.NodeName,
			ResyncFrequency: a.config.LocalProxyConfigResyncInterval,
		},
	)

	// Start watching for critical services to deregister, based on their
	// checks.
	go a.reapServices()

	// Start handling events.
	go a.handleEvents()

	// Start sending network coordinate to the server.
	if !c.DisableCoordinates {
		go a.sendCoordinate()
	}

	// Write out the PID file if necessary.
	if err := a.storePid(); err != nil {
		return err
	}

	// start DNS servers
	if err := a.listenAndServeDNS(); err != nil {
		return err
	}

	// Configure the http connection limiter.
	a.httpConnLimiter.SetConfig(connlimit.Config{
		MaxConnsPerClientIP: a.config.HTTPMaxConnsPerClient,
	})

	// Create listeners and unstarted servers; see comment on listenHTTP why
	// we are doing this.
	servers, err := a.listenHTTP()
	if err != nil {
		return err
	}

	// Start HTTP and HTTPS servers.
	for _, srv := range servers {
		a.apiServers.Start(srv)
	}

	// Start grpc and grpc_tls servers.
	if err := a.listenAndServeGRPC(); err != nil {
		return err
	}

	// Start a goroutine to terminate excess xDS sessions.
	go a.baseDeps.XDSStreamLimiter.Run(&lib.StopChannelContext{StopCh: a.shutdownCh})

	// register watches
	if err := a.reloadWatches(a.config); err != nil {
		return err
	}

	// start retry join
	go a.retryJoinLAN()
	if a.config.ServerMode {
		go a.retryJoinWAN()
	}

	if a.tlsConfigurator.Cert() != nil {
		m := tlsCertExpirationMonitor(a.tlsConfigurator, a.logger)
		go m.Monitor(&lib.StopChannelContext{StopCh: a.shutdownCh})
	}

	// consul version metric with labels
	metrics.SetGaugeWithLabels([]string{"version"}, 1, []metrics.Label{
		{Name: "version", Value: a.config.VersionWithMetadata()},
		{Name: "pre_release", Value: a.config.VersionPrerelease},
	})

	// start a go routine to reload config based on file watcher events
	if a.configFileWatcher != nil {
		a.baseDeps.Logger.Debug("starting file watcher")
		a.configFileWatcher.Start(context.Background())
		go func() {
			for event := range a.configFileWatcher.EventsCh() {
				a.baseDeps.Logger.Debug("auto-reload config triggered", "num-events", len(event.Filenames))
				err := a.AutoReloadConfig()
				if err != nil {
					a.baseDeps.Logger.Error("error loading config", "error", err)
				}
			}
		}()
	}

	if a.scadaProvider != nil {
		a.scadaProvider.UpdateMeta(map[string]string{
			"consul_server_id": string(a.config.NodeID),
		})

		if err = a.scadaProvider.Start(); err != nil {
			a.baseDeps.Logger.Error("scada provider failed to start, some HashiCorp Cloud Platform functionality has been disabled",
				"error", err, "resource_id", a.config.Cloud.ResourceID)
		}
	}

	return nil
}

var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"version"},
		Help: "Represents the Consul version.",
	},
}

// Failed returns a channel which is closed when the first server goroutine exits
// with a non-nil error.
func (a *Agent) Failed() <-chan struct{} {
	return a.apiServers.failed
}

func (a *Agent) listenAndServeGRPC() error {
	if len(a.config.GRPCAddrs) < 1 && len(a.config.GRPCTLSAddrs) < 1 {
		return nil
	}
	// TODO(agentless): rather than asserting the concrete type of delegate, we
	// should add a method to the Delegate interface to build a ConfigSource.
	var cfg xds.ProxyConfigSource = localproxycfg.NewConfigSource(a.proxyConfig)
	if server, ok := a.delegate.(*consul.Server); ok {
		catalogCfg := catalogproxycfg.NewConfigSource(catalogproxycfg.Config{
			NodeName:          a.config.NodeName,
			LocalState:        a.State,
			LocalConfigSource: cfg,
			Manager:           a.proxyConfig,
			GetStore:          func() catalogproxycfg.Store { return server.FSM().State() },
			Logger:            a.proxyConfig.Logger.Named("server-catalog"),
			SessionLimiter:    a.baseDeps.XDSStreamLimiter,
		})
		go func() {
			<-a.shutdownCh
			catalogCfg.Shutdown()
		}()
		cfg = catalogCfg
	}
	a.xdsServer = xds.NewServer(
		a.config.NodeName,
		a.logger.Named(logging.Envoy),
		a.config.ConnectServerlessPluginEnabled,
		cfg,
		func(id string) (acl.Authorizer, error) {
			return a.delegate.ResolveTokenAndDefaultMeta(id, nil, nil)
		},
		a,
	)
	a.xdsServer.Register(a.externalGRPCServer)

	// Attempt to spawn listeners
	var listeners []net.Listener
	start := func(port_name string, addrs []net.Addr, protocol middleware.Protocol) error {
		if len(addrs) < 1 {
			return nil
		}

		ln, err := a.startListeners(addrs)
		if err != nil {
			return err
		}
		for i := range ln {
			ln[i] = middleware.LabelledListener{Listener: ln[i], Protocol: protocol}
			listeners = append(listeners, ln[i])
		}

		for _, l := range ln {
			go func(innerL net.Listener) {
				a.logger.Info("Started gRPC listeners",
					"port_name", port_name,
					"address", innerL.Addr().String(),
					"network", innerL.Addr().Network(),
				)
				err := a.externalGRPCServer.Serve(innerL)
				if err != nil {
					a.logger.Error("gRPC server failed", "port_name", port_name, "error", err)
				}
			}(l)
		}
		return nil
	}

	// Only allow grpc to spawn with a plain-text listener.
	if a.config.GRPCPort > 0 {
		if err := start("grpc", a.config.GRPCAddrs, middleware.ProtocolPlaintext); err != nil {
			closeListeners(listeners)
			return err
		}
	}
	// Only allow grpc_tls to spawn with a TLS listener.
	if a.config.GRPCTLSPort > 0 {
		if err := start("grpc_tls", a.config.GRPCTLSAddrs, middleware.ProtocolTLS); err != nil {
			closeListeners(listeners)
			return err
		}
	}
	return nil
}

func (a *Agent) listenAndServeDNS() error {
	notif := make(chan net.Addr, len(a.config.DNSAddrs))
	errCh := make(chan error, len(a.config.DNSAddrs))
	for _, addr := range a.config.DNSAddrs {
		// create server
		s, err := NewDNSServer(a)
		if err != nil {
			return err
		}
		a.dnsServers = append(a.dnsServers, s)

		// start server
		a.wgServers.Add(1)
		go func(addr net.Addr) {
			defer a.wgServers.Done()
			err := s.ListenAndServe(addr.Network(), addr.String(), func() { notif <- addr })
			if err != nil && !strings.Contains(err.Error(), "accept") {
				errCh <- err
			}
		}(addr)
	}
	s, _ := NewDNSServer(a)

	grpcDNS.NewServer(grpcDNS.Config{
		Logger:      a.logger.Named("grpc-api.dns"),
		DNSServeMux: s.mux,
		LocalAddr:   grpcDNS.LocalAddr{IP: net.IPv4(127, 0, 0, 1), Port: a.config.GRPCPort},
	}).Register(a.externalGRPCServer)

	a.dnsServers = append(a.dnsServers, s)

	// wait for servers to be up
	timeout := time.After(time.Second)
	var merr *multierror.Error
	for range a.config.DNSAddrs {
		select {
		case addr := <-notif:
			a.logger.Info("Started DNS server",
				"address", addr.String(),
				"network", addr.Network(),
			)

		case err := <-errCh:
			merr = multierror.Append(merr, err)
		case <-timeout:
			merr = multierror.Append(merr, fmt.Errorf("agent: timeout starting DNS servers"))
			return merr.ErrorOrNil()
		}
	}
	return merr.ErrorOrNil()
}

// startListeners will return a net.Listener for every address unless an
// error is encountered, in which case it will close all previously opened
// listeners and return the error.
func (a *Agent) startListeners(addrs []net.Addr) ([]net.Listener, error) {
	var lns []net.Listener

	closeAll := func() {
		for _, l := range lns {
			l.Close()
		}
	}

	for _, addr := range addrs {
		var l net.Listener
		var err error

		switch x := addr.(type) {
		case *net.UnixAddr:
			l, err = a.listenSocket(x.Name)
			if err != nil {
				closeAll()
				return nil, err
			}

		case *net.TCPAddr:
			l, err = net.Listen("tcp", x.String())
			if err != nil {
				closeAll()
				return nil, err
			}
			l = &tcpKeepAliveListener{l.(*net.TCPListener)}

		case *capability.Addr:
			l, err = a.scadaProvider.Listen(x.Capability())
			if err != nil {
				return nil, err
			}

		default:
			closeAll()
			return nil, fmt.Errorf("unsupported address type %T", addr)
		}
		lns = append(lns, l)
	}
	return lns, nil
}

// listenHTTP binds listeners to the provided addresses and also returns
// pre-configured HTTP servers which are not yet started. The motivation is
// that in the current startup/shutdown setup we de-couple the listener
// creation from the server startup assuming that if any of the listeners
// cannot be bound we fail immediately and later failures do not occur.
// Therefore, starting a server with a running listener is assumed to not
// produce an error.
//
// The second motivation is that an HTTPS server needs to use the same TLSConfig
// on both the listener and the HTTP server. When listeners and servers are
// created at different times this becomes difficult to handle without keeping
// the TLS configuration somewhere or recreating it.
//
// This approach should ultimately be refactored to the point where we just
// start the server and any error should trigger a proper shutdown of the agent.
func (a *Agent) listenHTTP() ([]apiServer, error) {
	var ln []net.Listener
	var servers []apiServer

	start := func(proto string, addrs []net.Addr) error {
		listeners, err := a.startListeners(addrs)
		if err != nil {
			return err
		}
		ln = append(ln, listeners...)

		for _, l := range listeners {
			var tlscfg *tls.Config
			_, isTCP := l.(*tcpKeepAliveListener)
			isUnix := l.Addr().Network() == "unix"
			if (isTCP || isUnix) && proto == "https" {
				tlscfg = a.tlsConfigurator.IncomingHTTPSConfig()
				l = tls.NewListener(l, tlscfg)
			}

			srv := &HTTPHandlers{
				agent:          a,
				denylist:       NewDenylist(a.config.HTTPBlockEndpoints),
				proxyTransport: http.DefaultTransport,
			}
			a.configReloaders = append(a.configReloaders, srv.ReloadConfig)
			a.httpHandlers = srv
			httpServer := &http.Server{
				Addr:           l.Addr().String(),
				TLSConfig:      tlscfg,
				Handler:        srv.handler(),
				MaxHeaderBytes: a.config.HTTPMaxHeaderBytes,
			}

			if libscada.IsCapability(l.Addr()) {
				// wrap in http2 server handler
				httpServer.Handler = h2c.NewHandler(srv.handler(), &http2.Server{})
			}

			// Load the connlimit helper into the server
			connLimitFn := a.httpConnLimiter.HTTPConnStateFuncWithDefault429Handler(10 * time.Millisecond)

			if proto == "https" {
				if err := setupHTTPS(httpServer, connLimitFn, a.config.HTTPSHandshakeTimeout); err != nil {
					return err
				}
			} else {
				httpServer.ConnState = connLimitFn
			}

			servers = append(servers, newAPIServerHTTP(proto, l, httpServer))
		}
		return nil
	}

	httpAddrs := a.config.HTTPAddrs
	if a.config.IsCloudEnabled() {
		httpAddrs = append(httpAddrs, scada.CAPCoreAPI)
	}

	if err := start("http", httpAddrs); err != nil {
		closeListeners(ln)
		return nil, err
	}
	if err := start("https", a.config.HTTPSAddrs); err != nil {
		closeListeners(ln)
		return nil, err
	}
	return servers, nil
}

func closeListeners(lns []net.Listener) {
	for _, l := range lns {
		l.Close()
	}
}

// setupHTTPS adds HTTP/2 support, ConnState, and a connection handshake timeout
// to the http.Server.
func setupHTTPS(server *http.Server, connState func(net.Conn, http.ConnState), timeout time.Duration) error {
	// Enforce TLS handshake timeout
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			// Set deadline to prevent slow send before TLS handshake or first
			// byte of request.
			conn.SetReadDeadline(time.Now().Add(timeout))
		case http.StateActive:
			// Clear read deadline. We should maybe set read timeouts more
			// generally but that's a bigger task as some HTTP endpoints may
			// stream large requests and responses (e.g. snapshot) so we can't
			// set sensible blanket timeouts here.
			conn.SetReadDeadline(time.Time{})
		}
		// Pass through to conn limit. This is OK because we didn't change
		// state (i.e. Close conn).
		connState(conn, state)
	}

	// This will enable upgrading connections to HTTP/2 as
	// part of TLS negotiation.
	return http2.ConfigureServer(server, nil)
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used so dead TCP connections eventually go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(30 * time.Second)
	return tc, nil
}

func (a *Agent) listenSocket(path string) (net.Listener, error) {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		a.logger.Warn("Replacing socket", "path", path)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error removing socket file: %s", err)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	user, group, mode := a.config.UnixSocketUser, a.config.UnixSocketGroup, a.config.UnixSocketMode
	if err := setFilePermissions(path, user, group, mode); err != nil {
		return nil, fmt.Errorf("Failed setting up socket: %s", err)
	}
	return l, nil
}

// stopAllWatches stops all the currently running watches
func (a *Agent) stopAllWatches() {
	for _, wp := range a.watchPlans {
		wp.Stop()
	}
}

// reloadWatches stops any existing watch plans and attempts to load the given
// set of watches.
func (a *Agent) reloadWatches(cfg *config.RuntimeConfig) error {
	// Stop the current watches.
	a.stopAllWatches()
	a.watchPlans = nil

	// Return if there are no watches now.
	if len(cfg.Watches) == 0 {
		return nil
	}

	// Watches use the API to talk to this agent, so that must be enabled.
	if len(cfg.HTTPAddrs) == 0 && len(cfg.HTTPSAddrs) == 0 {
		return fmt.Errorf("watch plans require an HTTP or HTTPS endpoint")
	}

	// Compile the watches
	var watchPlans []*watch.Plan
	for _, params := range cfg.Watches {
		if handlerType, ok := params["handler_type"]; !ok {
			params["handler_type"] = "script"
		} else if handlerType != "http" && handlerType != "script" {
			return fmt.Errorf("Handler type '%s' not recognized", params["handler_type"])
		}

		// Don't let people use connect watches via this mechanism for now as it
		// needs thought about how to do securely and shouldn't be necessary. Note
		// that if the type assertion fails an type is not a string then
		// ParseExample below will error so we don't need to handle that case.
		if typ, ok := params["type"].(string); ok {
			if strings.HasPrefix(typ, "connect_") {
				return fmt.Errorf("Watch type %s is not allowed in agent config", typ)
			}
		}

		wp, err := makeWatchPlan(a.logger, params)
		if err != nil {
			return err
		}
		watchPlans = append(watchPlans, wp)
	}

	// Fire off a goroutine for each new watch plan.
	for _, wp := range watchPlans {
		config, err := a.config.APIConfig(true)
		if err != nil {
			a.logger.Error("Failed to run watch", "error", err)
			continue
		}

		a.watchPlans = append(a.watchPlans, wp)
		go func(wp *watch.Plan) {
			if h, ok := wp.Exempt["handler"]; ok {
				wp.Handler = makeWatchHandler(a.logger, h)
			} else if h, ok := wp.Exempt["args"]; ok {
				wp.Handler = makeWatchHandler(a.logger, h)
			} else {
				httpConfig := wp.Exempt["http_handler_config"].(*watch.HttpHandlerConfig)
				wp.Handler = makeHTTPWatchHandler(a.logger, httpConfig)
			}
			wp.Logger = a.logger.Named("watch")

			addr := config.Address
			if config.Scheme == "https" {
				addr = "https://" + addr
			}

			if err := wp.RunWithConfig(addr, config); err != nil {
				a.logger.Error("Failed to run watch", "error", err)
			}
		}(wp)
	}
	return nil
}

// newConsulConfig translates a RuntimeConfig into a consul.Config.
// TODO: move this function to a different file, maybe config.go
func newConsulConfig(runtimeCfg *config.RuntimeConfig, logger hclog.Logger) (*consul.Config, error) {
	cfg := consul.DefaultConfig()

	// This is set when the agent starts up
	cfg.NodeID = runtimeCfg.NodeID

	// Apply dev mode
	cfg.DevMode = runtimeCfg.DevMode

	// Override with our runtimeCfg
	// todo(fs): these are now always set in the runtime runtimeCfg so we can simplify this
	// todo(fs): or is there a reason to keep it like that?
	cfg.Datacenter = runtimeCfg.Datacenter
	cfg.PrimaryDatacenter = runtimeCfg.PrimaryDatacenter
	cfg.DataDir = runtimeCfg.DataDir
	cfg.NodeName = runtimeCfg.NodeName
	cfg.ACLResolverSettings = runtimeCfg.ACLResolverSettings

	cfg.CoordinateUpdateBatchSize = runtimeCfg.ConsulCoordinateUpdateBatchSize
	cfg.CoordinateUpdateMaxBatches = runtimeCfg.ConsulCoordinateUpdateMaxBatches
	cfg.CoordinateUpdatePeriod = runtimeCfg.ConsulCoordinateUpdatePeriod
	cfg.CheckOutputMaxSize = runtimeCfg.CheckOutputMaxSize

	cfg.RaftConfig.HeartbeatTimeout = runtimeCfg.ConsulRaftHeartbeatTimeout
	cfg.RaftConfig.LeaderLeaseTimeout = runtimeCfg.ConsulRaftLeaderLeaseTimeout
	cfg.RaftConfig.ElectionTimeout = runtimeCfg.ConsulRaftElectionTimeout

	cfg.SerfLANConfig.MemberlistConfig.BindAddr = runtimeCfg.SerfBindAddrLAN.IP.String()
	cfg.SerfLANConfig.MemberlistConfig.BindPort = runtimeCfg.SerfBindAddrLAN.Port
	cfg.SerfLANConfig.MemberlistConfig.CIDRsAllowed = runtimeCfg.SerfAllowedCIDRsLAN
	cfg.SerfWANConfig.MemberlistConfig.CIDRsAllowed = runtimeCfg.SerfAllowedCIDRsWAN
	cfg.SerfLANConfig.MemberlistConfig.AdvertiseAddr = runtimeCfg.SerfAdvertiseAddrLAN.IP.String()
	cfg.SerfLANConfig.MemberlistConfig.AdvertisePort = runtimeCfg.SerfAdvertiseAddrLAN.Port
	cfg.SerfLANConfig.MemberlistConfig.GossipVerifyIncoming = runtimeCfg.StaticRuntimeConfig.EncryptVerifyIncoming
	cfg.SerfLANConfig.MemberlistConfig.GossipVerifyOutgoing = runtimeCfg.StaticRuntimeConfig.EncryptVerifyOutgoing
	cfg.SerfLANConfig.MemberlistConfig.GossipInterval = runtimeCfg.GossipLANGossipInterval
	cfg.SerfLANConfig.MemberlistConfig.GossipNodes = runtimeCfg.GossipLANGossipNodes
	cfg.SerfLANConfig.MemberlistConfig.ProbeInterval = runtimeCfg.GossipLANProbeInterval
	cfg.SerfLANConfig.MemberlistConfig.ProbeTimeout = runtimeCfg.GossipLANProbeTimeout
	cfg.SerfLANConfig.MemberlistConfig.SuspicionMult = runtimeCfg.GossipLANSuspicionMult
	cfg.SerfLANConfig.MemberlistConfig.RetransmitMult = runtimeCfg.GossipLANRetransmitMult
	if runtimeCfg.ReconnectTimeoutLAN != 0 {
		cfg.SerfLANConfig.ReconnectTimeout = runtimeCfg.ReconnectTimeoutLAN
	}

	if runtimeCfg.SerfBindAddrWAN != nil {
		cfg.SerfWANConfig.MemberlistConfig.BindAddr = runtimeCfg.SerfBindAddrWAN.IP.String()
		cfg.SerfWANConfig.MemberlistConfig.BindPort = runtimeCfg.SerfBindAddrWAN.Port
		cfg.SerfWANConfig.MemberlistConfig.AdvertiseAddr = runtimeCfg.SerfAdvertiseAddrWAN.IP.String()
		cfg.SerfWANConfig.MemberlistConfig.AdvertisePort = runtimeCfg.SerfAdvertiseAddrWAN.Port
		cfg.SerfWANConfig.MemberlistConfig.GossipVerifyIncoming = runtimeCfg.StaticRuntimeConfig.EncryptVerifyIncoming
		cfg.SerfWANConfig.MemberlistConfig.GossipVerifyOutgoing = runtimeCfg.StaticRuntimeConfig.EncryptVerifyOutgoing
		cfg.SerfWANConfig.MemberlistConfig.GossipInterval = runtimeCfg.GossipWANGossipInterval
		cfg.SerfWANConfig.MemberlistConfig.GossipNodes = runtimeCfg.GossipWANGossipNodes
		cfg.SerfWANConfig.MemberlistConfig.ProbeInterval = runtimeCfg.GossipWANProbeInterval
		cfg.SerfWANConfig.MemberlistConfig.ProbeTimeout = runtimeCfg.GossipWANProbeTimeout
		cfg.SerfWANConfig.MemberlistConfig.SuspicionMult = runtimeCfg.GossipWANSuspicionMult
		cfg.SerfWANConfig.MemberlistConfig.RetransmitMult = runtimeCfg.GossipWANRetransmitMult
		if runtimeCfg.ReconnectTimeoutWAN != 0 {
			cfg.SerfWANConfig.ReconnectTimeout = runtimeCfg.ReconnectTimeoutWAN
		}
	} else {
		// Disable serf WAN federation
		cfg.SerfWANConfig = nil
	}

	cfg.AdvertiseReconnectTimeout = runtimeCfg.AdvertiseReconnectTimeout

	cfg.RPCAddr = runtimeCfg.RPCBindAddr
	cfg.RPCAdvertise = runtimeCfg.RPCAdvertiseAddr

	cfg.GRPCPort = runtimeCfg.GRPCPort
	cfg.GRPCTLSPort = runtimeCfg.GRPCTLSPort

	cfg.Segment = runtimeCfg.SegmentName
	if len(runtimeCfg.Segments) > 0 {
		segments, err := segmentConfig(runtimeCfg)
		if err != nil {
			return nil, err
		}
		cfg.Segments = segments
	}
	if runtimeCfg.Bootstrap {
		cfg.Bootstrap = true
	}
	if runtimeCfg.CheckOutputMaxSize > 0 {
		cfg.CheckOutputMaxSize = runtimeCfg.CheckOutputMaxSize
	}
	if runtimeCfg.RejoinAfterLeave {
		cfg.RejoinAfterLeave = true
	}
	if runtimeCfg.BootstrapExpect != 0 {
		cfg.BootstrapExpect = runtimeCfg.BootstrapExpect
	}
	if runtimeCfg.RPCProtocol > 0 {
		cfg.ProtocolVersion = uint8(runtimeCfg.RPCProtocol)
	}
	if runtimeCfg.RaftProtocol != 0 {
		cfg.RaftConfig.ProtocolVersion = raft.ProtocolVersion(runtimeCfg.RaftProtocol)
	}
	if runtimeCfg.RaftSnapshotThreshold != 0 {
		cfg.RaftConfig.SnapshotThreshold = uint64(runtimeCfg.RaftSnapshotThreshold)
	}
	if runtimeCfg.RaftSnapshotInterval != 0 {
		cfg.RaftConfig.SnapshotInterval = runtimeCfg.RaftSnapshotInterval
	}
	if runtimeCfg.RaftTrailingLogs != 0 {
		cfg.RaftConfig.TrailingLogs = uint64(runtimeCfg.RaftTrailingLogs)
	}
	if runtimeCfg.ACLInitialManagementToken != "" {
		cfg.ACLInitialManagementToken = runtimeCfg.ACLInitialManagementToken
	}
	cfg.ACLTokenReplication = runtimeCfg.ACLTokenReplication
	cfg.ACLsEnabled = runtimeCfg.ACLsEnabled
	if runtimeCfg.ACLEnableKeyListPolicy {
		cfg.ACLEnableKeyListPolicy = runtimeCfg.ACLEnableKeyListPolicy
	}
	if runtimeCfg.SessionTTLMin != 0 {
		cfg.SessionTTLMin = runtimeCfg.SessionTTLMin
	}
	if runtimeCfg.ReadReplica {
		cfg.ReadReplica = runtimeCfg.ReadReplica
	}

	// These are fully specified in the agent defaults, so we can simply
	// copy them over.
	cfg.AutopilotConfig.CleanupDeadServers = runtimeCfg.AutopilotCleanupDeadServers
	cfg.AutopilotConfig.LastContactThreshold = runtimeCfg.AutopilotLastContactThreshold
	cfg.AutopilotConfig.MaxTrailingLogs = uint64(runtimeCfg.AutopilotMaxTrailingLogs)
	cfg.AutopilotConfig.MinQuorum = runtimeCfg.AutopilotMinQuorum
	cfg.AutopilotConfig.ServerStabilizationTime = runtimeCfg.AutopilotServerStabilizationTime
	cfg.AutopilotConfig.RedundancyZoneTag = runtimeCfg.AutopilotRedundancyZoneTag
	cfg.AutopilotConfig.DisableUpgradeMigration = runtimeCfg.AutopilotDisableUpgradeMigration
	cfg.AutopilotConfig.UpgradeVersionTag = runtimeCfg.AutopilotUpgradeVersionTag

	// make sure the advertise address is always set
	if cfg.RPCAdvertise == nil {
		cfg.RPCAdvertise = cfg.RPCAddr
	}

	// Rate limiting for RPC calls.
	if runtimeCfg.RPCRateLimit > 0 {
		cfg.RPCRateLimit = runtimeCfg.RPCRateLimit
	}
	if runtimeCfg.RPCMaxBurst > 0 {
		cfg.RPCMaxBurst = runtimeCfg.RPCMaxBurst
	}

	// RPC timeouts/limits.
	if runtimeCfg.RPCHandshakeTimeout > 0 {
		cfg.RPCHandshakeTimeout = runtimeCfg.RPCHandshakeTimeout
	}
	if runtimeCfg.RPCMaxConnsPerClient > 0 {
		cfg.RPCMaxConnsPerClient = runtimeCfg.RPCMaxConnsPerClient
	}

	// RPC-related performance configs. We allow explicit zero value to disable so
	// copy it whatever the value.
	cfg.RPCHoldTimeout = runtimeCfg.RPCHoldTimeout
	cfg.RPCClientTimeout = runtimeCfg.RPCClientTimeout

	cfg.RPCConfig = runtimeCfg.RPCConfig

	if runtimeCfg.LeaveDrainTime > 0 {
		cfg.LeaveDrainTime = runtimeCfg.LeaveDrainTime
	}

	// set the src address for outgoing rpc connections
	// Use port 0 so that outgoing connections use a random port.
	if !ipaddr.IsAny(cfg.RPCAddr.IP) {
		cfg.RPCSrcAddr = &net.TCPAddr{IP: cfg.RPCAddr.IP}
	}

	// Format the build string
	revision := runtimeCfg.Revision
	if len(revision) > 8 {
		revision = revision[:8]
	}
	cfg.Build = fmt.Sprintf("%s%s:%s", runtimeCfg.VersionWithMetadata(), runtimeCfg.VersionPrerelease, revision)

	cfg.TLSConfig = runtimeCfg.TLS

	cfg.DefaultQueryTime = runtimeCfg.DefaultQueryTime
	cfg.MaxQueryTime = runtimeCfg.MaxQueryTime

	cfg.AutoEncryptAllowTLS = runtimeCfg.AutoEncryptAllowTLS

	// Copy the Connect CA bootstrap runtimeCfg
	if runtimeCfg.ConnectEnabled {
		cfg.ConnectEnabled = true
		cfg.ConnectMeshGatewayWANFederationEnabled = runtimeCfg.ConnectMeshGatewayWANFederationEnabled

		ca, err := runtimeCfg.ConnectCAConfiguration()
		if err != nil {
			return nil, err
		}

		cfg.CAConfig = ca
	}

	// copy over auto runtimeCfg settings
	cfg.AutoConfigEnabled = runtimeCfg.AutoConfig.Enabled
	cfg.AutoConfigIntroToken = runtimeCfg.AutoConfig.IntroToken
	cfg.AutoConfigIntroTokenFile = runtimeCfg.AutoConfig.IntroTokenFile
	cfg.AutoConfigServerAddresses = runtimeCfg.AutoConfig.ServerAddresses
	cfg.AutoConfigDNSSANs = runtimeCfg.AutoConfig.DNSSANs
	cfg.AutoConfigIPSANs = runtimeCfg.AutoConfig.IPSANs
	cfg.AutoConfigAuthzEnabled = runtimeCfg.AutoConfig.Authorizer.Enabled
	cfg.AutoConfigAuthzAuthMethod = runtimeCfg.AutoConfig.Authorizer.AuthMethod
	cfg.AutoConfigAuthzClaimAssertions = runtimeCfg.AutoConfig.Authorizer.ClaimAssertions
	cfg.AutoConfigAuthzAllowReuse = runtimeCfg.AutoConfig.Authorizer.AllowReuse

	// This will set up the LAN keyring, as well as the WAN and any segments
	// for servers.
	// TODO: move this closer to where the keyrings will be used.
	if err := setupKeyrings(cfg, runtimeCfg, logger); err != nil {
		return nil, fmt.Errorf("Failed to configure keyring: %v", err)
	}

	cfg.ConfigEntryBootstrap = runtimeCfg.ConfigEntryBootstrap
	cfg.RaftBoltDBConfig = runtimeCfg.RaftBoltDBConfig

	// Duplicate our own serf config once to make sure that the duplication
	// function does not drift.
	cfg.SerfLANConfig = consul.CloneSerfLANConfig(cfg.SerfLANConfig)

	cfg.PeeringEnabled = runtimeCfg.PeeringEnabled
	cfg.PeeringTestAllowPeerRegistrations = runtimeCfg.PeeringTestAllowPeerRegistrations

	cfg.Cloud.ManagementToken = runtimeCfg.Cloud.ManagementToken

	cfg.Reporting.License.Enabled = runtimeCfg.Reporting.License.Enabled

	enterpriseConsulConfig(cfg, runtimeCfg)

	return cfg, nil
}

// Setup the serf and memberlist config for any defined network segments.
func segmentConfig(config *config.RuntimeConfig) ([]consul.NetworkSegment, error) {
	var segments []consul.NetworkSegment

	for _, s := range config.Segments {
		// TODO: use consul.CloneSerfLANConfig(config.SerfLANConfig) here?
		serfConf := consul.DefaultConfig().SerfLANConfig

		serfConf.MemberlistConfig.BindAddr = s.Bind.IP.String()
		serfConf.MemberlistConfig.BindPort = s.Bind.Port
		serfConf.MemberlistConfig.AdvertiseAddr = s.Advertise.IP.String()
		serfConf.MemberlistConfig.AdvertisePort = s.Advertise.Port
		serfConf.MemberlistConfig.CIDRsAllowed = config.SerfAllowedCIDRsLAN

		if config.ReconnectTimeoutLAN != 0 {
			serfConf.ReconnectTimeout = config.ReconnectTimeoutLAN
		}
		if config.StaticRuntimeConfig.EncryptVerifyIncoming {
			serfConf.MemberlistConfig.GossipVerifyIncoming = config.StaticRuntimeConfig.EncryptVerifyIncoming
		}
		if config.StaticRuntimeConfig.EncryptVerifyOutgoing {
			serfConf.MemberlistConfig.GossipVerifyOutgoing = config.StaticRuntimeConfig.EncryptVerifyOutgoing
		}

		var rpcAddr *net.TCPAddr
		if s.RPCListener {
			rpcAddr = &net.TCPAddr{
				IP:   s.Bind.IP,
				Port: config.ServerPort,
			}
		}

		segments = append(segments, consul.NetworkSegment{
			Name:       s.Name,
			Bind:       serfConf.MemberlistConfig.BindAddr,
			Advertise:  serfConf.MemberlistConfig.AdvertiseAddr,
			Port:       s.Bind.Port,
			RPCAddr:    rpcAddr,
			SerfConfig: serfConf,
		})
	}

	return segments, nil
}

// registerEndpoint registers a handler for the consul RPC server
// under a unique name while making it accessible under the provided
// name. This allows overwriting handlers for the golang net/rpc
// service which does not allow this.
func (a *Agent) registerEndpoint(name string, handler interface{}) error {
	srv, ok := a.delegate.(*consul.Server)
	if !ok {
		panic("agent must be a server")
	}
	realname := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	a.endpointsLock.Lock()
	a.endpoints[name] = realname
	a.endpointsLock.Unlock()
	return srv.RegisterEndpoint(realname, handler)
}

// RPC is used to make an RPC call to the Consul servers
// This allows the agent to implement the Consul.Interface
func (a *Agent) RPC(method string, args interface{}, reply interface{}) error {
	a.endpointsLock.RLock()
	// fast path: only translate if there are overrides
	if len(a.endpoints) > 0 {
		p := strings.SplitN(method, ".", 2)
		if e := a.endpoints[p[0]]; e != "" {
			method = e + "." + p[1]
		}
	}
	a.endpointsLock.RUnlock()
	return a.delegate.RPC(method, args, reply)
}

// Leave is used to prepare the agent for a graceful shutdown
func (a *Agent) Leave() error {
	return a.delegate.Leave()
}

// ShutdownAgent is used to hard stop the agent. Should be preceded by
// Leave to do it gracefully. Should be followed by ShutdownEndpoints to
// terminate the HTTP and DNS servers as well.
func (a *Agent) ShutdownAgent() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}
	a.logger.Info("Requesting shutdown")
	// Stop the watches to avoid any notification/state change during shutdown
	a.stopAllWatches()

	// Stop config file watcher
	if a.configFileWatcher != nil {
		a.configFileWatcher.Stop()
	}

	a.stopLicenseManager()

	// this would be cancelled anyways (by the closing of the shutdown ch) but
	// this should help them to be stopped more quickly
	a.baseDeps.AutoConfig.Stop()
	a.baseDeps.MetricsConfig.Cancel()

	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	// Stop the service manager (must happen before we take the stateLock to avoid deadlock)
	if a.serviceManager != nil {
		a.serviceManager.Stop()
	}

	// Stop all the checks
	for _, chk := range a.checkMonitors {
		chk.Stop()
	}
	for _, chk := range a.checkTTLs {
		chk.Stop()
	}
	for _, chk := range a.checkHTTPs {
		chk.Stop()
	}
	for _, chk := range a.checkTCPs {
		chk.Stop()
	}
	for _, chk := range a.checkUDPs {
		chk.Stop()
	}
	for _, chk := range a.checkGRPCs {
		chk.Stop()
	}
	for _, chk := range a.checkDockers {
		chk.Stop()
	}
	for _, chk := range a.checkAliases {
		chk.Stop()
	}
	for _, chk := range a.checkH2PINGs {
		chk.Stop()
	}

	// Stop gRPC
	if a.externalGRPCServer != nil {
		a.externalGRPCServer.Stop()
	}

	// Stop the proxy config manager
	if a.proxyConfig != nil {
		a.proxyConfig.Close()
	}

	// Stop the cache background work
	if a.cache != nil {
		a.cache.Close()
	}

	a.rpcClientHealth.Close()

	// Shutdown SCADA provider
	if a.scadaProvider != nil {
		a.scadaProvider.Stop()
	}

	var err error
	if a.delegate != nil {
		err = a.delegate.Shutdown()
		if _, ok := a.delegate.(*consul.Server); ok {
			a.logger.Info("consul server down")
		} else {
			a.logger.Info("consul client down")
		}
	}

	pidErr := a.deletePid()
	if pidErr != nil {
		a.logger.Warn("could not delete pid file", "error", pidErr)
	}

	a.logger.Info("shutdown complete")
	a.shutdown = true
	close(a.shutdownCh)
	return err
}

// ShutdownEndpoints terminates the HTTP and DNS servers. Should be
// preceded by ShutdownAgent.
// TODO: remove this method, move to ShutdownAgent
func (a *Agent) ShutdownEndpoints() {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	ctx := context.TODO()

	for _, srv := range a.dnsServers {
		if srv.Server != nil {
			a.logger.Info("Stopping server",
				"protocol", "DNS",
				"address", srv.Server.Addr,
				"network", srv.Server.Net,
			)
			srv.Shutdown()
		}
	}
	a.dnsServers = nil

	a.apiServers.Shutdown(ctx)
	a.logger.Info("Waiting for endpoints to shut down")
	if err := a.apiServers.WaitForShutdown(); err != nil {
		a.logger.Error(err.Error())
	}
	a.logger.Info("Endpoints down")
}

// RetryJoinCh is a channel that transports errors
// from the retry join process.
func (a *Agent) RetryJoinCh() <-chan error {
	return a.retryJoinCh
}

// ShutdownCh is used to return a channel that can be
// selected to wait for the agent to perform a shutdown.
func (a *Agent) ShutdownCh() <-chan struct{} {
	return a.shutdownCh
}

// JoinLAN is used to have the agent join a LAN cluster
func (a *Agent) JoinLAN(addrs []string, entMeta *acl.EnterpriseMeta) (n int, err error) {
	a.logger.Info("(LAN) joining", "lan_addresses", addrs)
	n, err = a.delegate.JoinLAN(addrs, entMeta)
	if err == nil {
		a.logger.Info("(LAN) joined", "number_of_nodes", n)
		if a.joinLANNotifier != nil {
			if notifErr := a.joinLANNotifier.Notify(systemd.Ready); notifErr != nil {
				a.logger.Debug("systemd notify failed", "error", notifErr)
			}
		}
	} else {
		a.logger.Warn("(LAN) couldn't join",
			"number_of_nodes", n,
			"error", err,
		)
	}
	return
}

// JoinWAN is used to have the agent join a WAN cluster
func (a *Agent) JoinWAN(addrs []string) (n int, err error) {
	a.logger.Info("(WAN) joining", "wan_addresses", addrs)
	if srv, ok := a.delegate.(*consul.Server); ok {
		n, err = srv.JoinWAN(addrs)
	} else {
		err = fmt.Errorf("Must be a server to join WAN cluster")
	}
	if err == nil {
		a.logger.Info("(WAN) joined", "number_of_nodes", n)
	} else {
		a.logger.Warn("(WAN) couldn't join",
			"number_of_nodes", n,
			"error", err,
		)
	}
	return
}

// PrimaryMeshGatewayAddressesReadyCh returns a channel that will be closed
// when federation state replication ships back at least one primary mesh
// gateway (not via fallback config).
func (a *Agent) PrimaryMeshGatewayAddressesReadyCh() <-chan struct{} {
	if srv, ok := a.delegate.(*consul.Server); ok {
		return srv.PrimaryMeshGatewayAddressesReadyCh()
	}
	return nil
}

// PickRandomMeshGatewaySuitableForDialing is a convenience function used for writing tests.
func (a *Agent) PickRandomMeshGatewaySuitableForDialing(dc string) string {
	if srv, ok := a.delegate.(*consul.Server); ok {
		return srv.PickRandomMeshGatewaySuitableForDialing(dc)
	}
	return ""
}

// RefreshPrimaryGatewayFallbackAddresses is used to update the list of current
// fallback addresses for locating mesh gateways in the primary datacenter.
func (a *Agent) RefreshPrimaryGatewayFallbackAddresses(addrs []string) error {
	if srv, ok := a.delegate.(*consul.Server); ok {
		srv.RefreshPrimaryGatewayFallbackAddresses(addrs)
		return nil
	}
	return fmt.Errorf("Must be a server to track mesh gateways in the primary datacenter")
}

// ForceLeave is used to remove a failed node from the cluster
func (a *Agent) ForceLeave(node string, prune bool, entMeta *acl.EnterpriseMeta) error {
	a.logger.Info("Force leaving node", "node", node)

	err := a.delegate.RemoveFailedNode(node, prune, entMeta)
	if err != nil {
		a.logger.Warn("Failed to remove node",
			"node", node,
			"error", err,
		)
	}
	return err
}

// ForceLeaveWAN is used to remove a failed node from the WAN cluster
func (a *Agent) ForceLeaveWAN(node string, prune bool, entMeta *acl.EnterpriseMeta) error {
	a.logger.Info("(WAN) Force leaving node", "node", node)

	srv, ok := a.delegate.(*consul.Server)
	if !ok {
		return fmt.Errorf("Must be a server to force-leave a node from the WAN cluster")
	}

	err := srv.RemoveFailedNodeWAN(node, prune, entMeta)
	if err != nil {
		a.logger.Warn("(WAN) Failed to remove node",
			"node", node,
			"error", err,
		)
	}
	return err
}

// AgentLocalMember is used to retrieve the LAN member for the local node.
func (a *Agent) AgentLocalMember() serf.Member {
	return a.delegate.AgentLocalMember()
}

// LANMembersInAgentPartition is used to retrieve the LAN members for this
// agent's partition.
func (a *Agent) LANMembersInAgentPartition() []serf.Member {
	return a.delegate.LANMembersInAgentPartition()
}

// LANMembers returns the LAN members for one of:
//
// - the requested partition
// - the requested segment
// - all segments
//
// This is limited to segments and partitions that the node is a member of.
func (a *Agent) LANMembers(f consul.LANMemberFilter) ([]serf.Member, error) {
	return a.delegate.LANMembers(f)
}

// WANMembers is used to retrieve the WAN members
func (a *Agent) WANMembers() []serf.Member {
	if srv, ok := a.delegate.(*consul.Server); ok {
		return srv.WANMembers()
	}
	return nil
}

// StartSync is called once Services and Checks are registered.
// This is called to prevent a race between clients and the anti-entropy routines
func (a *Agent) StartSync() {
	go a.sync.Run()
	a.logger.Info("started state syncer")
}

// PauseSync is used to pause anti-entropy while bulk changes are made. It also
// sets state that agent-local watches use to "ride out" config reloads and bulk
// updates which might spuriously unload state and reload it again.
func (a *Agent) PauseSync() {
	// Do this outside of lock as it has it's own locking
	a.sync.Pause()

	// Coordinate local state watchers
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncCh == nil {
		a.syncCh = make(chan struct{})
	}
}

// ResumeSync is used to unpause anti-entropy after bulk changes are make
func (a *Agent) ResumeSync() {
	// a.sync maintains a stack/ref count of Pause calls since we call
	// Pause/Resume in nested way during a reload and AddService. We only want to
	// trigger local state watchers if this Resume call actually started sync back
	// up again (i.e. was the last resume on the stack). We could check that
	// separately with a.sync.Paused but that is racey since another Pause call
	// might be made between our Resume and checking Paused.
	resumed := a.sync.Resume()

	if !resumed {
		// Return early so we don't notify local watchers until we are actually
		// resumed.
		return
	}

	// Coordinate local state watchers
	a.syncMu.Lock()
	defer a.syncMu.Unlock()

	if a.syncCh != nil {
		close(a.syncCh)
		a.syncCh = nil
	}
}

// SyncPausedCh returns either a channel or nil. If nil sync is not paused. If
// non-nil, the channel will be closed when sync resumes.
func (a *Agent) SyncPausedCh() <-chan struct{} {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	return a.syncCh
}

// GetLANCoordinate returns the coordinates of this node in the local pools
// (assumes coordinates are enabled, so check that before calling).
func (a *Agent) GetLANCoordinate() (lib.CoordinateSet, error) {
	return a.delegate.GetLANCoordinate()
}

// sendCoordinate is a long-running loop that periodically sends our coordinate
// to the server. Closing the agent's shutdownChannel will cause this to exit.
func (a *Agent) sendCoordinate() {
OUTER:
	for {
		rate := a.config.SyncCoordinateRateTarget
		min := a.config.SyncCoordinateIntervalMin
		intv := lib.RateScaledInterval(rate, min, len(a.LANMembersInAgentPartition()))
		intv = intv + lib.RandomStagger(intv)

		select {
		case <-time.After(intv):
			members := a.LANMembersInAgentPartition()
			grok, err := consul.CanServersUnderstandProtocol(members, 3)
			if err != nil {
				a.logger.Error("Failed to check servers", "error", err)
				continue
			}
			if !grok {
				a.logger.Debug("Skipping coordinate updates until servers are upgraded")
				continue
			}

			cs, err := a.GetLANCoordinate()
			if err != nil {
				a.logger.Error("Failed to get coordinate", "error", err)
				continue
			}

			for segment, coord := range cs {
				agentToken := a.tokens.AgentToken()
				req := structs.CoordinateUpdateRequest{
					Datacenter:     a.config.Datacenter,
					Node:           a.config.NodeName,
					Segment:        segment,
					Coord:          coord,
					EnterpriseMeta: *a.AgentEnterpriseMeta(),
					WriteRequest:   structs.WriteRequest{Token: agentToken},
				}
				var reply struct{}
				// todo(kit) port all of these logger calls to hclog w/ loglevel configuration
				// todo(kit) handle acl.ErrNotFound cases here in the future
				if err := a.RPC("Coordinate.Update", &req, &reply); err != nil {
					if acl.IsErrPermissionDenied(err) {
						accessorID := a.aclAccessorID(agentToken)
						a.logger.Warn("Coordinate update blocked by ACLs", "accessorID", accessorID)
					} else {
						a.logger.Error("Coordinate update error", "error", err)
					}
					continue OUTER
				}
			}
		case <-a.shutdownCh:
			return
		}
	}
}

// reapServicesInternal does a single pass, looking for services to reap.
func (a *Agent) reapServicesInternal() {
	reaped := make(map[structs.ServiceID]bool)
	for checkID, cs := range a.State.AllCriticalCheckStates() {
		serviceID := cs.Check.CompoundServiceID()

		// There's nothing to do if there's no service.
		if serviceID.ID == "" {
			continue
		}

		// There might be multiple checks for one service, so
		// we don't need to reap multiple times.
		if reaped[serviceID] {
			continue
		}

		// See if there's a timeout.
		// todo(fs): this looks fishy... why is there another data structure in the agent with its own lock?
		a.stateLock.Lock()
		timeout := a.checkReapAfter[checkID]
		a.stateLock.Unlock()

		// Reap, if necessary. We keep track of which service
		// this is so that we won't try to remove it again.
		if timeout > 0 && cs.CriticalFor() > timeout {
			reaped[serviceID] = true
			if err := a.RemoveService(serviceID); err != nil {
				a.logger.Error("failed to deregister service with critical health that exceeded health check's 'deregister_critical_service_after' timeout",
					"service", serviceID.String(),
					"check", checkID.String(),
					"timeout", timeout.String(),
					"error", err,
				)
			} else {
				a.logger.Info("deregistered service with critical health due to exceeding health check's 'deregister_critical_service_after' timeout",
					"service", serviceID.String(),
					"check", checkID.String(),
					"timeout", timeout.String(),
				)
			}
		}
	}
}

// reapServices is a long running goroutine that looks for checks that have been
// critical too long and deregisters their associated services.
func (a *Agent) reapServices() {
	for {
		select {
		case <-time.After(a.config.CheckReapInterval):
			a.reapServicesInternal()

		case <-a.shutdownCh:
			return
		}
	}

}

// persistedService is used to wrap a service definition and bundle it
// with an ACL token so we can restore both at a later agent start.
type persistedService struct {
	Token   string
	Service *structs.NodeService
	Source  string
	// whether this service was registered as a sidecar, see structs.NodeService
	// we store this field here because it is excluded from json serialization
	// to exclude it from API output, but we need it to properly deregister
	// persisted sidecars.
	LocallyRegisteredAsSidecar bool `json:",omitempty"`
}

func (a *Agent) makeServiceFilePath(svcID structs.ServiceID) string {
	return filepath.Join(a.config.DataDir, servicesDir, svcID.StringHashSHA256())
}

// persistService saves a service definition to a JSON file in the data dir
func (a *Agent) persistService(service *structs.NodeService, source configSource) error {
	svcID := service.CompoundServiceID()
	svcPath := a.makeServiceFilePath(svcID)

	wrapped := persistedService{
		Token:                      a.State.ServiceToken(service.CompoundServiceID()),
		Service:                    service,
		Source:                     source.String(),
		LocallyRegisteredAsSidecar: service.LocallyRegisteredAsSidecar,
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	return file.WriteAtomic(svcPath, encoded)
}

// purgeService removes a persisted service definition file from the data dir
func (a *Agent) purgeService(serviceID structs.ServiceID) error {
	svcPath := a.makeServiceFilePath(serviceID)
	if _, err := os.Stat(svcPath); err == nil {
		return os.Remove(svcPath)
	}
	return nil
}

// persistCheck saves a check definition to the local agent's state directory
func (a *Agent) persistCheck(check *structs.HealthCheck, chkType *structs.CheckType, source configSource) error {
	cid := check.CompoundCheckID()
	checkPath := filepath.Join(a.config.DataDir, checksDir, cid.StringHashSHA256())

	// Create the persisted check
	wrapped := persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   a.State.CheckToken(check.CompoundCheckID()),
		Source:  source.String(),
	}

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	return file.WriteAtomic(checkPath, encoded)
}

// purgeCheck removes a persisted check definition file from the data dir
func (a *Agent) purgeCheck(checkID structs.CheckID) error {
	checkPath := filepath.Join(a.config.DataDir, checksDir, checkID.StringHashSHA256())
	if _, err := os.Stat(checkPath); err == nil {
		return os.Remove(checkPath)
	}
	return nil
}

// persistedServiceConfig is used to serialize the resolved service config that
// feeds into the ServiceManager at registration time so that it may be
// restored later on.
type persistedServiceConfig struct {
	ServiceID string
	Defaults  *structs.ServiceConfigResponse
	acl.EnterpriseMeta
}

func (a *Agent) makeServiceConfigFilePath(serviceID structs.ServiceID) string {
	return filepath.Join(a.config.DataDir, serviceConfigDir, serviceID.StringHashSHA256())
}

func (a *Agent) persistServiceConfig(serviceID structs.ServiceID, defaults *structs.ServiceConfigResponse) error {
	// Create the persisted config.
	wrapped := persistedServiceConfig{
		ServiceID:      serviceID.ID,
		Defaults:       defaults,
		EnterpriseMeta: serviceID.EnterpriseMeta,
	}

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	dir := filepath.Join(a.config.DataDir, serviceConfigDir)
	configPath := a.makeServiceConfigFilePath(serviceID)

	// Create the config dir if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating service configs dir %q: %s", dir, err)
	}

	return file.WriteAtomic(configPath, encoded)
}

func (a *Agent) purgeServiceConfig(serviceID structs.ServiceID) error {
	configPath := a.makeServiceConfigFilePath(serviceID)
	if _, err := os.Stat(configPath); err == nil {
		return os.Remove(configPath)
	}
	return nil
}

func (a *Agent) readPersistedServiceConfigs() (map[structs.ServiceID]*structs.ServiceConfigResponse, error) {
	out := make(map[structs.ServiceID]*structs.ServiceConfigResponse)

	configDir := filepath.Join(a.config.DataDir, serviceConfigDir)
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Failed reading service configs dir %q: %s", configDir, err)
	}

	for _, fi := range files {
		// Skip all dirs
		if fi.IsDir() {
			continue
		}

		// Skip all partially written temporary files
		if strings.HasSuffix(fi.Name(), "tmp") {
			a.logger.Warn("Ignoring temporary service config file", "file", fi.Name())
			continue
		}

		// Read the contents into a buffer
		file := filepath.Join(configDir, fi.Name())
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed reading service config file %q: %w", file, err)
		}

		// Try decoding the service config definition
		var p persistedServiceConfig
		if err := json.Unmarshal(buf, &p); err != nil {
			a.logger.Error("Failed decoding service config file",
				"file", file,
				"error", err,
			)
			continue
		}

		serviceID := structs.NewServiceID(p.ServiceID, &p.EnterpriseMeta)

		// Rename files that used the old md5 hash to the new sha256 name; only needed when upgrading from 1.10 and before.
		newPath := a.makeServiceConfigFilePath(serviceID)
		if file != newPath {
			if err := os.Rename(file, newPath); err != nil {
				a.logger.Error("Failed renaming service config file",
					"file", file,
					"targetFile", newPath,
					"error", err,
				)
			}
		}

		if acl.EqualPartitions("", p.PartitionOrEmpty()) {
			p.OverridePartition(a.AgentEnterpriseMeta().PartitionOrDefault())
		} else if !acl.EqualPartitions(a.AgentEnterpriseMeta().PartitionOrDefault(), p.PartitionOrDefault()) {
			a.logger.Info("Purging service config file in wrong partition",
				"file", file,
				"partition", p.PartitionOrDefault(),
			)
			if err := os.Remove(file); err != nil {
				a.logger.Error("Failed purging service config file",
					"file", file,
					"error", err,
				)
			}
			continue
		}

		out[serviceID] = p.Defaults
	}

	return out, nil
}

// AddService is used to add a service entry and its check. Any check for this service missing from chkTypes will be deleted.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddService(req AddServiceRequest) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	rl := addServiceLockedRequest{
		AddServiceRequest:    req,
		serviceDefaults:      serviceDefaultsFromCache(a.baseDeps, req),
		persistServiceConfig: true,
	}
	return a.addServiceLocked(rl)
}

// addServiceLocked adds a service entry to the service manager if enabled, or directly
// to the local state if it is not. This function assumes the state lock is already held.
func (a *Agent) addServiceLocked(req addServiceLockedRequest) error {
	// Must auto-assign the port and default checks (if needed) here to avoid race collisions.
	if req.Service.LocallyRegisteredAsSidecar {
		if req.Service.Port < 1 {
			port, err := a.sidecarPortFromServiceIDLocked(req.Service.CompoundServiceID())
			if err != nil {
				return err
			}
			req.Service.Port = port
		}
		// Setup default check if none given.
		if len(req.chkTypes) < 1 {
			req.chkTypes = sidecarDefaultChecks(req.Service.ID, req.Service.Address, req.Service.Proxy.LocalServiceAddress, req.Service.Port)
		}
	}

	req.Service.EnterpriseMeta.Normalize()

	if err := a.validateService(req.Service, req.chkTypes); err != nil {
		return err
	}

	if a.config.EnableCentralServiceConfig && (req.Service.IsSidecarProxy() || req.Service.IsGateway()) {
		return a.serviceManager.AddService(req)
	}

	req.persistServiceConfig = false
	return a.addServiceInternal(addServiceInternalRequest{addServiceLockedRequest: req})
}

type addServiceLockedRequest struct {
	AddServiceRequest

	persistServiceConfig bool

	// serviceDefaults is a function which will return centralized service
	// configuration.
	// When loading service definitions from disk this will return a copy
	// loaded from a persisted file. Otherwise it will query a Server for the
	// centralized config.
	// serviceDefaults is called when the Agent.stateLock is held, so it must
	// never attempt to acquire that lock.
	serviceDefaults func(context.Context) (*structs.ServiceConfigResponse, error)

	// checkStateSnapshot may optionally be set to a snapshot of the checks in
	// the local.State. If checkStateSnapshot is nil, addServiceInternal will
	// callState.Checks to get the snapshot.
	checkStateSnapshot map[structs.CheckID]*structs.HealthCheck
}

// AddServiceRequest contains the fields used to register a service on the local
// agent using Agent.AddService.
type AddServiceRequest struct {
	Service               *structs.NodeService
	chkTypes              []*structs.CheckType
	persist               bool
	token                 string
	replaceExistingChecks bool
	Source                configSource
}

type addServiceInternalRequest struct {
	addServiceLockedRequest

	// persistService may be set to a NodeService definition to indicate to
	// addServiceInternal that if persist=true, it should persist this definition
	// of the service, not the one from the Service field. This is necessary so
	// that the service is persisted without the serviceDefaults.
	persistService *structs.NodeService

	// persistServiceDefaults may be set to a ServiceConfigResponse to indicate to
	// addServiceInternal that it should persist the value in a file.
	persistServiceDefaults *structs.ServiceConfigResponse
}

// addServiceInternal adds the given service and checks to the local state.
func (a *Agent) addServiceInternal(req addServiceInternalRequest) error {
	service := req.Service

	// Pause the service syncs during modification
	a.PauseSync()
	defer a.ResumeSync()

	// Set default tagged addresses
	serviceIP := net.ParseIP(service.Address)
	serviceAddressIs4 := serviceIP != nil && serviceIP.To4() != nil
	serviceAddressIs6 := serviceIP != nil && serviceIP.To4() == nil
	if service.TaggedAddresses == nil {
		service.TaggedAddresses = map[string]structs.ServiceAddress{}
	}
	if _, ok := service.TaggedAddresses[structs.TaggedAddressLANIPv4]; !ok && serviceAddressIs4 {
		service.TaggedAddresses[structs.TaggedAddressLANIPv4] = structs.ServiceAddress{Address: service.Address, Port: service.Port}
	}
	if _, ok := service.TaggedAddresses[structs.TaggedAddressWANIPv4]; !ok && serviceAddressIs4 {
		service.TaggedAddresses[structs.TaggedAddressWANIPv4] = structs.ServiceAddress{Address: service.Address, Port: service.Port}
	}
	if _, ok := service.TaggedAddresses[structs.TaggedAddressLANIPv6]; !ok && serviceAddressIs6 {
		service.TaggedAddresses[structs.TaggedAddressLANIPv6] = structs.ServiceAddress{Address: service.Address, Port: service.Port}
	}
	if _, ok := service.TaggedAddresses[structs.TaggedAddressWANIPv6]; !ok && serviceAddressIs6 {
		service.TaggedAddresses[structs.TaggedAddressWANIPv6] = structs.ServiceAddress{Address: service.Address, Port: service.Port}
	}

	var checks []*structs.HealthCheck

	// all the checks must be associated with the same enterprise meta of the service
	// so this map can just use the main CheckID for indexing
	existingChecks := map[structs.CheckID]bool{}
	for _, check := range a.State.ChecksForService(service.CompoundServiceID(), false) {
		existingChecks[check.CompoundCheckID()] = false
	}

	// Note, this is explicitly a nil check instead of len() == 0 because
	// Agent.Start does not have a snapshot, and we don't want to query
	// State.Checks each time.
	if req.checkStateSnapshot == nil {
		req.checkStateSnapshot = a.State.AllChecks()
	}

	// Create an associated health check
	for i, chkType := range req.chkTypes {
		checkID := string(chkType.CheckID)
		if checkID == "" {
			checkID = fmt.Sprintf("service:%s", service.ID)
			if len(req.chkTypes) > 1 {
				checkID += fmt.Sprintf(":%d", i+1)
			}
		}

		cid := structs.NewCheckID(types.CheckID(checkID), &service.EnterpriseMeta)
		existingChecks[cid] = true

		name := chkType.Name
		if name == "" {
			name = fmt.Sprintf("Service '%s' check", service.Service)
		}

		var intervalStr string
		var timeoutStr string
		if chkType.Interval != 0 {
			intervalStr = chkType.Interval.String()
		}
		if chkType.Timeout != 0 {
			timeoutStr = chkType.Timeout.String()
		}

		check := &structs.HealthCheck{
			Node:           a.config.NodeName,
			CheckID:        types.CheckID(checkID),
			Name:           name,
			Interval:       intervalStr,
			Timeout:        timeoutStr,
			Status:         api.HealthCritical,
			Notes:          chkType.Notes,
			ServiceID:      service.ID,
			ServiceName:    service.Service,
			ServiceTags:    service.Tags,
			Type:           chkType.Type(),
			EnterpriseMeta: service.EnterpriseMeta,
		}
		if chkType.Status != "" {
			check.Status = chkType.Status
		}

		// Restore the fields from the snapshot.
		prev, ok := req.checkStateSnapshot[cid]
		if ok {
			check.Output = prev.Output
			check.Status = prev.Status
		}

		checks = append(checks, check)
	}

	// cleanup, store the ids of services and checks that weren't previously
	// registered so we clean them up if something fails halfway through the
	// process.
	var cleanupServices []structs.ServiceID
	var cleanupChecks []structs.CheckID

	sid := service.CompoundServiceID()
	if s := a.State.Service(sid); s == nil {
		cleanupServices = append(cleanupServices, sid)
	}

	for _, check := range checks {
		cid := check.CompoundCheckID()
		if c := a.State.Check(cid); c == nil {
			cleanupChecks = append(cleanupChecks, cid)
		}
	}

	err := a.State.AddServiceWithChecks(service, checks, req.token)
	if err != nil {
		a.cleanupRegistration(cleanupServices, cleanupChecks)
		return err
	}

	source := req.Source
	persist := req.persist
	for i := range checks {
		if err := a.addCheck(checks[i], req.chkTypes[i], service, req.token, source); err != nil {
			a.cleanupRegistration(cleanupServices, cleanupChecks)
			return err
		}

		if persist && a.config.DataDir != "" {
			if err := a.persistCheck(checks[i], req.chkTypes[i], source); err != nil {
				a.cleanupRegistration(cleanupServices, cleanupChecks)
				return err

			}
		}
	}

	// If a proxy service wishes to expose checks, check targets need to be rerouted to the proxy listener
	// This needs to be called after chkTypes are added to the agent, to avoid being overwritten
	psid := structs.NewServiceID(service.Proxy.DestinationServiceID, &service.EnterpriseMeta)

	if service.Proxy.Expose.Checks {
		err := a.rerouteExposedChecks(psid, service.Address)
		if err != nil {
			a.logger.Warn("failed to reroute L7 checks to exposed proxy listener")
		}
	} else {
		// Reset check targets if proxy was re-registered but no longer wants to expose checks
		// If the proxy is being registered for the first time then this is a no-op
		a.resetExposedChecks(psid)
	}

	if req.persistServiceConfig && a.config.DataDir != "" {
		var err error
		if req.persistServiceDefaults != nil {
			err = a.persistServiceConfig(service.CompoundServiceID(), req.persistServiceDefaults)
		} else {
			err = a.purgeServiceConfig(service.CompoundServiceID())
		}

		if err != nil {
			a.cleanupRegistration(cleanupServices, cleanupChecks)
			return err
		}
	}

	// Persist the service to a file
	if persist && a.config.DataDir != "" {
		if req.persistService == nil {
			req.persistService = service
		}

		if err := a.persistService(req.persistService, source); err != nil {
			a.cleanupRegistration(cleanupServices, cleanupChecks)
			return err
		}
	}

	if req.replaceExistingChecks {
		for checkID, keep := range existingChecks {
			if !keep {
				a.removeCheckLocked(checkID, persist)
			}
		}
	}

	return nil
}

// validateService validates an service and its checks, either returning an error or emitting a
// warning based on the nature of the error.
func (a *Agent) validateService(service *structs.NodeService, chkTypes []*structs.CheckType) error {
	if service.Service == "" {
		return fmt.Errorf("Service name missing")
	}
	if service.ID == "" && service.Service != "" {
		service.ID = service.Service
	}
	for _, check := range chkTypes {
		if err := check.Validate(); err != nil {
			return fmt.Errorf("Check is not valid: %v", err)
		}
	}

	// Set default weights if not specified. This is important as it ensures AE
	// doesn't consider the service different since it has nil weights.
	if service.Weights == nil {
		service.Weights = &structs.Weights{Passing: 1, Warning: 1}
	}

	// Warn if the service name is incompatible with DNS
	if dns.InvalidNameRe.MatchString(service.Service) {
		a.logger.Warn("Service name will not be discoverable "+
			"via DNS due to invalid characters. Valid characters include "+
			"all alpha-numerics and dashes.",
			"service", service.Service,
		)
	} else if len(service.Service) > dns.MaxLabelLength {
		a.logger.Warn("Service name will not be discoverable "+
			"via DNS due to it being too long. Valid lengths are between "+
			"1 and 63 bytes.",
			"service", service.Service,
		)
	}

	// Warn if any tags are incompatible with DNS
	for _, tag := range service.Tags {
		if dns.InvalidNameRe.MatchString(tag) {
			a.logger.Debug("Service tag will not be discoverable "+
				"via DNS due to invalid characters. Valid characters include "+
				"all alpha-numerics and dashes.",
				"tag", tag,
			)
		} else if len(tag) > dns.MaxLabelLength {
			a.logger.Debug("Service tag will not be discoverable "+
				"via DNS due to it being too long. Valid lengths are between "+
				"1 and 63 bytes.",
				"tag", tag,
			)
		}
	}

	// Check IPv4/IPv6 tagged addresses
	if service.TaggedAddresses != nil {
		if sa, ok := service.TaggedAddresses[structs.TaggedAddressLANIPv4]; ok {
			ip := net.ParseIP(sa.Address)
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("Service tagged address %q must be a valid ipv4 address", structs.TaggedAddressLANIPv4)
			}
		}
		if sa, ok := service.TaggedAddresses[structs.TaggedAddressWANIPv4]; ok {
			ip := net.ParseIP(sa.Address)
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("Service tagged address %q must be a valid ipv4 address", structs.TaggedAddressWANIPv4)
			}
		}
		if sa, ok := service.TaggedAddresses[structs.TaggedAddressLANIPv6]; ok {
			ip := net.ParseIP(sa.Address)
			if ip == nil || ip.To4() != nil {
				return fmt.Errorf("Service tagged address %q must be a valid ipv6 address", structs.TaggedAddressLANIPv6)
			}
		}
		if sa, ok := service.TaggedAddresses[structs.TaggedAddressLANIPv6]; ok {
			ip := net.ParseIP(sa.Address)
			if ip == nil || ip.To4() != nil {
				return fmt.Errorf("Service tagged address %q must be a valid ipv6 address", structs.TaggedAddressLANIPv6)
			}
		}
	}

	return nil
}

// cleanupRegistration is called on  registration error to ensure no there are no
// leftovers after a partial failure
func (a *Agent) cleanupRegistration(serviceIDs []structs.ServiceID, checksIDs []structs.CheckID) {
	for _, s := range serviceIDs {
		if err := a.State.RemoveService(s); err != nil {
			a.logger.Error("failed to remove service during cleanup",
				"service", s.String(),
				"error", err,
			)
		}
		if err := a.purgeService(s); err != nil {
			a.logger.Error("failed to purge service file during cleanup",
				"service", s.String(),
				"error", err,
			)
		}
		if err := a.purgeServiceConfig(s); err != nil {
			a.logger.Error("failed to purge service config file during cleanup",
				"service", s,
				"error", err,
			)
		}
		if err := a.removeServiceSidecars(s, true); err != nil {
			a.logger.Error("service registration: cleanup: failed remove sidecars for", "service", s, "error", err)
		}
	}

	for _, c := range checksIDs {
		a.cancelCheckMonitors(c)
		if err := a.State.RemoveCheck(c); err != nil {
			a.logger.Error("failed to remove check during cleanup",
				"check", c.String(),
				"error", err,
			)
		}
		if err := a.purgeCheck(c); err != nil {
			a.logger.Error("failed to purge check file during cleanup",
				"check", c.String(),
				"error", err,
			)
		}
	}
}

// RemoveService is used to remove a service entry.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveService(serviceID structs.ServiceID) error {
	return a.removeService(serviceID, true)
}

func (a *Agent) removeService(serviceID structs.ServiceID, persist bool) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.removeServiceLocked(serviceID, persist)
}

// removeServiceLocked is used to remove a service entry.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) removeServiceLocked(serviceID structs.ServiceID, persist bool) error {
	// Validate ServiceID
	if serviceID.ID == "" {
		return fmt.Errorf("ServiceID missing")
	}

	// Shut down the config watch in the service manager if enabled.
	if a.config.EnableCentralServiceConfig {
		a.serviceManager.RemoveService(serviceID)
	}

	// Reset the HTTP check targets if they were exposed through a proxy
	// If this is not a proxy or checks were not exposed then this is a no-op
	svc := a.State.Service(serviceID)

	if svc != nil {
		psid := structs.NewServiceID(svc.Proxy.DestinationServiceID, &svc.EnterpriseMeta)
		a.resetExposedChecks(psid)
	}

	checks := a.State.ChecksForService(serviceID, false)
	var checkIDs []structs.CheckID
	for id := range checks {
		checkIDs = append(checkIDs, id)
	}

	// Remove service immediately
	if err := a.State.RemoveServiceWithChecks(serviceID, checkIDs); err != nil {
		a.logger.Warn("Failed to deregister service",
			"service", serviceID.String(),
			"error", err,
		)
		return nil
	}

	// Remove the service from the data dir
	if persist {
		if err := a.purgeService(serviceID); err != nil {
			return err
		}
		if err := a.purgeServiceConfig(serviceID); err != nil {
			return err
		}
	}

	// Deregister any associated health checks
	for checkID := range checks {
		if err := a.removeCheckLocked(checkID, persist); err != nil {
			return err
		}
	}

	a.logger.Debug("removed service", "service", serviceID.String())

	// If any Sidecar services exist for the removed service ID, remove them too.
	return a.removeServiceSidecars(serviceID, persist)
}

func (a *Agent) removeServiceSidecars(serviceID structs.ServiceID, persist bool) error {
	sidecarSID := structs.NewServiceID(sidecarIDFromServiceID(serviceID.ID), &serviceID.EnterpriseMeta)
	if sidecar := a.State.Service(sidecarSID); sidecar != nil {
		// Double check that it's not just an ID collision and we actually added
		// this from a sidecar.
		if sidecar.LocallyRegisteredAsSidecar {
			// Remove it!
			err := a.removeServiceLocked(sidecarSID, persist)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// AddCheck is used to add a health check to the agent.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered. The Check may include a CheckType which
// is used to automatically update the check status
func (a *Agent) AddCheck(check *structs.HealthCheck, chkType *structs.CheckType, persist bool, token string, source configSource) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.addCheckLocked(check, chkType, persist, token, source)
}

func (a *Agent) addCheckLocked(check *structs.HealthCheck, chkType *structs.CheckType, persist bool, token string, source configSource) error {
	var service *structs.NodeService

	check.EnterpriseMeta.Normalize()

	if check.ServiceID != "" {
		cid := check.CompoundServiceID()
		service = a.State.Service(cid)
		if service == nil {
			return fmt.Errorf("ServiceID %q does not exist", cid.String())
		}
	}

	// Extra validations
	if err := check.Validate(); err != nil {
		return err
	}

	// snapshot the current state of the health check to avoid potential flapping
	cid := check.CompoundCheckID()
	existing := a.State.Check(cid)
	defer func() {
		if existing != nil {
			a.State.UpdateCheck(cid, existing.Status, existing.Output)
		}
	}()

	err := a.addCheck(check, chkType, service, token, source)
	if err != nil {
		a.State.RemoveCheck(cid)
		return err
	}

	// Add to the local state for anti-entropy
	err = a.State.AddCheck(check, token)
	if err != nil {
		return err
	}

	// Persist the check
	if persist && a.config.DataDir != "" {
		return a.persistCheck(check, chkType, source)
	}

	return nil
}

func (a *Agent) addCheck(check *structs.HealthCheck, chkType *structs.CheckType, service *structs.NodeService, token string, source configSource) error {
	if check.CheckID == "" {
		return fmt.Errorf("CheckID missing")
	}

	if chkType != nil {
		if err := chkType.Validate(); err != nil {
			return fmt.Errorf("Check is not valid: %v", err)
		}

		if chkType.IsScript() {
			if source == ConfigSourceLocal && !a.config.EnableLocalScriptChecks {
				return fmt.Errorf("Scripts are disabled on this agent; to enable, configure 'enable_script_checks' or 'enable_local_script_checks' to true")
			}

			if source == ConfigSourceRemote && !a.config.EnableRemoteScriptChecks {
				return fmt.Errorf("Scripts are disabled on this agent from remote calls; to enable, configure 'enable_script_checks' to true")
			}
		}
	}

	if check.ServiceID != "" {
		check.ServiceName = service.Service
		check.ServiceTags = service.Tags
		check.EnterpriseMeta = service.EnterpriseMeta
	}

	// Check if already registered
	if chkType != nil {
		maxOutputSize := a.config.CheckOutputMaxSize
		if maxOutputSize == 0 {
			maxOutputSize = checks.DefaultBufSize
		}
		if chkType.OutputMaxSize > 0 && maxOutputSize > chkType.OutputMaxSize {
			maxOutputSize = chkType.OutputMaxSize
		}

		// FailuresBeforeWarning has to default to same value as FailuresBeforeCritical
		if chkType.FailuresBeforeWarning == 0 {
			chkType.FailuresBeforeWarning = chkType.FailuresBeforeCritical
		}

		// Get the address of the proxy for this service if it exists
		// Need its config to know whether we should reroute checks to it
		var proxy *structs.NodeService
		if service != nil {
			// NOTE: Both services must live in the same namespace and
			// partition so this will correctly scope the results.
			for _, svc := range a.State.Services(&service.EnterpriseMeta) {
				if svc.Proxy.DestinationServiceID == service.ID {
					proxy = svc
					break
				}
			}
		}

		statusHandler := checks.NewStatusHandler(a.State, a.logger, chkType.SuccessBeforePassing, chkType.FailuresBeforeWarning, chkType.FailuresBeforeCritical)
		sid := check.CompoundServiceID()

		cid := check.CompoundCheckID()

		switch {

		case chkType.IsTTL():
			if existing, ok := a.checkTTLs[cid]; ok {
				existing.Stop()
				delete(a.checkTTLs, cid)
			}

			ttl := &checks.CheckTTL{
				Notify:        a.State,
				CheckID:       cid,
				ServiceID:     sid,
				TTL:           chkType.TTL,
				Logger:        a.logger,
				OutputMaxSize: maxOutputSize,
			}

			// Restore persisted state, if any
			if err := a.loadCheckState(check); err != nil {
				a.logger.Warn("failed restoring state for check",
					"check", cid.String(),
					"error", err,
				)
			}

			ttl.Start()
			a.checkTTLs[cid] = ttl

		case chkType.IsHTTP():
			if existing, ok := a.checkHTTPs[cid]; ok {
				existing.Stop()
				delete(a.checkHTTPs, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}

			tlsClientConfig := a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify, chkType.TLSServerName)

			http := &checks.CheckHTTP{
				CheckID:          cid,
				ServiceID:        sid,
				HTTP:             chkType.HTTP,
				Header:           chkType.Header,
				Method:           chkType.Method,
				Body:             chkType.Body,
				DisableRedirects: chkType.DisableRedirects,
				Interval:         chkType.Interval,
				Timeout:          chkType.Timeout,
				Logger:           a.logger,
				OutputMaxSize:    maxOutputSize,
				TLSClientConfig:  tlsClientConfig,
				StatusHandler:    statusHandler,
			}

			if proxy != nil && proxy.Proxy.Expose.Checks {
				port, err := a.listenerPortLocked(sid, cid)
				if err != nil {
					a.logger.Error("error exposing check",
						"check", cid.String(),
						"error", err,
					)
					return err
				}
				http.ProxyHTTP = httpInjectAddr(http.HTTP, proxy.Address, port)
				check.ExposedPort = port
			}

			http.Start()
			a.checkHTTPs[cid] = http

		case chkType.IsTCP():
			if existing, ok := a.checkTCPs[cid]; ok {
				existing.Stop()
				delete(a.checkTCPs, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}

			tcp := &checks.CheckTCP{
				CheckID:       cid,
				ServiceID:     sid,
				TCP:           chkType.TCP,
				Interval:      chkType.Interval,
				Timeout:       chkType.Timeout,
				Logger:        a.logger,
				StatusHandler: statusHandler,
			}
			tcp.Start()
			a.checkTCPs[cid] = tcp

		case chkType.IsUDP():
			if existing, ok := a.checkUDPs[cid]; ok {
				existing.Stop()
				delete(a.checkUDPs, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}

			udp := &checks.CheckUDP{
				CheckID:       cid,
				ServiceID:     sid,
				UDP:           chkType.UDP,
				Interval:      chkType.Interval,
				Timeout:       chkType.Timeout,
				Logger:        a.logger,
				StatusHandler: statusHandler,
			}
			udp.Start()
			a.checkUDPs[cid] = udp

		case chkType.IsGRPC():
			if existing, ok := a.checkGRPCs[cid]; ok {
				existing.Stop()
				delete(a.checkGRPCs, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}

			var tlsClientConfig *tls.Config
			if chkType.GRPCUseTLS {
				tlsClientConfig = a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify, chkType.TLSServerName)
			}

			grpc := &checks.CheckGRPC{
				CheckID:         cid,
				ServiceID:       sid,
				GRPC:            chkType.GRPC,
				Interval:        chkType.Interval,
				Timeout:         chkType.Timeout,
				Logger:          a.logger,
				TLSClientConfig: tlsClientConfig,
				StatusHandler:   statusHandler,
			}

			if proxy != nil && proxy.Proxy.Expose.Checks {
				port, err := a.listenerPortLocked(sid, cid)
				if err != nil {
					a.logger.Error("error exposing check",
						"check", cid.String(),
						"error", err,
					)
					return err
				}
				grpc.ProxyGRPC = grpcInjectAddr(grpc.GRPC, proxy.Address, port)
				check.ExposedPort = port
			}

			grpc.Start()
			a.checkGRPCs[cid] = grpc

		case chkType.IsDocker():
			if existing, ok := a.checkDockers[cid]; ok {
				existing.Stop()
				delete(a.checkDockers, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}

			if a.dockerClient == nil {
				dc, err := checks.NewDockerClient(os.Getenv("DOCKER_HOST"), int64(maxOutputSize))
				if err != nil {
					a.logger.Error("error creating docker client", "error", err)
					return err
				}
				a.logger.Debug("created docker client", "host", dc.Host())
				a.dockerClient = dc
			}

			dockerCheck := &checks.CheckDocker{
				CheckID:           cid,
				ServiceID:         sid,
				DockerContainerID: chkType.DockerContainerID,
				Shell:             chkType.Shell,
				ScriptArgs:        chkType.ScriptArgs,
				Interval:          chkType.Interval,
				Logger:            a.logger,
				Client:            a.dockerClient,
				StatusHandler:     statusHandler,
			}
			dockerCheck.Start()
			a.checkDockers[cid] = dockerCheck

		case chkType.IsOSService():
			if existing, ok := a.checkOSServices[cid]; ok {
				existing.Stop()
				delete(a.checkOSServices, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}

			if a.osServiceClient == nil {
				ossp, err := checks.NewOSServiceClient()
				if err != nil {
					a.logger.Error("error creating OS Service client", "error", err)
					return err
				}
				a.logger.Debug("created OS Service client")
				a.osServiceClient = ossp
			}

			osServiceCheck := &checks.CheckOSService{
				CheckID:       cid,
				ServiceID:     sid,
				OSService:     chkType.OSService,
				Timeout:       chkType.Timeout,
				Interval:      chkType.Interval,
				Logger:        a.logger,
				Client:        a.osServiceClient,
				StatusHandler: statusHandler,
			}
			osServiceCheck.Start()
			a.checkOSServices[cid] = osServiceCheck

		case chkType.IsMonitor():
			if existing, ok := a.checkMonitors[cid]; ok {
				existing.Stop()
				delete(a.checkMonitors, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}
			monitor := &checks.CheckMonitor{
				Notify:        a.State,
				CheckID:       cid,
				ServiceID:     sid,
				ScriptArgs:    chkType.ScriptArgs,
				Interval:      chkType.Interval,
				Timeout:       chkType.Timeout,
				Logger:        a.logger,
				OutputMaxSize: maxOutputSize,
				StatusHandler: statusHandler,
			}
			monitor.Start()
			a.checkMonitors[cid] = monitor

		case chkType.IsH2PING():
			if existing, ok := a.checkH2PINGs[cid]; ok {
				existing.Stop()
				delete(a.checkH2PINGs, cid)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Warn("check has interval below minimum",
					"check", cid.String(),
					"minimum_interval", checks.MinInterval,
				)
				chkType.Interval = checks.MinInterval
			}
			var tlsClientConfig *tls.Config
			if chkType.H2PingUseTLS {
				tlsClientConfig = a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify, chkType.TLSServerName)
				tlsClientConfig.NextProtos = []string{http2.NextProtoTLS}
			}

			h2ping := &checks.CheckH2PING{
				CheckID:         cid,
				ServiceID:       sid,
				H2PING:          chkType.H2PING,
				Interval:        chkType.Interval,
				Timeout:         chkType.Timeout,
				Logger:          a.logger,
				TLSClientConfig: tlsClientConfig,
				StatusHandler:   statusHandler,
			}

			h2ping.Start()
			a.checkH2PINGs[cid] = h2ping

		case chkType.IsAlias():
			if existing, ok := a.checkAliases[cid]; ok {
				existing.Stop()
				delete(a.checkAliases, cid)
			}

			var rpcReq structs.NodeSpecificRequest
			rpcReq.Datacenter = a.config.Datacenter
			rpcReq.EnterpriseMeta = *a.AgentEnterpriseMeta()

			// The token to set is really important. The behavior below follows
			// the same behavior as anti-entropy: we use the user-specified token
			// if set (either on the service or check definition), otherwise
			// we use the "UserToken" on the agent. This is tested.
			rpcReq.Token = a.tokens.UserToken()
			if token != "" {
				rpcReq.Token = token
			}

			aliasServiceID := structs.NewServiceID(chkType.AliasService, &check.EnterpriseMeta)
			chkImpl := &checks.CheckAlias{
				Notify:         a.State,
				RPC:            a.delegate,
				RPCReq:         rpcReq,
				CheckID:        cid,
				Node:           chkType.AliasNode,
				ServiceID:      aliasServiceID,
				EnterpriseMeta: check.EnterpriseMeta,
			}
			chkImpl.Start()
			a.checkAliases[cid] = chkImpl

		default:
			return fmt.Errorf("Check type is not valid")
		}

		// Notify channel that watches for service state changes
		// This is a non-blocking send to avoid synchronizing on a large number of check updates
		s := a.State.ServiceState(sid)
		if s != nil && !s.Deleted {
			select {
			case s.WatchCh <- struct{}{}:
			default:
			}
		}

		if chkType.DeregisterCriticalServiceAfter > 0 {
			timeout := chkType.DeregisterCriticalServiceAfter
			if timeout < a.config.CheckDeregisterIntervalMin {
				timeout = a.config.CheckDeregisterIntervalMin
				a.logger.Warn("check has deregister interval below minimum",
					"check", cid.String(),
					"minimum_interval", a.config.CheckDeregisterIntervalMin,
				)
			}
			a.checkReapAfter[cid] = timeout
		} else {
			delete(a.checkReapAfter, cid)
		}
	}

	return nil
}

// RemoveCheck is used to remove a health check.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveCheck(checkID structs.CheckID, persist bool) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.removeCheckLocked(checkID, persist)
}

// removeCheckLocked is used to remove a health check.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) removeCheckLocked(checkID structs.CheckID, persist bool) error {
	// Validate CheckID
	if checkID.ID == "" {
		return fmt.Errorf("CheckID missing")
	}

	// Notify channel that watches for service state changes
	// This is a non-blocking send to avoid synchronizing on a large number of check updates
	var svcID structs.ServiceID
	if c := a.State.Check(checkID); c != nil {
		svcID = c.CompoundServiceID()
	}

	s := a.State.ServiceState(svcID)
	if s != nil && !s.Deleted {
		select {
		case s.WatchCh <- struct{}{}:
		default:
		}
	}

	// Delete port from allocated port set
	// If checks weren't being exposed then this is a no-op
	portKey := listenerPortKey(svcID, checkID)
	delete(a.exposedPorts, portKey)

	a.cancelCheckMonitors(checkID)
	a.State.RemoveCheck(checkID)

	if persist {
		if err := a.purgeCheck(checkID); err != nil {
			return err
		}
		if err := a.purgeCheckState(checkID); err != nil {
			return err
		}
	}

	a.logger.Debug("removed check", "check", checkID.String())
	return nil
}

// ServiceHTTPBasedChecks returns HTTP and GRPC based Checks
// for the given serviceID
func (a *Agent) ServiceHTTPBasedChecks(serviceID structs.ServiceID) []structs.CheckType {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	var chkTypes = make([]structs.CheckType, 0)
	for _, c := range a.checkHTTPs {
		if c.ServiceID == serviceID {
			chkTypes = append(chkTypes, c.CheckType())
		}
	}
	for _, c := range a.checkGRPCs {
		if c.ServiceID == serviceID {
			chkTypes = append(chkTypes, c.CheckType())
		}
	}
	return chkTypes
}

// AdvertiseAddrLAN returns the AdvertiseAddrLAN config value
func (a *Agent) AdvertiseAddrLAN() string {
	return a.config.AdvertiseAddrLAN.String()
}

func (a *Agent) cancelCheckMonitors(checkID structs.CheckID) {
	// Stop any monitors
	delete(a.checkReapAfter, checkID)
	if check, ok := a.checkMonitors[checkID]; ok {
		check.Stop()
		delete(a.checkMonitors, checkID)
	}
	if check, ok := a.checkHTTPs[checkID]; ok {
		check.Stop()
		delete(a.checkHTTPs, checkID)
	}
	if check, ok := a.checkTCPs[checkID]; ok {
		check.Stop()
		delete(a.checkTCPs, checkID)
	}
	if check, ok := a.checkUDPs[checkID]; ok {
		check.Stop()
		delete(a.checkUDPs, checkID)
	}
	if check, ok := a.checkGRPCs[checkID]; ok {
		check.Stop()
		delete(a.checkGRPCs, checkID)
	}
	if check, ok := a.checkTTLs[checkID]; ok {
		check.Stop()
		delete(a.checkTTLs, checkID)
	}
	if check, ok := a.checkDockers[checkID]; ok {
		check.Stop()
		delete(a.checkDockers, checkID)
	}
	if check, ok := a.checkH2PINGs[checkID]; ok {
		check.Stop()
		delete(a.checkH2PINGs, checkID)
	}
	if check, ok := a.checkAliases[checkID]; ok {
		check.Stop()
		delete(a.checkAliases, checkID)
	}
}

// updateTTLCheck is used to update the status of a TTL check via the Agent API.
func (a *Agent) updateTTLCheck(checkID structs.CheckID, status, output string) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	// Grab the TTL check.
	check, ok := a.checkTTLs[checkID]
	if !ok {
		return fmt.Errorf("CheckID %q does not have associated TTL", checkID.String())
	}

	// Set the status through CheckTTL to reset the TTL.
	outputTruncated := check.SetStatus(status, output)

	// We don't write any files in dev mode so bail here.
	if a.config.DataDir == "" {
		return nil
	}

	// Persist the state so the TTL check can come up in a good state after
	// an agent restart, especially with long TTL values.
	if err := a.persistCheckState(check, status, outputTruncated); err != nil {
		return fmt.Errorf("failed persisting state for check %q: %s", checkID.String(), err)
	}

	return nil
}

// persistCheckState is used to record the check status into the data dir.
// This allows the state to be restored on a later agent start. Currently
// only useful for TTL based checks.
func (a *Agent) persistCheckState(check *checks.CheckTTL, status, output string) error {
	// Create the persisted state
	state := persistedCheckState{
		CheckID:        check.CheckID.ID,
		Status:         status,
		Output:         output,
		Expires:        time.Now().Add(check.TTL).Unix(),
		EnterpriseMeta: check.CheckID.EnterpriseMeta,
	}

	// Encode the state
	buf, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Create the state dir if it doesn't exist
	dir := filepath.Join(a.config.DataDir, checkStateDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating check state dir %q: %s", dir, err)
	}

	// Write the state to the file
	file := filepath.Join(dir, check.CheckID.StringHashSHA256())

	// Create temp file in same dir, to make more likely atomic
	tempFile := file + ".tmp"

	// persistCheckState is called frequently, so don't use writeFileAtomic to avoid calling fsync here
	if err := ioutil.WriteFile(tempFile, buf, 0600); err != nil {
		return fmt.Errorf("failed writing temp file %q: %s", tempFile, err)
	}
	if err := os.Rename(tempFile, file); err != nil {
		return fmt.Errorf("failed to rename temp file from %q to %q: %s", tempFile, file, err)
	}

	return nil
}

// loadCheckState is used to restore the persisted state of a check.
func (a *Agent) loadCheckState(check *structs.HealthCheck) error {
	cid := check.CompoundCheckID()
	// Try to read the persisted state for this check
	file := filepath.Join(a.config.DataDir, checkStateDir, cid.StringHashSHA256())
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			// try the md5 based name. This can be removed once we no longer support upgrades from versions that use MD5 hashing
			oldFile := filepath.Join(a.config.DataDir, checkStateDir, cid.StringHashMD5())
			buf, err = ioutil.ReadFile(oldFile)
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				} else {
					return fmt.Errorf("failed reading check state %q: %w", file, err)
				}
			}
			if err := os.Rename(oldFile, file); err != nil {
				a.logger.Error("Failed renaming check state",
					"file", oldFile,
					"targetFile", file,
					"error", err,
				)
			}
		} else {
			return fmt.Errorf("failed reading file %q: %w", file, err)
		}
	}

	// Decode the state data
	var p persistedCheckState
	if err := json.Unmarshal(buf, &p); err != nil {
		a.logger.Error("failed decoding check state", "error", err)
		return a.purgeCheckState(cid)
	}

	// Check if the state has expired
	if time.Now().Unix() >= p.Expires {
		a.logger.Debug("check state expired, not restoring", "check", cid.String())
		return a.purgeCheckState(cid)
	}

	// Restore the fields from the state
	check.Output = p.Output
	check.Status = p.Status
	return nil
}

// purgeCheckState is used to purge the state of a check from the data dir
func (a *Agent) purgeCheckState(checkID structs.CheckID) error {
	file := filepath.Join(a.config.DataDir, checkStateDir, checkID.StringHashSHA256())
	err := os.Remove(file)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Stats is used to get various debugging state from the sub-systems
func (a *Agent) Stats() map[string]map[string]string {
	stats := a.delegate.Stats()
	stats["agent"] = map[string]string{
		"check_monitors": strconv.Itoa(len(a.checkMonitors)),
		"check_ttls":     strconv.Itoa(len(a.checkTTLs)),
	}
	for k, v := range a.State.Stats() {
		stats["agent"][k] = v
	}

	revision := a.config.Revision
	if len(revision) > 8 {
		revision = revision[:8]
	}
	stats["build"] = map[string]string{
		"revision":         revision,
		"version":          a.config.Version,
		"version_metadata": a.config.VersionMetadata,
		"prerelease":       a.config.VersionPrerelease,
	}

	for outerKey, outerValue := range a.enterpriseStats() {
		if _, ok := stats[outerKey]; ok {
			for innerKey, innerValue := range outerValue {
				stats[outerKey][innerKey] = innerValue
			}
		} else {
			stats[outerKey] = outerValue
		}
	}

	return stats
}

// storePid is used to write out our PID to a file if necessary
func (a *Agent) storePid() error {
	// Quit fast if no pidfile
	pidPath := a.config.PidFile
	if pidPath == "" {
		return nil
	}

	// Open the PID file
	pidFile, err := os.OpenFile(pidPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("Could not open pid file: %v", err)
	}
	defer pidFile.Close()

	// Write out the PID
	pid := os.Getpid()
	_, err = pidFile.WriteString(fmt.Sprintf("%d", pid))
	if err != nil {
		return fmt.Errorf("Could not write to pid file: %s", err)
	}
	return nil
}

// deletePid is used to delete our PID on exit
func (a *Agent) deletePid() error {
	// Quit fast if no pidfile
	pidPath := a.config.PidFile
	if pidPath == "" {
		return nil
	}

	stat, err := os.Stat(pidPath)
	if err != nil {
		return fmt.Errorf("Could not remove pid file: %s", err)
	}

	if stat.IsDir() {
		return fmt.Errorf("Specified pid file path is directory")
	}

	err = os.Remove(pidPath)
	if err != nil {
		return fmt.Errorf("Could not remove pid file: %s", err)
	}
	return nil
}

// loadServices will load service definitions from configuration and persisted
// definitions on disk, and load them into the local agent.
func (a *Agent) loadServices(conf *config.RuntimeConfig, snap map[structs.CheckID]*structs.HealthCheck) error {
	// Load any persisted service configs so we can feed those into the initial
	// registrations below.
	persistedServiceConfigs, err := a.readPersistedServiceConfigs()
	if err != nil {
		return err
	}

	// Register the services from config
	for _, service := range conf.Services {
		// Default service partition to the same as agent
		if service.EnterpriseMeta.PartitionOrEmpty() == "" {
			service.EnterpriseMeta.OverridePartition(a.AgentEnterpriseMeta().PartitionOrDefault())
		}

		ns := service.NodeService()
		chkTypes, err := service.CheckTypes()
		if err != nil {
			return fmt.Errorf("Failed to validate checks for service %q: %v", service.Name, err)
		}

		// Grab and validate sidecar if there is one too
		sidecar, sidecarChecks, sidecarToken, err := sidecarServiceFromNodeService(ns, service.Token)
		if err != nil {
			return fmt.Errorf("Failed to validate sidecar for service %q: %v", service.Name, err)
		}

		// Remove sidecar from NodeService now it's done it's job it's just a config
		// syntax sugar and shouldn't be persisted in local or server state.
		ns.Connect.SidecarService = nil

		sid := ns.CompoundServiceID()
		err = a.addServiceLocked(addServiceLockedRequest{
			AddServiceRequest: AddServiceRequest{
				Service:               ns,
				chkTypes:              chkTypes,
				persist:               false, // don't rewrite the file with the same data we just read
				token:                 service.Token,
				replaceExistingChecks: false, // do default behavior
				Source:                ConfigSourceLocal,
			},
			serviceDefaults:      serviceDefaultsFromStruct(persistedServiceConfigs[sid]),
			persistServiceConfig: false, // don't rewrite the file with the same data we just read
			checkStateSnapshot:   snap,
		})
		if err != nil {
			return fmt.Errorf("Failed to register service %q: %v", service.Name, err)
		}

		// If there is a sidecar service, register that too.
		if sidecar != nil {
			sidecarServiceID := sidecar.CompoundServiceID()
			err = a.addServiceLocked(addServiceLockedRequest{
				AddServiceRequest: AddServiceRequest{
					Service:               sidecar,
					chkTypes:              sidecarChecks,
					persist:               false, // don't rewrite the file with the same data we just read
					token:                 sidecarToken,
					replaceExistingChecks: false, // do default behavior
					Source:                ConfigSourceLocal,
				},
				serviceDefaults:      serviceDefaultsFromStruct(persistedServiceConfigs[sidecarServiceID]),
				persistServiceConfig: false, // don't rewrite the file with the same data we just read
				checkStateSnapshot:   snap,
			})
			if err != nil {
				return fmt.Errorf("Failed to register sidecar for service %q: %v", service.Name, err)
			}
		}
	}

	// Load any persisted services
	svcDir := filepath.Join(a.config.DataDir, servicesDir)
	files, err := ioutil.ReadDir(svcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed reading services dir %q: %w", svcDir, err)
	}
	for _, fi := range files {
		// Skip all dirs
		if fi.IsDir() {
			continue
		}

		// Skip all partially written temporary files
		if strings.HasSuffix(fi.Name(), "tmp") {
			a.logger.Warn("Ignoring temporary service file", "file", fi.Name())
			continue
		}

		// Read the contents into a buffer
		file := filepath.Join(svcDir, fi.Name())
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed reading service file %q: %w", file, err)
		}

		// Try decoding the service definition
		var p persistedService
		if err := json.Unmarshal(buf, &p); err != nil {
			// Backwards-compatibility for pre-0.5.1 persisted services
			if err := json.Unmarshal(buf, &p.Service); err != nil {
				a.logger.Error("Failed decoding service file",
					"file", file,
					"error", err,
				)
				continue
			}
		}

		// Rename files that used the old md5 hash to the new sha256 name; only needed when upgrading from 1.10 and before.
		newPath := a.makeServiceFilePath(p.Service.CompoundServiceID())
		if file != newPath {
			if err := os.Rename(file, newPath); err != nil {
				a.logger.Error("Failed renaming service file",
					"file", file,
					"targetFile", newPath,
					"error", err,
				)
			}
		}

		if acl.EqualPartitions("", p.Service.PartitionOrEmpty()) {
			// NOTE: in case loading a service with empty partition (e.g., OSS -> ENT),
			// we always default the service partition to the agent's partition.
			p.Service.OverridePartition(a.AgentEnterpriseMeta().PartitionOrDefault())
		} else if !acl.EqualPartitions(a.AgentEnterpriseMeta().PartitionOrDefault(), p.Service.PartitionOrDefault()) {
			a.logger.Info("Purging service file in wrong partition",
				"file", file,
				"partition", p.Service.EnterpriseMeta.PartitionOrDefault(),
			)
			if err := os.Remove(file); err != nil {
				a.logger.Error("Failed purging service file",
					"file", file,
					"error", err,
				)
			}
			continue
		}

		// Restore LocallyRegisteredAsSidecar, see persistedService.LocallyRegisteredAsSidecar
		p.Service.LocallyRegisteredAsSidecar = p.LocallyRegisteredAsSidecar

		serviceID := p.Service.CompoundServiceID()

		source, ok := ConfigSourceFromName(p.Source)
		if !ok {
			a.logger.Warn("service exists with invalid source, purging",
				"service", serviceID.String(),
				"source", p.Source,
			)
			if err := a.purgeService(serviceID); err != nil {
				return fmt.Errorf("failed purging service %q: %w", serviceID, err)
			}
			if err := a.purgeServiceConfig(serviceID); err != nil {
				return fmt.Errorf("failed purging service config %q: %w", serviceID, err)
			}
			continue
		}

		if a.State.Service(serviceID) != nil {
			// Purge previously persisted service. This allows config to be
			// preferred over services persisted from the API.
			a.logger.Debug("service exists, not restoring from file",
				"service", serviceID.String(),
				"file", file,
			)
			if err := a.purgeService(serviceID); err != nil {
				return fmt.Errorf("failed purging service %q: %w", serviceID.String(), err)
			}
			if err := a.purgeServiceConfig(serviceID); err != nil {
				return fmt.Errorf("failed purging service config %q: %w", serviceID.String(), err)
			}
		} else {
			a.logger.Debug("restored service definition from file",
				"service", serviceID.String(),
				"file", file,
			)
			err = a.addServiceLocked(addServiceLockedRequest{
				AddServiceRequest: AddServiceRequest{
					Service:               p.Service,
					chkTypes:              nil,
					persist:               false, // don't rewrite the file with the same data we just read
					token:                 p.Token,
					replaceExistingChecks: false, // do default behavior
					Source:                source,
				},
				serviceDefaults:      serviceDefaultsFromStruct(persistedServiceConfigs[serviceID]),
				persistServiceConfig: false, // don't rewrite the file with the same data we just read
				checkStateSnapshot:   snap,
			})
			if err != nil {
				return fmt.Errorf("failed adding service %q: %w", serviceID, err)
			}
		}
	}

	for serviceID := range persistedServiceConfigs {
		if a.State.Service(serviceID) == nil {
			// This can be cleaned up now.
			if err := a.purgeServiceConfig(serviceID); err != nil {
				return fmt.Errorf("failed purging service config %q: %w", serviceID, err)
			}
		}
	}

	return nil
}

// unloadServices will deregister all services.
func (a *Agent) unloadServices() error {
	for id := range a.State.AllServices() {
		if err := a.removeServiceLocked(id, false); err != nil {
			return fmt.Errorf("Failed deregistering service '%s': %v", id, err)
		}
	}
	return nil
}

// loadChecks loads check definitions and/or persisted check definitions from
// disk and re-registers them with the local agent.
func (a *Agent) loadChecks(conf *config.RuntimeConfig, snap map[structs.CheckID]*structs.HealthCheck) error {
	// Register the checks from config
	for _, check := range conf.Checks {
		health := check.HealthCheck(conf.NodeName)
		// Restore the fields from the snapshot.
		if prev, ok := snap[health.CompoundCheckID()]; ok {
			health.Output = prev.Output
			health.Status = prev.Status
		}

		chkType := check.CheckType()
		if err := a.addCheckLocked(health, chkType, false, check.Token, ConfigSourceLocal); err != nil {
			return fmt.Errorf("Failed to register check '%s': %v %v", check.Name, err, check)
		}
	}

	// Load any persisted checks
	checkDir := filepath.Join(a.config.DataDir, checksDir)
	files, err := ioutil.ReadDir(checkDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("Failed reading checks dir %q: %w", checkDir, err)
	}
	for _, fi := range files {
		// Ignore dirs - we only care about the check definition files
		if fi.IsDir() {
			continue
		}

		// Read the contents into a buffer
		file := filepath.Join(checkDir, fi.Name())
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed reading check file %q: %w", file, err)
		}

		// Decode the check
		var p persistedCheck
		if err := json.Unmarshal(buf, &p); err != nil {
			a.logger.Error("Failed decoding check file",
				"file", file,
				"error", err,
			)
			continue
		}
		checkID := p.Check.CompoundCheckID()

		// Rename files that used the old md5 hash to the new sha256 name; only needed when upgrading from 1.10 and before.
		newPath := filepath.Join(a.config.DataDir, checksDir, checkID.StringHashSHA256())
		if file != newPath {
			if err := os.Rename(file, newPath); err != nil {
				a.logger.Error("Failed renaming check file",
					"file", file,
					"targetFile", newPath,
					"error", err,
				)
			}
		}

		if !acl.EqualPartitions(a.AgentEnterpriseMeta().PartitionOrDefault(), p.Check.PartitionOrDefault()) {
			a.logger.Info("Purging check file in wrong partition",
				"file", file,
				"partition", p.Check.PartitionOrDefault(),
			)
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("failed purging check %q: %w", checkID, err)
			}
			continue
		}

		source, ok := ConfigSourceFromName(p.Source)
		if !ok {
			a.logger.Warn("check exists with invalid source, purging",
				"check", checkID.String(),
				"source", p.Source,
			)
			if err := a.purgeCheck(checkID); err != nil {
				return fmt.Errorf("failed purging check %q: %w", checkID, err)
			}
			continue
		}

		if a.State.Check(checkID) != nil {
			// Purge previously persisted check. This allows config to be
			// preferred over persisted checks from the API.
			a.logger.Debug("check exists, not restoring from file",
				"check", checkID.String(),
				"file", file,
			)
			if err := a.purgeCheck(checkID); err != nil {
				return fmt.Errorf("Failed purging check %q: %w", checkID, err)
			}
		} else {
			// Default check to critical to avoid placing potentially unhealthy
			// services into the active pool
			p.Check.Status = api.HealthCritical

			// Restore the fields from the snapshot.
			if prev, ok := snap[p.Check.CompoundCheckID()]; ok {
				p.Check.Output = prev.Output
				p.Check.Status = prev.Status
			}

			if err := a.addCheckLocked(p.Check, p.ChkType, false, p.Token, source); err != nil {
				// Purge the check if it is unable to be restored.
				a.logger.Warn("Failed to restore check",
					"check", checkID.String(),
					"error", err,
				)
				if err := a.purgeCheck(checkID); err != nil {
					return fmt.Errorf("Failed purging check %q: %w", checkID, err)
				}
			}
			a.logger.Debug("restored health check from file",
				"check", p.Check.CheckID,
				"file", file,
			)
		}
	}

	return nil
}

// unloadChecks will deregister all checks known to the local agent.
func (a *Agent) unloadChecks() error {
	for id := range a.State.AllChecks() {
		if err := a.removeCheckLocked(id, false); err != nil {
			return fmt.Errorf("Failed deregistering check '%s': %s", id, err)
		}
	}
	return nil
}

// snapshotCheckState is used to snapshot the current state of the health
// checks. This is done before we reload our checks, so that we can properly
// restore into the same state.
func (a *Agent) snapshotCheckState() map[structs.CheckID]*structs.HealthCheck {
	return a.State.AllChecks()
}

// loadMetadata loads node metadata fields from the agent config and
// updates them on the local agent.
func (a *Agent) loadMetadata(conf *config.RuntimeConfig) error {
	meta := map[string]string{}
	for k, v := range conf.NodeMeta {
		meta[k] = v
	}
	meta[structs.MetaSegmentKey] = conf.SegmentName
	return a.State.LoadMetadata(meta)
}

// unloadMetadata resets the local metadata state
func (a *Agent) unloadMetadata() {
	a.State.UnloadMetadata()
}

// serviceMaintCheckID returns the ID of a given service's maintenance check
func serviceMaintCheckID(serviceID structs.ServiceID) structs.CheckID {
	cid := types.CheckID(structs.ServiceMaintPrefix + serviceID.ID)
	return structs.NewCheckID(cid, &serviceID.EnterpriseMeta)
}

// EnableServiceMaintenance will register a false health check against the given
// service ID with critical status. This will exclude the service from queries.
func (a *Agent) EnableServiceMaintenance(serviceID structs.ServiceID, reason, token string) error {
	service := a.State.Service(serviceID)
	if service == nil {
		return fmt.Errorf("No service registered with ID %q", serviceID.String())
	}

	// Check if maintenance mode is not already enabled
	checkID := serviceMaintCheckID(serviceID)
	if a.State.Check(checkID) != nil {
		return nil
	}

	// Use default notes if no reason provided
	if reason == "" {
		reason = defaultServiceMaintReason
	}

	// Create and register the critical health check
	check := &structs.HealthCheck{
		Node:           a.config.NodeName,
		CheckID:        checkID.ID,
		Name:           "Service Maintenance Mode",
		Notes:          reason,
		ServiceID:      service.ID,
		ServiceName:    service.Service,
		Status:         api.HealthCritical,
		Type:           "maintenance",
		EnterpriseMeta: checkID.EnterpriseMeta,
	}
	a.AddCheck(check, nil, true, token, ConfigSourceLocal)
	a.logger.Info("Service entered maintenance mode", "service", serviceID.String())

	return nil
}

// DisableServiceMaintenance will deregister the fake maintenance mode check
// if the service has been marked as in maintenance.
func (a *Agent) DisableServiceMaintenance(serviceID structs.ServiceID) error {
	if a.State.Service(serviceID) == nil {
		return fmt.Errorf("No service registered with ID %q", serviceID.String())
	}

	// Check if maintenance mode is enabled
	checkID := serviceMaintCheckID(serviceID)
	if a.State.Check(checkID) == nil {
		// maintenance mode is not enabled
		return nil
	}

	// Deregister the maintenance check
	a.RemoveCheck(checkID, true)
	a.logger.Info("Service left maintenance mode", "service", serviceID.String())

	return nil
}

// EnableNodeMaintenance places a node into maintenance mode.
func (a *Agent) EnableNodeMaintenance(reason, token string) {
	// Ensure node maintenance is not already enabled
	if a.State.Check(structs.NodeMaintCheckID) != nil {
		return
	}

	// Use a default notes value
	if reason == "" {
		reason = defaultNodeMaintReason
	}

	// Create and register the node maintenance check
	check := &structs.HealthCheck{
		Node:    a.config.NodeName,
		CheckID: structs.NodeMaint,
		Name:    "Node Maintenance Mode",
		Notes:   reason,
		Status:  api.HealthCritical,
		Type:    "maintenance",
	}
	a.AddCheck(check, nil, true, token, ConfigSourceLocal)
	a.logger.Info("Node entered maintenance mode")
}

// DisableNodeMaintenance removes a node from maintenance mode
func (a *Agent) DisableNodeMaintenance() {
	if a.State.Check(structs.NodeMaintCheckID) == nil {
		return
	}
	a.RemoveCheck(structs.NodeMaintCheckID, true)
	a.logger.Info("Node left maintenance mode")
}

func (a *Agent) AutoReloadConfig() error {
	return a.reloadConfig(true)
}

func (a *Agent) ReloadConfig() error {
	return a.reloadConfig(false)
}

// ReloadConfig will atomically reload all configuration, including
// all services, checks, tokens, metadata, dnsServer configs, etc.
// It will also reload all ongoing watches.
func (a *Agent) reloadConfig(autoReload bool) error {
	newCfg, err := a.baseDeps.AutoConfig.ReadConfig()
	if err != nil {
		return err
	}

	// copy over the existing node id, this cannot be
	// changed while running anyways but this prevents
	// breaking some existing behavior.
	newCfg.NodeID = a.config.NodeID

	// if auto reload is enabled, make sure we have the right certs file watched.
	if autoReload {
		for _, f := range []struct {
			oldCfg tlsutil.ProtocolConfig
			newCfg tlsutil.ProtocolConfig
		}{
			{a.config.TLS.InternalRPC, newCfg.TLS.InternalRPC},
			{a.config.TLS.GRPC, newCfg.TLS.GRPC},
			{a.config.TLS.HTTPS, newCfg.TLS.HTTPS},
		} {
			if f.oldCfg.KeyFile != f.newCfg.KeyFile {
				err = a.configFileWatcher.Replace(f.oldCfg.KeyFile, f.newCfg.KeyFile)
				if err != nil {
					return err
				}
			}
			if f.oldCfg.CertFile != f.newCfg.CertFile {
				err = a.configFileWatcher.Replace(f.oldCfg.CertFile, f.newCfg.CertFile)
				if err != nil {
					return err
				}
			}
			if revertStaticConfig(f.oldCfg, f.newCfg) {
				a.logger.Warn("Changes to your configuration were detected that for security reasons cannot be automatically applied by 'auto_reload_config'. Manually reload your configuration (e.g. with 'consul reload') to apply these changes.", "StaticRuntimeConfig", f.oldCfg, "StaticRuntimeConfig From file", f.newCfg)
			}
		}
		if !reflect.DeepEqual(newCfg.StaticRuntimeConfig, a.config.StaticRuntimeConfig) {
			a.logger.Warn("Changes to your configuration were detected that for security reasons cannot be automatically applied by 'auto_reload_config'. Manually reload your configuration (e.g. with 'consul reload') to apply these changes.", "StaticRuntimeConfig", a.config.StaticRuntimeConfig, "StaticRuntimeConfig From file", newCfg.StaticRuntimeConfig)
			// reset not reloadable fields
			newCfg.StaticRuntimeConfig = a.config.StaticRuntimeConfig
		}
	}

	return a.reloadConfigInternal(newCfg)
}

func revertStaticConfig(oldCfg tlsutil.ProtocolConfig, newCfg tlsutil.ProtocolConfig) bool {
	newNewCfg := oldCfg
	newNewCfg.CertFile = newCfg.CertFile
	newNewCfg.KeyFile = newCfg.KeyFile
	newOldcfg := newCfg
	newOldcfg.CertFile = oldCfg.CertFile
	newOldcfg.KeyFile = oldCfg.KeyFile
	if !reflect.DeepEqual(newOldcfg, oldCfg) {
		return true
	}
	return false
}

// reloadConfigInternal is mainly needed for some unit tests. Instead of parsing
// the configuration using CLI flags and on disk config, this just takes a
// runtime configuration and applies it.
func (a *Agent) reloadConfigInternal(newCfg *config.RuntimeConfig) error {
	// Change the log level and update it
	if logging.ValidateLogLevel(newCfg.Logging.LogLevel) {
		a.logger.SetLevel(logging.LevelFromString(newCfg.Logging.LogLevel))
	} else {
		a.logger.Warn("Invalid log level in new configuration", "level", newCfg.Logging.LogLevel)
		newCfg.Logging.LogLevel = a.config.Logging.LogLevel
	}

	// Bulk update the services and checks
	a.PauseSync()
	defer a.ResumeSync()

	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	// Snapshot the current state, and use that to initialize the checks when
	// they are recreated.
	snap := a.snapshotCheckState()

	// First unload all checks, services, and metadata. This lets us begin the reload
	// with a clean slate.
	if err := a.unloadServices(); err != nil {
		return fmt.Errorf("Failed unloading services: %s", err)
	}
	if err := a.unloadChecks(); err != nil {
		return fmt.Errorf("Failed unloading checks: %s", err)
	}
	a.unloadMetadata()

	// Reload tokens - should be done before all the other loading
	// to ensure the correct tokens are available for attaching to
	// the checks and service registrations.
	a.tokens.Load(newCfg.ACLTokens, a.logger)

	if err := a.tlsConfigurator.Update(newCfg.TLS); err != nil {
		return fmt.Errorf("Failed reloading tls configuration: %s", err)
	}

	// Reload service/check definitions and metadata.
	if err := a.loadServices(newCfg, snap); err != nil {
		return fmt.Errorf("Failed reloading services: %s", err)
	}
	if err := a.loadChecks(newCfg, snap); err != nil {
		return fmt.Errorf("Failed reloading checks: %s", err)
	}
	if err := a.loadMetadata(newCfg); err != nil {
		return fmt.Errorf("Failed reloading metadata: %s", err)
	}

	if err := a.reloadWatches(newCfg); err != nil {
		return fmt.Errorf("Failed reloading watches: %v", err)
	}

	a.httpConnLimiter.SetConfig(connlimit.Config{
		MaxConnsPerClientIP: newCfg.HTTPMaxConnsPerClient,
	})

	for _, s := range a.dnsServers {
		if err := s.ReloadConfig(newCfg); err != nil {
			return fmt.Errorf("Failed reloading dns config : %v", err)
		}
	}

	err := a.reloadEnterprise(newCfg)
	if err != nil {
		return err
	}

	cc := consul.ReloadableConfig{
		RPCClientTimeout:      newCfg.RPCClientTimeout,
		RPCRateLimit:          newCfg.RPCRateLimit,
		RPCMaxBurst:           newCfg.RPCMaxBurst,
		RPCMaxConnsPerClient:  newCfg.RPCMaxConnsPerClient,
		ConfigEntryBootstrap:  newCfg.ConfigEntryBootstrap,
		RaftSnapshotThreshold: newCfg.RaftSnapshotThreshold,
		RaftSnapshotInterval:  newCfg.RaftSnapshotInterval,
		HeartbeatTimeout:      newCfg.ConsulRaftHeartbeatTimeout,
		ElectionTimeout:       newCfg.ConsulRaftElectionTimeout,
		RaftTrailingLogs:      newCfg.RaftTrailingLogs,
		Reporting: consul.Reporting{
			License: consul.License{
				Enabled: newCfg.Reporting.License.Enabled,
			},
		},
	}
	if err := a.delegate.ReloadConfig(cc); err != nil {
		return err
	}

	if a.cache.ReloadOptions(newCfg.Cache) {
		a.logger.Info("Cache options have been updated")
	} else {
		a.logger.Debug("Cache options have not been modified")
	}

	// Update filtered metrics
	metrics.UpdateFilter(newCfg.Telemetry.AllowedPrefixes,
		newCfg.Telemetry.BlockedPrefixes)

	a.State.SetDiscardCheckOutput(newCfg.DiscardCheckOutput)

	for _, r := range a.configReloaders {
		if err := r(newCfg); err != nil {
			return err
		}
	}

	a.proxyConfig.SetUpdateRateLimit(newCfg.XDSUpdateRateLimit)

	a.config.EnableDebug = newCfg.EnableDebug

	return nil
}

// LocalBlockingQuery performs a blocking query in a generic way against
// local agent state that has no RPC or raft to back it. It uses `hash` parameter
// instead of an `index`.
// `alwaysBlock` determines whether we block if the provided hash is empty.
// Callers like the AgentService endpoint will want to return the current result if a hash isn't provided.
// On the other hand, for cache notifications we always want to block. This avoids an empty first response.
func (a *Agent) LocalBlockingQuery(alwaysBlock bool, hash string, wait time.Duration,
	fn func(ws memdb.WatchSet) (string, interface{}, error)) (string, interface{}, error) {

	// If we are not blocking we can skip tracking and allocating - nil WatchSet
	// is still valid to call Add on and will just be a no op.
	var ws memdb.WatchSet
	var ctx context.Context = &lib.StopChannelContext{StopCh: a.shutdownCh}
	shouldBlock := false

	if alwaysBlock || hash != "" {
		if wait == 0 {
			wait = defaultQueryTime
		}
		if wait > 10*time.Minute {
			wait = maxQueryTime
		}
		// Apply a small amount of jitter to the request.
		wait += lib.RandomStagger(wait / 16)
		var cancel func()
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(wait))
		defer cancel()

		shouldBlock = true
	}

	for {
		// Must reset this every loop in case the Watch set is already closed but
		// hash remains same. In that case we'll need to re-block on ws.Watch()
		// again.
		ws = memdb.NewWatchSet()
		curHash, curResp, err := fn(ws)
		if err != nil {
			return "", curResp, err
		}

		// Return immediately if there is no timeout, the hash is different or the
		// Watch returns true (indicating timeout fired). Note that Watch on a nil
		// WatchSet immediately returns false which would incorrectly cause this to
		// loop and repeat again, however we rely on the invariant that ws == nil
		// IFF timeout == nil in which case the Watch call is never invoked.
		if !shouldBlock || hash != curHash || ws.WatchCtx(ctx) != nil {
			return curHash, curResp, err
		}
		// Watch returned false indicating a change was detected, loop and repeat
		// the callback to load the new value. If agent sync is paused it means
		// local state is currently being bulk-edited e.g. config reload. In this
		// case it's likely that local state just got unloaded and may or may not be
		// reloaded yet. Wait a short amount of time for Sync to resume to ride out
		// typical config reloads.
		if syncPauseCh := a.SyncPausedCh(); syncPauseCh != nil {
			select {
			case <-syncPauseCh:
			case <-ctx.Done():
			}
		}
	}
}

// registerCache types on a.cache.
// This function may only be called once from New.
//
// Note: this function no longer registered all cache-types. Newer cache-types
// that do not depend on Agent are registered from registerCacheTypes.
func (a *Agent) registerCache() {
	// Note that you should register the _agent_ as the RPC implementation and not
	// the a.delegate directly, otherwise tests that rely on overriding RPC
	// routing via a.registerEndpoint will not work.

	a.cache.RegisterType(cachetype.ConnectCARootName, &cachetype.ConnectCARoot{RPC: a})

	a.cache.RegisterType(cachetype.ConnectCALeafName, &cachetype.ConnectCALeaf{
		RPC:                              a,
		Cache:                            a.cache,
		Datacenter:                       a.config.Datacenter,
		TestOverrideCAChangeInitialDelay: a.config.ConnectTestCALeafRootChangeSpread,
	})

	a.cache.RegisterType(cachetype.IntentionMatchName, &cachetype.IntentionMatch{RPC: a})

	a.cache.RegisterType(cachetype.IntentionUpstreamsName, &cachetype.IntentionUpstreams{RPC: a})
	a.cache.RegisterType(cachetype.IntentionUpstreamsDestinationName, &cachetype.IntentionUpstreamsDestination{RPC: a})

	a.cache.RegisterType(cachetype.CatalogServicesName, &cachetype.CatalogServices{RPC: a})

	a.cache.RegisterType(cachetype.HealthServicesName, &cachetype.HealthServices{RPC: a})

	a.cache.RegisterType(cachetype.PreparedQueryName, &cachetype.PreparedQuery{RPC: a})

	a.cache.RegisterType(cachetype.NodeServicesName, &cachetype.NodeServices{RPC: a})

	a.cache.RegisterType(cachetype.ResolvedServiceConfigName, &cachetype.ResolvedServiceConfig{RPC: a})

	a.cache.RegisterType(cachetype.CatalogListServicesName, &cachetype.CatalogListServices{RPC: a})

	a.cache.RegisterType(cachetype.CatalogServiceListName, &cachetype.CatalogServiceList{RPC: a})

	a.cache.RegisterType(cachetype.CatalogDatacentersName, &cachetype.CatalogDatacenters{RPC: a})

	a.cache.RegisterType(cachetype.InternalServiceDumpName, &cachetype.InternalServiceDump{RPC: a})

	a.cache.RegisterType(cachetype.CompiledDiscoveryChainName, &cachetype.CompiledDiscoveryChain{RPC: a})

	a.cache.RegisterType(cachetype.GatewayServicesName, &cachetype.GatewayServices{RPC: a})

	a.cache.RegisterType(cachetype.ServiceGatewaysName, &cachetype.ServiceGateways{RPC: a})

	a.cache.RegisterType(cachetype.ConfigEntryListName, &cachetype.ConfigEntryList{RPC: a})

	a.cache.RegisterType(cachetype.ConfigEntryName, &cachetype.ConfigEntry{RPC: a})

	a.cache.RegisterType(cachetype.ServiceHTTPChecksName, &cachetype.ServiceHTTPChecks{Agent: a})

	a.cache.RegisterType(cachetype.TrustBundleReadName, &cachetype.TrustBundle{Client: a.rpcClientPeering})

	a.cache.RegisterType(cachetype.ExportedPeeredServicesName, &cachetype.ExportedPeeredServices{RPC: a})

	a.cache.RegisterType(cachetype.FederationStateListMeshGatewaysName,
		&cachetype.FederationStateListMeshGateways{RPC: a})

	a.cache.RegisterType(cachetype.TrustBundleListName, &cachetype.TrustBundles{Client: a.rpcClientPeering})

	a.cache.RegisterType(cachetype.PeeredUpstreamsName, &cachetype.PeeredUpstreams{RPC: a})

	a.cache.RegisterType(cachetype.PeeringListName, &cachetype.Peerings{Client: a.rpcClientPeering})

	a.registerEntCache()
}

// LocalState returns the agent's local state
func (a *Agent) LocalState() *local.State {
	return a.State
}

// rerouteExposedChecks will inject proxy address into check targets
// Future calls to check() will dial the proxy listener
// The agent stateLock MUST be held for this to be called
func (a *Agent) rerouteExposedChecks(serviceID structs.ServiceID, proxyAddr string) error {
	for cid, c := range a.checkHTTPs {
		if c.ServiceID != serviceID {
			continue
		}
		port, err := a.listenerPortLocked(serviceID, cid)
		if err != nil {
			return err
		}
		c.ProxyHTTP = httpInjectAddr(c.HTTP, proxyAddr, port)
		hc := a.State.Check(cid)
		hc.ExposedPort = port
	}
	for cid, c := range a.checkGRPCs {
		if c.ServiceID != serviceID {
			continue
		}
		port, err := a.listenerPortLocked(serviceID, cid)
		if err != nil {
			return err
		}
		c.ProxyGRPC = grpcInjectAddr(c.GRPC, proxyAddr, port)
		hc := a.State.Check(cid)
		hc.ExposedPort = port
	}
	return nil
}

// resetExposedChecks will set Proxy addr in HTTP checks to empty string
// Future calls to check() will use the original target c.HTTP or c.GRPC
// The agent stateLock MUST be held for this to be called
func (a *Agent) resetExposedChecks(serviceID structs.ServiceID) {
	ids := make([]structs.CheckID, 0)
	for cid, c := range a.checkHTTPs {
		if c.ServiceID == serviceID {
			c.ProxyHTTP = ""
			hc := a.State.Check(cid)
			hc.ExposedPort = 0
			ids = append(ids, cid)
		}
	}
	for cid, c := range a.checkGRPCs {
		if c.ServiceID == serviceID {
			c.ProxyGRPC = ""
			hc := a.State.Check(cid)
			hc.ExposedPort = 0
			ids = append(ids, cid)
		}
	}
	for _, checkID := range ids {
		delete(a.exposedPorts, listenerPortKey(serviceID, checkID))
	}
}

// listenerPort allocates a port from the configured range
// The agent stateLock MUST be held when this is called
func (a *Agent) listenerPortLocked(svcID structs.ServiceID, checkID structs.CheckID) (int, error) {
	key := listenerPortKey(svcID, checkID)
	if a.exposedPorts == nil {
		a.exposedPorts = make(map[string]int)
	}
	if p, ok := a.exposedPorts[key]; ok {
		return p, nil
	}

	allocated := make(map[int]bool)
	for _, v := range a.exposedPorts {
		allocated[v] = true
	}

	var port int
	for i := 0; i < a.config.ExposeMaxPort-a.config.ExposeMinPort; i++ {
		port = a.config.ExposeMinPort + i
		if !allocated[port] {
			a.exposedPorts[key] = port
			break
		}
	}
	if port == 0 {
		return 0, fmt.Errorf("no ports available to expose '%s'", checkID)
	}

	return port, nil
}

func (a *Agent) proxyDataSources() proxycfg.DataSources {
	sources := proxycfg.DataSources{
		CARoots:                         proxycfgglue.CacheCARoots(a.cache),
		CompiledDiscoveryChain:          proxycfgglue.CacheCompiledDiscoveryChain(a.cache),
		ConfigEntry:                     proxycfgglue.CacheConfigEntry(a.cache),
		ConfigEntryList:                 proxycfgglue.CacheConfigEntryList(a.cache),
		Datacenters:                     proxycfgglue.CacheDatacenters(a.cache),
		FederationStateListMeshGateways: proxycfgglue.CacheFederationStateListMeshGateways(a.cache),
		GatewayServices:                 proxycfgglue.CacheGatewayServices(a.cache),
		ServiceGateways:                 proxycfgglue.CacheServiceGateways(a.cache),
		Health:                          proxycfgglue.ClientHealth(a.rpcClientHealth),
		HTTPChecks:                      proxycfgglue.CacheHTTPChecks(a.cache),
		Intentions:                      proxycfgglue.CacheIntentions(a.cache),
		IntentionUpstreams:              proxycfgglue.CacheIntentionUpstreams(a.cache),
		IntentionUpstreamsDestination:   proxycfgglue.CacheIntentionUpstreamsDestination(a.cache),
		InternalServiceDump:             proxycfgglue.CacheInternalServiceDump(a.cache),
		LeafCertificate:                 proxycfgglue.CacheLeafCertificate(a.cache),
		PeeredUpstreams:                 proxycfgglue.CachePeeredUpstreams(a.cache),
		PeeringList:                     proxycfgglue.CachePeeringList(a.cache),
		PreparedQuery:                   proxycfgglue.CachePrepraredQuery(a.cache),
		ResolvedServiceConfig:           proxycfgglue.CacheResolvedServiceConfig(a.cache),
		ServiceList:                     proxycfgglue.CacheServiceList(a.cache),
		TrustBundle:                     proxycfgglue.CacheTrustBundle(a.cache),
		TrustBundleList:                 proxycfgglue.CacheTrustBundleList(a.cache),
		ExportedPeeredServices:          proxycfgglue.CacheExportedPeeredServices(a.cache),
	}

	if server, ok := a.delegate.(*consul.Server); ok {
		deps := proxycfgglue.ServerDataSourceDeps{
			Datacenter:     a.config.Datacenter,
			EventPublisher: a.baseDeps.EventPublisher,
			ViewStore:      a.baseDeps.ViewStore,
			Logger:         a.logger.Named("proxycfg.server-data-sources"),
			ACLResolver:    a.delegate,
			GetStore:       func() proxycfgglue.Store { return server.FSM().State() },
		}
		sources.ConfigEntry = proxycfgglue.ServerConfigEntry(deps)
		sources.ConfigEntryList = proxycfgglue.ServerConfigEntryList(deps)
		sources.CompiledDiscoveryChain = proxycfgglue.ServerCompiledDiscoveryChain(deps, proxycfgglue.CacheCompiledDiscoveryChain(a.cache))
		sources.ExportedPeeredServices = proxycfgglue.ServerExportedPeeredServices(deps)
		sources.FederationStateListMeshGateways = proxycfgglue.ServerFederationStateListMeshGateways(deps)
		sources.GatewayServices = proxycfgglue.ServerGatewayServices(deps)
		sources.Health = proxycfgglue.ServerHealth(deps, proxycfgglue.ClientHealth(a.rpcClientHealth))
		sources.HTTPChecks = proxycfgglue.ServerHTTPChecks(deps, a.config.NodeName, proxycfgglue.CacheHTTPChecks(a.cache), a.State)
		sources.Intentions = proxycfgglue.ServerIntentions(deps)
		sources.IntentionUpstreams = proxycfgglue.ServerIntentionUpstreams(deps)
		sources.IntentionUpstreamsDestination = proxycfgglue.ServerIntentionUpstreamsDestination(deps)
		sources.InternalServiceDump = proxycfgglue.ServerInternalServiceDump(deps, proxycfgglue.CacheInternalServiceDump(a.cache))
		sources.PeeringList = proxycfgglue.ServerPeeringList(deps)
		sources.PeeredUpstreams = proxycfgglue.ServerPeeredUpstreams(deps)
		sources.ResolvedServiceConfig = proxycfgglue.ServerResolvedServiceConfig(deps, proxycfgglue.CacheResolvedServiceConfig(a.cache))
		sources.ServiceList = proxycfgglue.ServerServiceList(deps, proxycfgglue.CacheServiceList(a.cache))
		sources.TrustBundle = proxycfgglue.ServerTrustBundle(deps)
		sources.TrustBundleList = proxycfgglue.ServerTrustBundleList(deps)
	}

	a.fillEnterpriseProxyDataSources(&sources)
	return sources

}

func listenerPortKey(svcID structs.ServiceID, checkID structs.CheckID) string {
	return fmt.Sprintf("%s:%s", svcID, checkID)
}

// grpcInjectAddr injects an ip and port into an address of the form: ip:port[/service]
func grpcInjectAddr(existing string, ip string, port int) string {
	portRepl := fmt.Sprintf("${1}:%d${3}", port)
	out := grpcAddrRE.ReplaceAllString(existing, portRepl)

	addrRepl := fmt.Sprintf("%s${2}${3}", ip)
	out = grpcAddrRE.ReplaceAllString(out, addrRepl)

	return out
}

// httpInjectAddr injects a port then an IP into a URL
func httpInjectAddr(url string, ip string, port int) string {
	portRepl := fmt.Sprintf("${1}${2}:%d${4}${5}", port)
	out := httpAddrRE.ReplaceAllString(url, portRepl)

	// Ensure that ipv6 addr is enclosed in brackets (RFC 3986)
	ip = fixIPv6(ip)
	addrRepl := fmt.Sprintf("${1}%s${3}${4}${5}", ip)
	out = httpAddrRE.ReplaceAllString(out, addrRepl)

	return out
}

func fixIPv6(address string) string {
	if strings.Count(address, ":") < 2 {
		return address
	}
	if !strings.HasSuffix(address, "]") {
		address = address + "]"
	}
	if !strings.HasPrefix(address, "[") {
		address = "[" + address
	}
	return address
}

// defaultIfEmpty returns the value if not empty otherwise the default value.
func defaultIfEmpty(val, defaultVal string) string {
	if val != "" {
		return val
	}
	return defaultVal
}

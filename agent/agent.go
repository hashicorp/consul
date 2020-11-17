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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/ae"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
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
	"github.com/hashicorp/consul/logging"
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
	GetLANCoordinate() (lib.CoordinateSet, error)
	Leave() error
	LANMembers() []serf.Member
	LANMembersAllSegments() ([]serf.Member, error)
	LANSegmentMembers(segment string) ([]serf.Member, error)
	LocalMember() serf.Member
	JoinLAN(addrs []string) (n int, err error)
	RemoveFailedNode(node string, prune bool) error
	ResolveToken(secretID string) (acl.Authorizer, error)
	ResolveTokenToIdentity(secretID string) (structs.ACLIdentity, error)
	ResolveTokenAndDefaultMeta(secretID string, entMeta *structs.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error)
	RPC(method string, args interface{}, reply interface{}) error
	UseLegacyACLs() bool
	SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer, replyFn structs.SnapshotReplyFn) error
	Shutdown() error
	Stats() map[string]map[string]string
	ReloadConfig(config *consul.Config) error
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

	// aclMasterAuthorizer is an object that helps manage local ACL enforcement.
	aclMasterAuthorizer acl.Authorizer

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

	// checkTCPs maps the check ID to an associated TCP check
	checkTCPs map[structs.CheckID]*checks.CheckTCP

	// checkGRPCs maps the check ID to an associated GRPC check
	checkGRPCs map[structs.CheckID]*checks.CheckGRPC

	// checkTTLs maps the check ID to an associated check TTL
	checkTTLs map[structs.CheckID]*checks.CheckTTL

	// checkDockers maps the check ID to an associated Docker Exec based check
	checkDockers map[structs.CheckID]*checks.CheckDocker

	// checkAliases maps the check ID to an associated Alias checks
	checkAliases map[structs.CheckID]*checks.CheckAlias

	// exposedPorts tracks listener ports for checks exposed through a proxy
	exposedPorts map[string]int

	// stateLock protects the agent state
	stateLock sync.Mutex

	// dockerClient is the client for performing docker health checks.
	dockerClient *checks.DockerClient

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

	// grpcServer is the server instance used currently to serve xDS API for
	// Envoy.
	grpcServer *grpc.Server

	// tlsConfigurator is the central instance to provide a *tls.Config
	// based on the current consul configuration.
	tlsConfigurator *tlsutil.Configurator

	// httpConnLimiter is used to limit connections to the HTTP server by client
	// IP.
	httpConnLimiter connlimit.Limiter

	// configReloaders are subcomponents that need to be notified on a reload so
	// they can update their internal state.
	configReloaders []ConfigReloader

	// TODO: pass directly to HTTPHandlers and DNSServer once those are passed
	// into Agent, which will allow us to remove this field.
	rpcClientHealth *health.Client

	// enterpriseAgent embeds fields that we only access in consul-enterprise builds
	enterpriseAgent
}

// New process the desired options and creates a new Agent.
// This process will
//   * parse the config given the config Flags
//   * setup logging
//      * using predefined logger given in an option
//        OR
//      * initialize a new logger from the configuration
//        including setting up gRPC logging
//   * initialize telemetry
//   * create a TLS Configurator
//   * build a shared connection pool
//   * create the ServiceManager
//   * setup the NodeID if one isn't provided in the configuration
//   * create the AutoConfig object for future use in fully
//     resolving the configuration
func New(bd BaseDeps) (*Agent, error) {
	a := Agent{
		checkReapAfter:  make(map[structs.CheckID]time.Duration),
		checkMonitors:   make(map[structs.CheckID]*checks.CheckMonitor),
		checkTTLs:       make(map[structs.CheckID]*checks.CheckTTL),
		checkHTTPs:      make(map[structs.CheckID]*checks.CheckHTTP),
		checkTCPs:       make(map[structs.CheckID]*checks.CheckTCP),
		checkGRPCs:      make(map[structs.CheckID]*checks.CheckGRPC),
		checkDockers:    make(map[structs.CheckID]*checks.CheckDocker),
		checkAliases:    make(map[structs.CheckID]*checks.CheckAlias),
		eventCh:         make(chan serf.UserEvent, 1024),
		eventBuf:        make([]*UserEvent, 256),
		joinLANNotifier: &systemd.Notifier{},
		retryJoinCh:     make(chan error),
		shutdownCh:      make(chan struct{}),
		endpoints:       make(map[string]string),

		baseDeps:        bd,
		tokens:          bd.Tokens,
		logger:          bd.Logger,
		tlsConfigurator: bd.TLSConfigurator,
		config:          bd.RuntimeConfig,
		cache:           bd.Cache,
	}

	cacheName := cachetype.HealthServicesName
	if bd.RuntimeConfig.UseStreamingBackend {
		cacheName = cachetype.StreamingHealthServicesName
	}
	a.rpcClientHealth = &health.Client{
		Cache:     bd.Cache,
		NetRPC:    &a,
		CacheName: cacheName,
		// Temporarily until streaming supports all connect events
		CacheNameConnect: cachetype.HealthServicesName,
	}

	a.serviceManager = NewServiceManager(&a)

	// TODO: do this somewhere else, maybe move to newBaseDeps
	var err error
	a.aclMasterAuthorizer, err = initializeACLs(bd.RuntimeConfig.NodeName)
	if err != nil {
		return nil, err
	}

	// We used to do this in the Start method. However it doesn't need to go
	// there any longer. Originally it did because we passed the agent
	// delegate to some of the cache registrations. Now we just
	// pass the agent itself so its safe to move here.
	a.registerCache()

	// TODO: why do we ignore failure to load persisted tokens?
	_ = a.tokens.Load(bd.RuntimeConfig.ACLTokens, a.logger)

	// TODO: pass in a fully populated apiServers into Agent.New
	a.apiServers = NewAPIServers(a.logger)

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

	if err := a.tlsConfigurator.Update(a.config.ToTLSUtilConfig()); err != nil {
		return fmt.Errorf("Failed to load TLS configurations after applying auto-config settings: %w", err)
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
		server, err := consul.NewServer(consulCfg, a.baseDeps.Deps)
		if err != nil {
			return fmt.Errorf("Failed to start Consul server: %v", err)
		}
		a.delegate = server
	} else {
		client, err := consul.NewClient(consulCfg, a.baseDeps.Deps)
		if err != nil {
			return fmt.Errorf("Failed to start Consul client: %v", err)
		}
		a.delegate = client
	}

	// the staggering of the state syncing depends on the cluster size.
	a.sync.ClusterSize = func() int { return len(a.delegate.LANMembers()) }

	// link the state with the consul server/client and the state syncer
	// via callbacks. After several attempts this was easier than using
	// channels since the event notification needs to be non-blocking
	// and that should be hidden in the state syncer implementation.
	a.State.Delegate = a.delegate
	a.State.TriggerSyncChanges = a.sync.SyncChanges.Trigger

	if err := a.baseDeps.AutoConfig.Start(&lib.StopChannelContext{StopCh: a.shutdownCh}); err != nil {
		return fmt.Errorf("AutoConf failed to start certificate monitor: %w", err)
	}
	a.serviceManager.Start()

	// Load checks/services/metadata.
	if err := a.loadServices(c, nil); err != nil {
		return err
	}
	if err := a.loadChecks(c, nil); err != nil {
		return err
	}
	if err := a.loadMetadata(c); err != nil {
		return err
	}

	var intentionDefaultAllow bool
	switch a.config.ACLDefaultPolicy {
	case "allow":
		intentionDefaultAllow = true
	case "deny":
		intentionDefaultAllow = false
	default:
		return fmt.Errorf("unexpected ACL default policy value of %q", a.config.ACLDefaultPolicy)
	}

	// Start the proxy config manager.
	a.proxyConfig, err = proxycfg.NewManager(proxycfg.ManagerConfig{
		Cache:  a.cache,
		Logger: a.logger.Named(logging.ProxyConfig),
		State:  a.State,
		Source: &structs.QuerySource{
			Node:       a.config.NodeName,
			Datacenter: a.config.Datacenter,
			Segment:    a.config.SegmentName,
		},
		DNSConfig: proxycfg.DNSConfig{
			Domain:    a.config.DNSDomain,
			AltDomain: a.config.DNSAltDomain,
		},
		TLSConfigurator:       a.tlsConfigurator,
		IntentionDefaultAllow: intentionDefaultAllow,
	})
	if err != nil {
		return err
	}
	go func() {
		if err := a.proxyConfig.Run(); err != nil {
			a.logger.Error("proxy config manager exited with error", "error", err)
		}
	}()

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

	// Start gRPC server.
	if err := a.listenAndServeGRPC(); err != nil {
		return err
	}

	// register watches
	if err := a.reloadWatches(a.config); err != nil {
		return err
	}

	// start retry join
	go a.retryJoinLAN()
	if a.config.ServerMode {
		go a.retryJoinWAN()
	}

	// DEPRECATED: Warn users if they're emitting deprecated metrics. Remove this warning and the flagged metrics in a
	// future release of Consul.
	if !a.config.Telemetry.DisableCompatOneNine {
		a.logger.Warn("DEPRECATED Backwards compatibility with pre-1.9 metrics enabled. These metrics will be removed in a future version of Consul. Set `telemetry { disable_compat_1.9 = true }` to disable them.")
	}

	return nil
}

// Failed returns a channel which is closed when the first server goroutine exits
// with a non-nil error.
func (a *Agent) Failed() <-chan struct{} {
	return a.apiServers.failed
}

func (a *Agent) listenAndServeGRPC() error {
	if len(a.config.GRPCAddrs) < 1 {
		return nil
	}

	xdsServer := &xds.Server{
		Logger:       a.logger,
		CfgMgr:       a.proxyConfig,
		ResolveToken: a.resolveToken,
		CheckFetcher: a,
		CfgFetcher:   a,
	}
	xdsServer.Initialize()

	var err error
	if a.config.HTTPSPort > 0 {
		// gRPC uses the same TLS settings as the HTTPS API. If HTTPS is
		// enabled then gRPC will require HTTPS as well.
		a.grpcServer, err = xdsServer.GRPCServer(a.tlsConfigurator)
	} else {
		a.grpcServer, err = xdsServer.GRPCServer(nil)
	}
	if err != nil {
		return err
	}

	ln, err := a.startListeners(a.config.GRPCAddrs)
	if err != nil {
		return err
	}

	for _, l := range ln {
		go func(innerL net.Listener) {
			a.logger.Info("Started gRPC server",
				"address", innerL.Addr().String(),
				"network", innerL.Addr().Network(),
			)
			err := a.grpcServer.Serve(innerL)
			if err != nil {
				a.logger.Error("gRPC server failed", "error", err)
			}
		}(l)
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

func (a *Agent) startListeners(addrs []net.Addr) ([]net.Listener, error) {
	var ln []net.Listener
	for _, addr := range addrs {
		var l net.Listener
		var err error

		switch x := addr.(type) {
		case *net.UnixAddr:
			l, err = a.listenSocket(x.Name)
			if err != nil {
				return nil, err
			}

		case *net.TCPAddr:
			l, err = net.Listen("tcp", x.String())
			if err != nil {
				return nil, err
			}
			l = &tcpKeepAliveListener{l.(*net.TCPListener)}

		default:
			return nil, fmt.Errorf("unsupported address type %T", addr)
		}
		ln = append(ln, l)
	}
	return ln, nil
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
			if isTCP && proto == "https" {
				tlscfg = a.tlsConfigurator.IncomingHTTPSConfig()
				l = tls.NewListener(l, tlscfg)
			}

			srv := &HTTPHandlers{
				agent:    a,
				denylist: NewDenylist(a.config.HTTPBlockEndpoints),
			}
			a.configReloaders = append(a.configReloaders, srv.ReloadConfig)
			a.httpHandlers = srv
			httpServer := &http.Server{
				Addr:      l.Addr().String(),
				TLSConfig: tlscfg,
				Handler:   srv.handler(a.config.EnableDebug),
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

			servers = append(servers, apiServer{
				Protocol: proto,
				Addr:     l.Addr(),
				Shutdown: httpServer.Shutdown,
				Run: func() error {
					err := httpServer.Serve(l)
					if err == nil || err == http.ErrServerClosed {
						return nil
					}
					return fmt.Errorf("%s server %s failed: %w", proto, l.Addr(), err)
				},
			})
		}
		return nil
	}

	if err := start("http", a.config.HTTPAddrs); err != nil {
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
	cfg.SerfLANConfig.MemberlistConfig.GossipVerifyIncoming = runtimeCfg.EncryptVerifyIncoming
	cfg.SerfLANConfig.MemberlistConfig.GossipVerifyOutgoing = runtimeCfg.EncryptVerifyOutgoing
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
		cfg.SerfWANConfig.MemberlistConfig.GossipVerifyIncoming = runtimeCfg.EncryptVerifyIncoming
		cfg.SerfWANConfig.MemberlistConfig.GossipVerifyOutgoing = runtimeCfg.EncryptVerifyOutgoing
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
	if runtimeCfg.ACLMasterToken != "" {
		cfg.ACLMasterToken = runtimeCfg.ACLMasterToken
	}
	if runtimeCfg.ACLDatacenter != "" {
		cfg.ACLDatacenter = runtimeCfg.ACLDatacenter
	}
	if runtimeCfg.ACLTokenTTL != 0 {
		cfg.ACLTokenTTL = runtimeCfg.ACLTokenTTL
	}
	if runtimeCfg.ACLPolicyTTL != 0 {
		cfg.ACLPolicyTTL = runtimeCfg.ACLPolicyTTL
	}
	if runtimeCfg.ACLRoleTTL != 0 {
		cfg.ACLRoleTTL = runtimeCfg.ACLRoleTTL
	}
	if runtimeCfg.ACLDefaultPolicy != "" {
		cfg.ACLDefaultPolicy = runtimeCfg.ACLDefaultPolicy
	}
	if runtimeCfg.ACLDownPolicy != "" {
		cfg.ACLDownPolicy = runtimeCfg.ACLDownPolicy
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
		cfg.RPCRate = runtimeCfg.RPCRateLimit
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
	cfg.Build = fmt.Sprintf("%s%s:%s", runtimeCfg.Version, runtimeCfg.VersionPrerelease, revision)

	// Copy the TLS configuration
	cfg.VerifyIncoming = runtimeCfg.VerifyIncoming || runtimeCfg.VerifyIncomingRPC
	if runtimeCfg.CAPath != "" || runtimeCfg.CAFile != "" {
		cfg.UseTLS = true
	}
	cfg.VerifyOutgoing = runtimeCfg.VerifyOutgoing
	cfg.VerifyServerHostname = runtimeCfg.VerifyServerHostname
	cfg.CAFile = runtimeCfg.CAFile
	cfg.CAPath = runtimeCfg.CAPath
	cfg.CertFile = runtimeCfg.CertFile
	cfg.KeyFile = runtimeCfg.KeyFile
	cfg.ServerName = runtimeCfg.ServerName
	cfg.Domain = runtimeCfg.DNSDomain
	cfg.TLSMinVersion = runtimeCfg.TLSMinVersion
	cfg.TLSCipherSuites = runtimeCfg.TLSCipherSuites
	cfg.TLSPreferServerCipherSuites = runtimeCfg.TLSPreferServerCipherSuites
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

	enterpriseConsulConfig(cfg, runtimeCfg)
	return cfg, nil
}

// Setup the serf and memberlist config for any defined network segments.
func segmentConfig(config *config.RuntimeConfig) ([]consul.NetworkSegment, error) {
	var segments []consul.NetworkSegment

	for _, s := range config.Segments {
		serfConf := consul.DefaultConfig().SerfLANConfig

		serfConf.MemberlistConfig.BindAddr = s.Bind.IP.String()
		serfConf.MemberlistConfig.BindPort = s.Bind.Port
		serfConf.MemberlistConfig.AdvertiseAddr = s.Advertise.IP.String()
		serfConf.MemberlistConfig.AdvertisePort = s.Advertise.Port

		if config.ReconnectTimeoutLAN != 0 {
			serfConf.ReconnectTimeout = config.ReconnectTimeoutLAN
		}
		if config.EncryptVerifyIncoming {
			serfConf.MemberlistConfig.GossipVerifyIncoming = config.EncryptVerifyIncoming
		}
		if config.EncryptVerifyOutgoing {
			serfConf.MemberlistConfig.GossipVerifyOutgoing = config.EncryptVerifyOutgoing
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

	// this would be cancelled anyways (by the closing of the shutdown ch) but
	// this should help them to be stopped more quickly
	a.baseDeps.AutoConfig.Stop()

	// Stop the service manager (must happen before we take the stateLock to avoid deadlock)
	if a.serviceManager != nil {
		a.serviceManager.Stop()
	}

	// Stop all the checks
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
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
	for _, chk := range a.checkGRPCs {
		chk.Stop()
	}
	for _, chk := range a.checkDockers {
		chk.Stop()
	}
	for _, chk := range a.checkAliases {
		chk.Stop()
	}

	// Stop gRPC
	if a.grpcServer != nil {
		a.grpcServer.Stop()
	}

	// Stop the proxy config manager
	if a.proxyConfig != nil {
		a.proxyConfig.Close()
	}

	// Stop the cache background work
	if a.cache != nil {
		a.cache.Close()
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
func (a *Agent) JoinLAN(addrs []string) (n int, err error) {
	a.logger.Info("(LAN) joining", "lan_addresses", addrs)
	n, err = a.delegate.JoinLAN(addrs)
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
func (a *Agent) ForceLeave(node string, prune bool) (err error) {
	a.logger.Info("Force leaving node", "node", node)
	if ok := a.IsMember(node); !ok {
		return fmt.Errorf("agent: No node found with name '%s'", node)
	}
	err = a.delegate.RemoveFailedNode(node, prune)
	if err != nil {
		a.logger.Warn("Failed to remove node",
			"node", node,
			"error", err,
		)
	}
	return err
}

// LocalMember is used to return the local node
func (a *Agent) LocalMember() serf.Member {
	return a.delegate.LocalMember()
}

// LANMembers is used to retrieve the LAN members
func (a *Agent) LANMembers() []serf.Member {
	return a.delegate.LANMembers()
}

// WANMembers is used to retrieve the WAN members
func (a *Agent) WANMembers() []serf.Member {
	if srv, ok := a.delegate.(*consul.Server); ok {
		return srv.WANMembers()
	}
	return nil
}

// IsMember is used to check if a node with the given nodeName
// is a member
func (a *Agent) IsMember(nodeName string) bool {
	for _, m := range a.LANMembers() {
		if m.Name == nodeName {
			return true
		}
	}

	return false
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
		intv := lib.RateScaledInterval(rate, min, len(a.LANMembers()))
		intv = intv + lib.RandomStagger(intv)

		select {
		case <-time.After(intv):
			members := a.LANMembers()
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
					Datacenter:   a.config.Datacenter,
					Node:         a.config.NodeName,
					Segment:      segment,
					Coord:        coord,
					WriteRequest: structs.WriteRequest{Token: agentToken},
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
	for checkID, cs := range a.State.CriticalCheckStates(structs.WildcardEnterpriseMeta()) {
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
				a.logger.Error("unable to deregister service after check has been critical for too long",
					"service", serviceID.String(),
					"check", checkID.String(),
					"error", err)
			} else {
				a.logger.Info("Check for service has been critical for too long; deregistered service",
					"service", serviceID.String(),
					"check", checkID.String(),
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

// persistService saves a service definition to a JSON file in the data dir
func (a *Agent) persistService(service *structs.NodeService, source configSource) error {
	svcID := service.CompoundServiceID()
	svcPath := filepath.Join(a.config.DataDir, servicesDir, svcID.StringHash())

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
	svcPath := filepath.Join(a.config.DataDir, servicesDir, serviceID.StringHash())
	if _, err := os.Stat(svcPath); err == nil {
		return os.Remove(svcPath)
	}
	return nil
}

// persistCheck saves a check definition to the local agent's state directory
func (a *Agent) persistCheck(check *structs.HealthCheck, chkType *structs.CheckType, source configSource) error {
	cid := check.CompoundCheckID()
	checkPath := filepath.Join(a.config.DataDir, checksDir, cid.StringHash())

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
	checkPath := filepath.Join(a.config.DataDir, checksDir, checkID.StringHash())
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
	structs.EnterpriseMeta
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
	configPath := filepath.Join(dir, serviceID.StringHash())

	// Create the config dir if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating service configs dir %q: %s", dir, err)
	}

	return file.WriteAtomic(configPath, encoded)
}

func (a *Agent) purgeServiceConfig(serviceID structs.ServiceID) error {
	configPath := filepath.Join(a.config.DataDir, serviceConfigDir, serviceID.StringHash())
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
			return nil, fmt.Errorf("failed reading service config file %q: %s", file, err)
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
		out[structs.NewServiceID(p.ServiceID, &p.EnterpriseMeta)] = p.Defaults
	}

	return out, nil
}

// AddServiceAndReplaceChecks is used to add a service entry and its check. Any check for this service missing from chkTypes will be deleted.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddServiceAndReplaceChecks(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.addServiceLocked(&addServiceRequest{
		service:               service,
		chkTypes:              chkTypes,
		previousDefaults:      nil,
		waitForCentralConfig:  true,
		persist:               persist,
		persistServiceConfig:  true,
		token:                 token,
		replaceExistingChecks: true,
		source:                source,
		snap:                  a.snapshotCheckState(),
	})
}

// AddService is used to add a service entry.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddService(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.addServiceLocked(&addServiceRequest{
		service:               service,
		chkTypes:              chkTypes,
		previousDefaults:      nil,
		waitForCentralConfig:  true,
		persist:               persist,
		persistServiceConfig:  true,
		token:                 token,
		replaceExistingChecks: false,
		source:                source,
		snap:                  a.snapshotCheckState(),
	})
}

// addServiceLocked adds a service entry to the service manager if enabled, or directly
// to the local state if it is not. This function assumes the state lock is already held.
func (a *Agent) addServiceLocked(req *addServiceRequest) error {
	req.fixupForAddServiceLocked()

	req.service.EnterpriseMeta.Normalize()

	if err := a.validateService(req.service, req.chkTypes); err != nil {
		return err
	}

	if a.config.EnableCentralServiceConfig {
		return a.serviceManager.AddService(req)
	}

	// previousDefaults are ignored here because they are only relevant for central config.
	req.persistService = nil
	req.persistDefaults = nil
	req.persistServiceConfig = false

	return a.addServiceInternal(req)
}

// addServiceRequest is the union of arguments for calling both
// addServiceLocked and addServiceInternal. The overlap was significant enough
// to warrant merging them and indicating which fields are meant to be set only
// in one of the two contexts.
//
// Before using the request struct one of the fixupFor*() methods should be
// invoked to clear irrelevant fields.
//
// The ServiceManager.AddService signature is largely just a passthrough for
// addServiceLocked and should be treated as such.
type addServiceRequest struct {
	service               *structs.NodeService
	chkTypes              []*structs.CheckType
	previousDefaults      *structs.ServiceConfigResponse // just for: addServiceLocked
	waitForCentralConfig  bool                           // just for: addServiceLocked
	persistService        *structs.NodeService           // just for: addServiceInternal
	persistDefaults       *structs.ServiceConfigResponse // just for: addServiceInternal
	persist               bool
	persistServiceConfig  bool
	token                 string
	replaceExistingChecks bool
	source                configSource
	snap                  map[structs.CheckID]*structs.HealthCheck
}

func (r *addServiceRequest) fixupForAddServiceLocked() {
	r.persistService = nil
	r.persistDefaults = nil
}

func (r *addServiceRequest) fixupForAddServiceInternal() {
	r.previousDefaults = nil
	r.waitForCentralConfig = false
}

// addServiceInternal adds the given service and checks to the local state.
func (a *Agent) addServiceInternal(req *addServiceRequest) error {
	req.fixupForAddServiceInternal()
	var (
		service               = req.service
		chkTypes              = req.chkTypes
		persistService        = req.persistService
		persistDefaults       = req.persistDefaults
		persist               = req.persist
		persistServiceConfig  = req.persistServiceConfig
		token                 = req.token
		replaceExistingChecks = req.replaceExistingChecks
		source                = req.source
		snap                  = req.snap
	)

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

	// Create an associated health check
	for i, chkType := range chkTypes {
		checkID := string(chkType.CheckID)
		if checkID == "" {
			checkID = fmt.Sprintf("service:%s", service.ID)
			if len(chkTypes) > 1 {
				checkID += fmt.Sprintf(":%d", i+1)
			}
		}

		cid := structs.NewCheckID(types.CheckID(checkID), &service.EnterpriseMeta)
		existingChecks[cid] = true

		name := chkType.Name
		if name == "" {
			name = fmt.Sprintf("Service '%s' check", service.Service)
		}
		check := &structs.HealthCheck{
			Node:           a.config.NodeName,
			CheckID:        types.CheckID(checkID),
			Name:           name,
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
		prev, ok := snap[cid]
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

	err := a.State.AddServiceWithChecks(service, checks, token)
	if err != nil {
		a.cleanupRegistration(cleanupServices, cleanupChecks)
		return err
	}

	for i := range checks {
		if err := a.addCheck(checks[i], chkTypes[i], service, token, source); err != nil {
			a.cleanupRegistration(cleanupServices, cleanupChecks)
			return err
		}

		if persist && a.config.DataDir != "" {
			if err := a.persistCheck(checks[i], chkTypes[i], source); err != nil {
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

	if persistServiceConfig && a.config.DataDir != "" {
		var err error
		if persistDefaults != nil {
			err = a.persistServiceConfig(service.CompoundServiceID(), persistDefaults)
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
		if persistService == nil {
			persistService = service
		}

		if err := a.persistService(persistService, source); err != nil {
			a.cleanupRegistration(cleanupServices, cleanupChecks)
			return err
		}
	}

	if replaceExistingChecks {
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
	sidecarSID := structs.NewServiceID(a.sidecarServiceID(serviceID.ID), &serviceID.EnterpriseMeta)
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

		// Get the address of the proxy for this service if it exists
		// Need its config to know whether we should reroute checks to it
		var proxy *structs.NodeService
		if service != nil {
			for _, svc := range a.State.Services(&service.EnterpriseMeta) {
				if svc.Proxy.DestinationServiceID == service.ID {
					proxy = svc
					break
				}
			}
		}

		statusHandler := checks.NewStatusHandler(a.State, a.logger, chkType.SuccessBeforePassing, chkType.FailuresBeforeCritical)
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

			tlsClientConfig := a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify)

			http := &checks.CheckHTTP{
				CheckID:         cid,
				ServiceID:       sid,
				HTTP:            chkType.HTTP,
				Header:          chkType.Header,
				Method:          chkType.Method,
				Body:            chkType.Body,
				Interval:        chkType.Interval,
				Timeout:         chkType.Timeout,
				Logger:          a.logger,
				OutputMaxSize:   maxOutputSize,
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
				http.ProxyHTTP = httpInjectAddr(http.HTTP, proxy.Address, port)
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
				tlsClientConfig = a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify)
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
			if prev := a.checkDockers[cid]; prev != nil {
				prev.Stop()
			}
			dockerCheck.Start()
			a.checkDockers[cid] = dockerCheck

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

		case chkType.IsAlias():
			if existing, ok := a.checkAliases[cid]; ok {
				existing.Stop()
				delete(a.checkAliases, cid)
			}

			var rpcReq structs.NodeSpecificRequest
			rpcReq.Datacenter = a.config.Datacenter

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

// resolveProxyCheckAddress returns the best address to use for a TCP check of
// the proxy's public listener. It expects the input to already have default
// values populated by applyProxyConfigDefaults. It may return an empty string
// indicating that the TCP check should not be created at all.
//
// By default this uses the proxy's bind address which in turn defaults to the
// agent's bind address. If the proxy bind address ends up being 0.0.0.0 we have
// to assume the agent can dial it over loopback which is usually true.
//
// In some topologies such as proxy being in a different container, the IP the
// agent used to dial proxy over a local bridge might not be the same as the
// container's public routable IP address so we allow a manual override of the
// check address in config "tcp_check_address" too.
//
// Finally the TCP check can be disabled by another manual override
// "disable_tcp_check" in cases where the agent will never be able to dial the
// proxy directly for some reason.
func (a *Agent) resolveProxyCheckAddress(proxyCfg map[string]interface{}) string {
	// If user disabled the check return empty string
	if disable, ok := proxyCfg["disable_tcp_check"].(bool); ok && disable {
		return ""
	}

	// If user specified a custom one, use that
	if chkAddr, ok := proxyCfg["tcp_check_address"].(string); ok && chkAddr != "" {
		return chkAddr
	}

	// If we have a bind address and its diallable, use that
	if bindAddr, ok := proxyCfg["bind_address"].(string); ok &&
		bindAddr != "" && bindAddr != "0.0.0.0" && bindAddr != "[::]" {
		return bindAddr
	}

	// Default to localhost
	return "127.0.0.1"
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
	file := filepath.Join(dir, check.CheckID.StringHash())

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
	file := filepath.Join(a.config.DataDir, checkStateDir, cid.StringHash())
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed reading file %q: %s", file, err)
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
	file := filepath.Join(a.config.DataDir, checkStateDir, checkID.StringHash())
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
		"revision":   revision,
		"version":    a.config.Version,
		"prerelease": a.config.VersionPrerelease,
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
		ns := service.NodeService()
		chkTypes, err := service.CheckTypes()
		if err != nil {
			return fmt.Errorf("Failed to validate checks for service %q: %v", service.Name, err)
		}

		// Grab and validate sidecar if there is one too
		sidecar, sidecarChecks, sidecarToken, err := a.sidecarServiceFromNodeService(ns, service.Token)
		if err != nil {
			return fmt.Errorf("Failed to validate sidecar for service %q: %v", service.Name, err)
		}

		// Remove sidecar from NodeService now it's done it's job it's just a config
		// syntax sugar and shouldn't be persisted in local or server state.
		ns.Connect.SidecarService = nil

		sid := ns.CompoundServiceID()
		err = a.addServiceLocked(&addServiceRequest{
			service:               ns,
			chkTypes:              chkTypes,
			previousDefaults:      persistedServiceConfigs[sid],
			waitForCentralConfig:  false, // exclusively use cached values
			persist:               false, // don't rewrite the file with the same data we just read
			persistServiceConfig:  false, // don't rewrite the file with the same data we just read
			token:                 service.Token,
			replaceExistingChecks: false, // do default behavior
			source:                ConfigSourceLocal,
			snap:                  snap,
		})
		if err != nil {
			return fmt.Errorf("Failed to register service %q: %v", service.Name, err)
		}

		// If there is a sidecar service, register that too.
		if sidecar != nil {
			sidecarServiceID := sidecar.CompoundServiceID()
			err = a.addServiceLocked(&addServiceRequest{
				service:               sidecar,
				chkTypes:              sidecarChecks,
				previousDefaults:      persistedServiceConfigs[sidecarServiceID],
				waitForCentralConfig:  false, // exclusively use cached values
				persist:               false, // don't rewrite the file with the same data we just read
				persistServiceConfig:  false, // don't rewrite the file with the same data we just read
				token:                 sidecarToken,
				replaceExistingChecks: false, // do default behavior
				source:                ConfigSourceLocal,
				snap:                  snap,
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
		return fmt.Errorf("Failed reading services dir %q: %s", svcDir, err)
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
			return fmt.Errorf("failed reading service file %q: %s", file, err)
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
				return fmt.Errorf("failed purging service %q: %s", serviceID, err)
			}
			if err := a.purgeServiceConfig(serviceID); err != nil {
				return fmt.Errorf("failed purging service config %q: %s", serviceID, err)
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
				return fmt.Errorf("failed purging service %q: %s", serviceID.String(), err)
			}
			if err := a.purgeServiceConfig(serviceID); err != nil {
				return fmt.Errorf("failed purging service config %q: %s", serviceID.String(), err)
			}
		} else {
			a.logger.Debug("restored service definition from file",
				"service", serviceID.String(),
				"file", file,
			)
			err = a.addServiceLocked(&addServiceRequest{
				service:               p.Service,
				chkTypes:              nil,
				previousDefaults:      persistedServiceConfigs[serviceID],
				waitForCentralConfig:  false, // exclusively use cached values
				persist:               false, // don't rewrite the file with the same data we just read
				persistServiceConfig:  false, // don't rewrite the file with the same data we just read
				token:                 p.Token,
				replaceExistingChecks: false, // do default behavior
				source:                source,
				snap:                  snap,
			})
			if err != nil {
				return fmt.Errorf("failed adding service %q: %s", serviceID, err)
			}
		}
	}

	for serviceID := range persistedServiceConfigs {
		if a.State.Service(serviceID) == nil {
			// This can be cleaned up now.
			if err := a.purgeServiceConfig(serviceID); err != nil {
				return fmt.Errorf("failed purging service config %q: %s", serviceID, err)
			}
		}
	}

	return nil
}

// unloadServices will deregister all services.
func (a *Agent) unloadServices() error {
	for id := range a.State.Services(structs.WildcardEnterpriseMeta()) {
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
		return fmt.Errorf("Failed reading checks dir %q: %s", checkDir, err)
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
			return fmt.Errorf("failed reading check file %q: %s", file, err)
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

		source, ok := ConfigSourceFromName(p.Source)
		if !ok {
			a.logger.Warn("check exists with invalid source, purging",
				"check", checkID.String(),
				"source", p.Source,
			)
			if err := a.purgeCheck(checkID); err != nil {
				return fmt.Errorf("failed purging check %q: %s", checkID, err)
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
				return fmt.Errorf("Failed purging check %q: %s", checkID, err)
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
					return fmt.Errorf("Failed purging check %q: %s", checkID, err)
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
	for id := range a.State.Checks(structs.WildcardEnterpriseMeta()) {
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
	return a.State.Checks(structs.WildcardEnterpriseMeta())
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

func (a *Agent) loadLimits(conf *config.RuntimeConfig) {
	a.config.RPCRateLimit = conf.RPCRateLimit
	a.config.RPCMaxBurst = conf.RPCMaxBurst
}

// ReloadConfig will atomically reload all configuration, including
// all services, checks, tokens, metadata, dnsServer configs, etc.
// It will also reload all ongoing watches.
func (a *Agent) ReloadConfig() error {
	newCfg, err := a.baseDeps.AutoConfig.ReadConfig()
	if err != nil {
		return err
	}

	// copy over the existing node id, this cannot be
	// changed while running anyways but this prevents
	// breaking some existing behavior.
	newCfg.NodeID = a.config.NodeID

	// DEPRECATED: Warn users on reload if they're emitting deprecated metrics. Remove this warning and the flagged
	// metrics in a future release of Consul.
	if !a.config.Telemetry.DisableCompatOneNine {
		a.logger.Warn("DEPRECATED Backwards compatibility with pre-1.9 metrics enabled. These metrics will be removed in a future version of Consul. Set `telemetry { disable_compat_1.9 = true }` to disable them.")
	}

	return a.reloadConfigInternal(newCfg)
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

	if err := a.tlsConfigurator.Update(newCfg.ToTLSUtilConfig()); err != nil {
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

	a.loadLimits(newCfg)

	a.httpConnLimiter.SetConfig(connlimit.Config{
		MaxConnsPerClientIP: newCfg.HTTPMaxConnsPerClient,
	})

	for _, s := range a.dnsServers {
		if err := s.ReloadConfig(newCfg); err != nil {
			return fmt.Errorf("Failed reloading dns config : %v", err)
		}
	}

	// this only gets used by the consulConfig function and since
	// that is only ever done during init and reload here then
	// an in place modification is safe as reloads cannot be
	// concurrent due to both gaining a full lock on the stateLock
	a.config.ConfigEntryBootstrap = newCfg.ConfigEntryBootstrap

	err := a.reloadEnterprise(newCfg)
	if err != nil {
		return err
	}

	// create the config for the rpc server/client
	consulCfg, err := newConsulConfig(a.config, a.logger)
	if err != nil {
		return err
	}

	if err := a.delegate.ReloadConfig(consulCfg); err != nil {
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

	a.cache.RegisterType(cachetype.ConfigEntriesName, &cachetype.ConfigEntries{RPC: a})

	a.cache.RegisterType(cachetype.ConfigEntryName, &cachetype.ConfigEntry{RPC: a})

	a.cache.RegisterType(cachetype.ServiceHTTPChecksName, &cachetype.ServiceHTTPChecks{Agent: a})

	a.cache.RegisterType(cachetype.FederationStateListMeshGatewaysName,
		&cachetype.FederationStateListMeshGateways{RPC: a})
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
			ids = append(ids, cid)
		}
	}
	for cid, c := range a.checkGRPCs {
		if c.ServiceID == serviceID {
			c.ProxyGRPC = ""
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

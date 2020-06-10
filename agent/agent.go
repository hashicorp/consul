package agent

import (
	"context"
	"crypto/sha512"
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

	"github.com/hashicorp/go-connlimit"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"google.golang.org/grpc"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/ae"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/proxycfg"
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
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/shirou/gopsutil/host"
	"golang.org/x/net/http2"
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

	// Name of the file tokens will be persisted within
	tokensPath = "acl-tokens.json"

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
	ACLsEnabled() bool
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
	// config is the agent configuration.
	config *config.RuntimeConfig

	// Used for writing our logs
	logger hclog.InterceptLogger

	// LogOutput is a Writer which is used when creating dependencies that
	// require logging. Note that this LogOutput is not used by the agent logger,
	// so setting this field does not result in the agent logs being written to
	// LogOutput.
	// FIXME: refactor so that: dependencies accept an hclog.Logger,
	// or LogOutput is part of RuntimeConfig, or change Agent.logger to be
	// a new type with an Out() io.Writer method which returns this value.
	LogOutput io.Writer

	// In-memory sink used for collecting metrics
	MemSink *metrics.InmemSink

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

	reloadCh chan chan error

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex

	InterruptStartCh chan struct{}

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

	// httpServers provides the HTTP API on various endpoints
	httpServers []*HTTPServer

	// wgServers is the wait group for all HTTP and DNS servers
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

	// persistedTokensLock is used to synchronize access to the persisted token
	// store within the data directory. This will prevent loading while writing as
	// well as multiple concurrent writes.
	persistedTokensLock sync.RWMutex

	// httpConnLimiter is used to limit connections to the HTTP server by client
	// IP.
	httpConnLimiter connlimit.Limiter

	// Connection Pool
	connPool *pool.ConnPool

	// enterpriseAgent embeds fields that we only access in consul-enterprise builds
	enterpriseAgent
}

// New verifies the configuration given has a Datacenter and DataDir
// configured, and maps the remaining config fields to fields on the Agent.
func New(c *config.RuntimeConfig, logger hclog.InterceptLogger) (*Agent, error) {
	if c.Datacenter == "" {
		return nil, fmt.Errorf("Must configure a Datacenter")
	}
	if c.DataDir == "" && !c.DevMode {
		return nil, fmt.Errorf("Must configure a DataDir")
	}

	tlsConfigurator, err := tlsutil.NewConfigurator(c.ToTLSUtilConfig(), logger)
	if err != nil {
		return nil, err
	}

	a := Agent{
		config:           c,
		checkReapAfter:   make(map[structs.CheckID]time.Duration),
		checkMonitors:    make(map[structs.CheckID]*checks.CheckMonitor),
		checkTTLs:        make(map[structs.CheckID]*checks.CheckTTL),
		checkHTTPs:       make(map[structs.CheckID]*checks.CheckHTTP),
		checkTCPs:        make(map[structs.CheckID]*checks.CheckTCP),
		checkGRPCs:       make(map[structs.CheckID]*checks.CheckGRPC),
		checkDockers:     make(map[structs.CheckID]*checks.CheckDocker),
		checkAliases:     make(map[structs.CheckID]*checks.CheckAlias),
		eventCh:          make(chan serf.UserEvent, 1024),
		eventBuf:         make([]*UserEvent, 256),
		joinLANNotifier:  &systemd.Notifier{},
		reloadCh:         make(chan chan error),
		retryJoinCh:      make(chan error),
		shutdownCh:       make(chan struct{}),
		InterruptStartCh: make(chan struct{}),
		endpoints:        make(map[string]string),
		tokens:           new(token.Store),
		logger:           logger,
		tlsConfigurator:  tlsConfigurator,
	}
	err = a.initializeConnectionPool()
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize the connection pool: %w", err)
	}

	a.serviceManager = NewServiceManager(&a)

	if err := a.initializeACLs(); err != nil {
		return nil, err
	}

	// Retrieve or generate the node ID before setting up the rest of the
	// agent, which depends on it.
	if err := a.setupNodeID(c); err != nil {
		return nil, fmt.Errorf("Failed to setup node ID: %v", err)
	}

	return &a, nil
}

func (a *Agent) initializeConnectionPool() error {
	var rpcSrcAddr *net.TCPAddr
	if !ipaddr.IsAny(a.config.RPCBindAddr) {
		rpcSrcAddr = &net.TCPAddr{IP: a.config.RPCBindAddr.IP}
	}

	// Ensure we have a log output for the connection pool.
	logOutput := a.LogOutput
	if logOutput == nil {
		logOutput = os.Stderr
	}

	pool := &pool.ConnPool{
		Server:          a.config.ServerMode,
		SrcAddr:         rpcSrcAddr,
		LogOutput:       logOutput,
		TLSConfigurator: a.tlsConfigurator,
		Datacenter:      a.config.Datacenter,
	}
	if a.config.ServerMode {
		pool.MaxTime = 2 * time.Minute
		pool.MaxStreams = 64
	} else {
		pool.MaxTime = 127 * time.Second
		pool.MaxStreams = 32
	}

	a.connPool = pool
	return nil
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
func (a *Agent) Start() error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	c := a.config

	if err := a.CheckSecurity(c); err != nil {
		a.logger.Error("Security error while parsing configuration: %#v", err)
		return err
	}

	// Warn if the node name is incompatible with DNS
	if InvalidDnsRe.MatchString(a.config.NodeName) {
		a.logger.Warn("Node name will not be discoverable "+
			"via DNS due to invalid characters. Valid characters include "+
			"all alpha-numerics and dashes.",
			"node_name", a.config.NodeName,
		)
	} else if len(a.config.NodeName) > MaxDNSLabelLength {
		a.logger.Warn("Node name will not be discoverable "+
			"via DNS due to it being too long. Valid lengths are between "+
			"1 and 63 bytes.",
			"node_name", a.config.NodeName,
		)
	}

	// load the tokens - this requires the logger to be setup
	// which is why we can't do this in New
	a.loadTokens(a.config)
	a.loadEnterpriseTokens(a.config)

	// create the local state
	a.State = local.NewState(LocalConfig(c), a.logger, a.tokens)

	// create the state synchronization manager which performs
	// regular and on-demand state synchronizations (anti-entropy).
	a.sync = ae.NewStateSyncer(a.State, c.AEInterval, a.shutdownCh, a.logger)

	// create the cache
	a.cache = cache.New(nil)

	// create the config for the rpc server/client
	consulCfg, err := a.consulConfig()
	if err != nil {
		return err
	}

	// ServerUp is used to inform that a new consul server is now
	// up. This can be used to speed up the sync process if we are blocking
	// waiting to discover a consul server
	consulCfg.ServerUp = a.sync.SyncFull.Trigger

	err = a.initEnterprise(consulCfg)
	if err != nil {
		return fmt.Errorf("failed to start Consul enterprise component: %v", err)
	}

	options := []consul.ConsulOption{
		consul.WithLogger(a.logger),
		consul.WithTokenStore(a.tokens),
		consul.WithTLSConfigurator(a.tlsConfigurator),
		consul.WithConnectionPool(a.connPool),
	}

	// Setup either the client or the server.
	if c.ServerMode {
		server, err := consul.NewServerWithOptions(consulCfg, options...)
		if err != nil {
			return fmt.Errorf("Failed to start Consul server: %v", err)
		}
		a.delegate = server
	} else {
		client, err := consul.NewClientWithOptions(consulCfg, options...)
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

	// Register the cache. We do this much later so the delegate is
	// populated from above.
	a.registerCache()

	if a.config.AutoEncryptTLS && !a.config.ServerMode {
		reply, err := a.setupClientAutoEncrypt()
		if err != nil {
			return fmt.Errorf("AutoEncrypt failed: %s", err)
		}
		rootsReq, leafReq, err := a.setupClientAutoEncryptCache(reply)
		if err != nil {
			return fmt.Errorf("AutoEncrypt failed: %s", err)
		}
		if err = a.setupClientAutoEncryptWatching(rootsReq, leafReq); err != nil {
			return fmt.Errorf("AutoEncrypt failed: %s", err)
		}
		a.logger.Info("automatically upgraded to TLS")
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
		TLSConfigurator: a.tlsConfigurator,
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
		if err := a.serveHTTP(srv); err != nil {
			return err
		}
		a.httpServers = append(a.httpServers, srv)
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

	return nil
}

func (a *Agent) setupClientAutoEncrypt() (*structs.SignedResponse, error) {
	client := a.delegate.(*consul.Client)

	addrs := a.config.StartJoinAddrsLAN
	disco, err := newDiscover()
	if err != nil && len(addrs) == 0 {
		return nil, err
	}
	addrs = append(addrs, retryJoinAddrs(disco, retryJoinSerfVariant, "LAN", a.config.RetryJoinLAN, a.logger)...)

	reply, priv, err := client.RequestAutoEncryptCerts(addrs, a.config.ServerPort, a.tokens.AgentToken(), a.InterruptStartCh)
	if err != nil {
		return nil, err
	}

	connectCAPems := []string{}
	for _, ca := range reply.ConnectCARoots.Roots {
		connectCAPems = append(connectCAPems, ca.RootCert)
	}
	if err := a.tlsConfigurator.UpdateAutoEncrypt(reply.ManualCARoots, connectCAPems, reply.IssuedCert.CertPEM, priv, reply.VerifyServerHostname); err != nil {
		return nil, err
	}
	return reply, nil

}

func (a *Agent) setupClientAutoEncryptCache(reply *structs.SignedResponse) (*structs.DCSpecificRequest, *cachetype.ConnectCALeafRequest, error) {
	rootsReq := &structs.DCSpecificRequest{
		Datacenter:   a.config.Datacenter,
		QueryOptions: structs.QueryOptions{Token: a.tokens.AgentToken()},
	}

	// prepolutate roots cache
	rootRes := cache.FetchResult{Value: &reply.ConnectCARoots, Index: reply.ConnectCARoots.QueryMeta.Index}
	if err := a.cache.Prepopulate(cachetype.ConnectCARootName, rootRes, a.config.Datacenter, a.tokens.AgentToken(), rootsReq.CacheInfo().Key); err != nil {
		return nil, nil, err
	}

	leafReq := &cachetype.ConnectCALeafRequest{
		Datacenter: a.config.Datacenter,
		Token:      a.tokens.AgentToken(),
		Agent:      a.config.NodeName,
		DNSSAN:     a.config.AutoEncryptDNSSAN,
		IPSAN:      a.config.AutoEncryptIPSAN,
	}

	// prepolutate leaf cache
	certRes := cache.FetchResult{Value: &reply.IssuedCert, Index: reply.ConnectCARoots.QueryMeta.Index}
	if err := a.cache.Prepopulate(cachetype.ConnectCALeafName, certRes, a.config.Datacenter, a.tokens.AgentToken(), leafReq.Key()); err != nil {
		return nil, nil, err
	}
	return rootsReq, leafReq, nil
}

func (a *Agent) setupClientAutoEncryptWatching(rootsReq *structs.DCSpecificRequest, leafReq *cachetype.ConnectCALeafRequest) error {
	// setup watches
	ch := make(chan cache.UpdateEvent, 10)
	ctx, cancel := context.WithCancel(context.Background())

	// Watch for root changes
	err := a.cache.Notify(ctx, cachetype.ConnectCARootName, rootsReq, rootsWatchID, ch)
	if err != nil {
		cancel()
		return err
	}

	// Watch the leaf cert
	err = a.cache.Notify(ctx, cachetype.ConnectCALeafName, leafReq, leafWatchID, ch)
	if err != nil {
		cancel()
		return err
	}

	// Setup actions in case the watches are firing.
	go func() {
		for {
			select {
			case <-a.shutdownCh:
				cancel()
				return
			case <-ctx.Done():
				return
			case u := <-ch:
				switch u.CorrelationID {
				case rootsWatchID:
					roots, ok := u.Result.(*structs.IndexedCARoots)
					if !ok {
						err := fmt.Errorf("invalid type for roots response: %T", u.Result)
						a.logger.Error("watch error for correlation id",
							"correlation_id", u.CorrelationID,
							"error", err,
						)
						continue
					}
					pems := []string{}
					for _, root := range roots.Roots {
						pems = append(pems, root.RootCert)
					}
					a.tlsConfigurator.UpdateAutoEncryptCA(pems)
				case leafWatchID:
					leaf, ok := u.Result.(*structs.IssuedCert)
					if !ok {
						err := fmt.Errorf("invalid type for leaf response: %T", u.Result)
						a.logger.Error("watch error for correlation id",
							"correlation_id", u.CorrelationID,
							"error", err,
						)
						continue
					}
					a.tlsConfigurator.UpdateAutoEncryptCert(leaf.CertPEM, leaf.PrivateKeyPEM)
				}
			}
		}
	}()

	// Setup safety net in case the auto_encrypt cert doesn't get renewed
	// in time. The agent would be stuck in that case because the watches
	// never use the AutoEncrypt.Sign endpoint.
	go func() {
		for {

			// Check 10sec after cert expires. The agent cache
			// should be handling the expiration and renew before
			// it.
			// If there is no cert, AutoEncryptCertNotAfter returns
			// a value in the past which immediately triggers the
			// renew, but this case shouldn't happen because at
			// this point, auto_encrypt was just being setup
			// successfully.
			autoLogger := a.logger.Named(logging.AutoEncrypt)
			interval := a.tlsConfigurator.AutoEncryptCertNotAfter().Sub(time.Now().Add(10 * time.Second))
			a.logger.Debug("setting up client certificate expiration check on interval", "interval", interval)
			select {
			case <-a.shutdownCh:
				return
			case <-time.After(interval):
				// check auto encrypt client cert expiration
				if a.tlsConfigurator.AutoEncryptCertExpired() {
					autoLogger.Debug("client certificate expired.")
					reply, err := a.setupClientAutoEncrypt()
					if err != nil {
						autoLogger.Error("client certificate expired, failed to renew", "error", err)
						// in case of an error, try again in one minute
						interval = time.Minute
						continue
					}
					_, _, err = a.setupClientAutoEncryptCache(reply)
					if err != nil {
						autoLogger.Error("client certificate expired, failed to populate cache", "error", err)
						// in case of an error, try again in one minute
						interval = time.Minute
						continue
					}
				}
			}
		}
	}()

	return nil
}

func (a *Agent) listenAndServeGRPC() error {
	if len(a.config.GRPCAddrs) < 1 {
		return nil
	}

	xdsServer := &xds.Server{
		Logger:       a.logger,
		CfgMgr:       a.proxyConfig,
		Authz:        a,
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
func (a *Agent) listenHTTP() ([]*HTTPServer, error) {
	var ln []net.Listener
	var servers []*HTTPServer
	start := func(proto string, addrs []net.Addr) error {
		listeners, err := a.startListeners(addrs)
		if err != nil {
			return err
		}

		for _, l := range listeners {
			var tlscfg *tls.Config
			_, isTCP := l.(*tcpKeepAliveListener)
			if isTCP && proto == "https" {
				tlscfg = a.tlsConfigurator.IncomingHTTPSConfig()
				l = tls.NewListener(l, tlscfg)
			}

			srv := &HTTPServer{
				Server: &http.Server{
					Addr:      l.Addr().String(),
					TLSConfig: tlscfg,
				},
				ln:       l,
				agent:    a,
				denylist: NewDenylist(a.config.HTTPBlockEndpoints),
				proto:    proto,
			}
			srv.Server.Handler = srv.handler(a.config.EnableDebug)

			// Load the connlimit helper into the server
			connLimitFn := a.httpConnLimiter.HTTPConnStateFunc()

			if proto == "https" {
				// Enforce TLS handshake timeout
				srv.Server.ConnState = func(conn net.Conn, state http.ConnState) {
					switch state {
					case http.StateNew:
						// Set deadline to prevent slow send before TLS handshake or first
						// byte of request.
						conn.SetReadDeadline(time.Now().Add(a.config.HTTPSHandshakeTimeout))
					case http.StateActive:
						// Clear read deadline. We should maybe set read timeouts more
						// generally but that's a bigger task as some HTTP endpoints may
						// stream large requests and responses (e.g. snapshot) so we can't
						// set sensible blanket timeouts here.
						conn.SetReadDeadline(time.Time{})
					}
					// Pass through to conn limit. This is OK because we didn't change
					// state (i.e. Close conn).
					connLimitFn(conn, state)
				}

				// This will enable upgrading connections to HTTP/2 as
				// part of TLS negotiation.
				err = http2.ConfigureServer(srv.Server, nil)
				if err != nil {
					return err
				}
			} else {
				srv.Server.ConnState = connLimitFn
			}

			ln = append(ln, l)
			servers = append(servers, srv)
		}
		return nil
	}

	if err := start("http", a.config.HTTPAddrs); err != nil {
		for _, l := range ln {
			l.Close()
		}
		return nil, err
	}
	if err := start("https", a.config.HTTPSAddrs); err != nil {
		for _, l := range ln {
			l.Close()
		}
		return nil, err
	}
	return servers, nil
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

func (a *Agent) serveHTTP(srv *HTTPServer) error {
	// https://github.com/golang/go/issues/20239
	//
	// In go.8.1 there is a race between Serve and Shutdown. If
	// Shutdown is called before the Serve go routine was scheduled then
	// the Serve go routine never returns. This deadlocks the agent
	// shutdown for some tests since it will wait forever.
	notif := make(chan net.Addr)
	a.wgServers.Add(1)
	go func() {
		defer a.wgServers.Done()
		notif <- srv.ln.Addr()
		err := srv.Serve(srv.ln)
		if err != nil && err != http.ErrServerClosed {
			a.logger.Error("error closing server", "error", err)
		}
	}()

	select {
	case addr := <-notif:
		if srv.proto == "https" {
			a.logger.Info("Started HTTPS server",
				"address", addr.String(),
				"network", addr.Network(),
			)
		} else {
			a.logger.Info("Started HTTP server",
				"address", addr.String(),
				"network", addr.Network(),
			)
		}
		return nil
	case <-time.After(time.Second):
		return fmt.Errorf("agent: timeout starting HTTP servers")
	}
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

		// Parse the watches, excluding 'handler' and 'args'
		wp, err := watch.ParseExempt(params, []string{"handler", "args"})
		if err != nil {
			return fmt.Errorf("Failed to parse watch (%#v): %v", params, err)
		}

		// Get the handler and subprocess arguments
		handler, hasHandler := wp.Exempt["handler"]
		args, hasArgs := wp.Exempt["args"]
		if hasHandler {
			a.logger.Warn("The 'handler' field in watches has been deprecated " +
				"and replaced with the 'args' field. See https://www.consul.io/docs/agent/watches.html")
		}
		if _, ok := handler.(string); hasHandler && !ok {
			return fmt.Errorf("Watch handler must be a string")
		}
		if raw, ok := args.([]interface{}); hasArgs && ok {
			var parsed []string
			for _, arg := range raw {
				v, ok := arg.(string)
				if !ok {
					return fmt.Errorf("Watch args must be a list of strings")
				}

				parsed = append(parsed, v)
			}
			wp.Exempt["args"] = parsed
		} else if hasArgs && !ok {
			return fmt.Errorf("Watch args must be a list of strings")
		}
		if hasHandler && hasArgs || hasHandler && wp.HandlerType == "http" || hasArgs && wp.HandlerType == "http" {
			return fmt.Errorf("Only one watch handler allowed")
		}
		if !hasHandler && !hasArgs && wp.HandlerType != "http" {
			return fmt.Errorf("Must define a watch handler")
		}

		// Store the watch plan
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
			wp.LogOutput = a.LogOutput

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

// consulConfig is used to return a consul configuration
func (a *Agent) consulConfig() (*consul.Config, error) {
	// Start with the provided config or default config
	base := consul.DefaultConfig()

	// This is set when the agent starts up
	base.NodeID = a.config.NodeID

	// Apply dev mode
	base.DevMode = a.config.DevMode

	// Override with our config
	// todo(fs): these are now always set in the runtime config so we can simplify this
	// todo(fs): or is there a reason to keep it like that?
	base.Datacenter = a.config.Datacenter
	base.PrimaryDatacenter = a.config.PrimaryDatacenter
	base.DataDir = a.config.DataDir
	base.NodeName = a.config.NodeName

	base.CoordinateUpdateBatchSize = a.config.ConsulCoordinateUpdateBatchSize
	base.CoordinateUpdateMaxBatches = a.config.ConsulCoordinateUpdateMaxBatches
	base.CoordinateUpdatePeriod = a.config.ConsulCoordinateUpdatePeriod
	base.CheckOutputMaxSize = a.config.CheckOutputMaxSize

	base.RaftConfig.HeartbeatTimeout = a.config.ConsulRaftHeartbeatTimeout
	base.RaftConfig.LeaderLeaseTimeout = a.config.ConsulRaftLeaderLeaseTimeout
	base.RaftConfig.ElectionTimeout = a.config.ConsulRaftElectionTimeout

	base.SerfLANConfig.MemberlistConfig.BindAddr = a.config.SerfBindAddrLAN.IP.String()
	base.SerfLANConfig.MemberlistConfig.BindPort = a.config.SerfBindAddrLAN.Port
	base.SerfLANConfig.MemberlistConfig.CIDRsAllowed = a.config.SerfAllowedCIDRsLAN
	base.SerfWANConfig.MemberlistConfig.CIDRsAllowed = a.config.SerfAllowedCIDRsWAN
	base.SerfLANConfig.MemberlistConfig.AdvertiseAddr = a.config.SerfAdvertiseAddrLAN.IP.String()
	base.SerfLANConfig.MemberlistConfig.AdvertisePort = a.config.SerfAdvertiseAddrLAN.Port
	base.SerfLANConfig.MemberlistConfig.GossipVerifyIncoming = a.config.EncryptVerifyIncoming
	base.SerfLANConfig.MemberlistConfig.GossipVerifyOutgoing = a.config.EncryptVerifyOutgoing
	base.SerfLANConfig.MemberlistConfig.GossipInterval = a.config.GossipLANGossipInterval
	base.SerfLANConfig.MemberlistConfig.GossipNodes = a.config.GossipLANGossipNodes
	base.SerfLANConfig.MemberlistConfig.ProbeInterval = a.config.GossipLANProbeInterval
	base.SerfLANConfig.MemberlistConfig.ProbeTimeout = a.config.GossipLANProbeTimeout
	base.SerfLANConfig.MemberlistConfig.SuspicionMult = a.config.GossipLANSuspicionMult
	base.SerfLANConfig.MemberlistConfig.RetransmitMult = a.config.GossipLANRetransmitMult
	if a.config.ReconnectTimeoutLAN != 0 {
		base.SerfLANConfig.ReconnectTimeout = a.config.ReconnectTimeoutLAN
	}

	if a.config.SerfBindAddrWAN != nil {
		base.SerfWANConfig.MemberlistConfig.BindAddr = a.config.SerfBindAddrWAN.IP.String()
		base.SerfWANConfig.MemberlistConfig.BindPort = a.config.SerfBindAddrWAN.Port
		base.SerfWANConfig.MemberlistConfig.AdvertiseAddr = a.config.SerfAdvertiseAddrWAN.IP.String()
		base.SerfWANConfig.MemberlistConfig.AdvertisePort = a.config.SerfAdvertiseAddrWAN.Port
		base.SerfWANConfig.MemberlistConfig.GossipVerifyIncoming = a.config.EncryptVerifyIncoming
		base.SerfWANConfig.MemberlistConfig.GossipVerifyOutgoing = a.config.EncryptVerifyOutgoing
		base.SerfWANConfig.MemberlistConfig.GossipInterval = a.config.GossipWANGossipInterval
		base.SerfWANConfig.MemberlistConfig.GossipNodes = a.config.GossipWANGossipNodes
		base.SerfWANConfig.MemberlistConfig.ProbeInterval = a.config.GossipWANProbeInterval
		base.SerfWANConfig.MemberlistConfig.ProbeTimeout = a.config.GossipWANProbeTimeout
		base.SerfWANConfig.MemberlistConfig.SuspicionMult = a.config.GossipWANSuspicionMult
		base.SerfWANConfig.MemberlistConfig.RetransmitMult = a.config.GossipWANRetransmitMult
		if a.config.ReconnectTimeoutWAN != 0 {
			base.SerfWANConfig.ReconnectTimeout = a.config.ReconnectTimeoutWAN
		}
	} else {
		// Disable serf WAN federation
		base.SerfWANConfig = nil
	}

	base.RPCAddr = a.config.RPCBindAddr
	base.RPCAdvertise = a.config.RPCAdvertiseAddr

	base.Segment = a.config.SegmentName
	if len(a.config.Segments) > 0 {
		segments, err := a.segmentConfig()
		if err != nil {
			return nil, err
		}
		base.Segments = segments
	}
	if a.config.Bootstrap {
		base.Bootstrap = true
	}
	if a.config.CheckOutputMaxSize > 0 {
		base.CheckOutputMaxSize = a.config.CheckOutputMaxSize
	}
	if a.config.RejoinAfterLeave {
		base.RejoinAfterLeave = true
	}
	if a.config.BootstrapExpect != 0 {
		base.BootstrapExpect = a.config.BootstrapExpect
	}
	if a.config.RPCProtocol > 0 {
		base.ProtocolVersion = uint8(a.config.RPCProtocol)
	}
	if a.config.RaftProtocol != 0 {
		base.RaftConfig.ProtocolVersion = raft.ProtocolVersion(a.config.RaftProtocol)
	}
	if a.config.RaftSnapshotThreshold != 0 {
		base.RaftConfig.SnapshotThreshold = uint64(a.config.RaftSnapshotThreshold)
	}
	if a.config.RaftSnapshotInterval != 0 {
		base.RaftConfig.SnapshotInterval = a.config.RaftSnapshotInterval
	}
	if a.config.RaftTrailingLogs != 0 {
		base.RaftConfig.TrailingLogs = uint64(a.config.RaftTrailingLogs)
	}
	if a.config.ACLMasterToken != "" {
		base.ACLMasterToken = a.config.ACLMasterToken
	}
	if a.config.ACLDatacenter != "" {
		base.ACLDatacenter = a.config.ACLDatacenter
	}
	if a.config.ACLTokenTTL != 0 {
		base.ACLTokenTTL = a.config.ACLTokenTTL
	}
	if a.config.ACLPolicyTTL != 0 {
		base.ACLPolicyTTL = a.config.ACLPolicyTTL
	}
	if a.config.ACLRoleTTL != 0 {
		base.ACLRoleTTL = a.config.ACLRoleTTL
	}
	if a.config.ACLDefaultPolicy != "" {
		base.ACLDefaultPolicy = a.config.ACLDefaultPolicy
	}
	if a.config.ACLDownPolicy != "" {
		base.ACLDownPolicy = a.config.ACLDownPolicy
	}
	base.ACLTokenReplication = a.config.ACLTokenReplication
	base.ACLsEnabled = a.config.ACLsEnabled
	if a.config.ACLEnableKeyListPolicy {
		base.ACLEnableKeyListPolicy = a.config.ACLEnableKeyListPolicy
	}
	if a.config.SessionTTLMin != 0 {
		base.SessionTTLMin = a.config.SessionTTLMin
	}
	if a.config.NonVotingServer {
		base.NonVoter = a.config.NonVotingServer
	}

	// These are fully specified in the agent defaults, so we can simply
	// copy them over.
	base.AutopilotConfig.CleanupDeadServers = a.config.AutopilotCleanupDeadServers
	base.AutopilotConfig.LastContactThreshold = a.config.AutopilotLastContactThreshold
	base.AutopilotConfig.MaxTrailingLogs = uint64(a.config.AutopilotMaxTrailingLogs)
	base.AutopilotConfig.MinQuorum = a.config.AutopilotMinQuorum
	base.AutopilotConfig.ServerStabilizationTime = a.config.AutopilotServerStabilizationTime
	base.AutopilotConfig.RedundancyZoneTag = a.config.AutopilotRedundancyZoneTag
	base.AutopilotConfig.DisableUpgradeMigration = a.config.AutopilotDisableUpgradeMigration
	base.AutopilotConfig.UpgradeVersionTag = a.config.AutopilotUpgradeVersionTag

	// make sure the advertise address is always set
	if base.RPCAdvertise == nil {
		base.RPCAdvertise = base.RPCAddr
	}

	// Rate limiting for RPC calls.
	if a.config.RPCRateLimit > 0 {
		base.RPCRate = a.config.RPCRateLimit
	}
	if a.config.RPCMaxBurst > 0 {
		base.RPCMaxBurst = a.config.RPCMaxBurst
	}

	// RPC timeouts/limits.
	if a.config.RPCHandshakeTimeout > 0 {
		base.RPCHandshakeTimeout = a.config.RPCHandshakeTimeout
	}
	if a.config.RPCMaxConnsPerClient > 0 {
		base.RPCMaxConnsPerClient = a.config.RPCMaxConnsPerClient
	}

	// RPC-related performance configs. We allow explicit zero value to disable so
	// copy it whatever the value.
	base.RPCHoldTimeout = a.config.RPCHoldTimeout

	if a.config.LeaveDrainTime > 0 {
		base.LeaveDrainTime = a.config.LeaveDrainTime
	}

	// set the src address for outgoing rpc connections
	// Use port 0 so that outgoing connections use a random port.
	if !ipaddr.IsAny(base.RPCAddr.IP) {
		base.RPCSrcAddr = &net.TCPAddr{IP: base.RPCAddr.IP}
	}

	// Format the build string
	revision := a.config.Revision
	if len(revision) > 8 {
		revision = revision[:8]
	}
	base.Build = fmt.Sprintf("%s%s:%s", a.config.Version, a.config.VersionPrerelease, revision)

	// Copy the TLS configuration
	base.VerifyIncoming = a.config.VerifyIncoming || a.config.VerifyIncomingRPC
	if a.config.CAPath != "" || a.config.CAFile != "" {
		base.UseTLS = true
	}
	base.VerifyOutgoing = a.config.VerifyOutgoing
	base.VerifyServerHostname = a.config.VerifyServerHostname
	base.CAFile = a.config.CAFile
	base.CAPath = a.config.CAPath
	base.CertFile = a.config.CertFile
	base.KeyFile = a.config.KeyFile
	base.ServerName = a.config.ServerName
	base.Domain = a.config.DNSDomain
	base.TLSMinVersion = a.config.TLSMinVersion
	base.TLSCipherSuites = a.config.TLSCipherSuites
	base.TLSPreferServerCipherSuites = a.config.TLSPreferServerCipherSuites
	base.DefaultQueryTime = a.config.DefaultQueryTime
	base.MaxQueryTime = a.config.MaxQueryTime

	base.AutoEncryptAllowTLS = a.config.AutoEncryptAllowTLS

	// Copy the Connect CA bootstrap config
	if a.config.ConnectEnabled {
		base.ConnectEnabled = true
		base.ConnectMeshGatewayWANFederationEnabled = a.config.ConnectMeshGatewayWANFederationEnabled

		// Allow config to specify cluster_id provided it's a valid UUID. This is
		// meant only for tests where a deterministic ID makes fixtures much simpler
		// to work with but since it's only read on initial cluster bootstrap it's not
		// that much of a liability in production. The worst a user could do is
		// configure logically separate clusters with same ID by mistake but we can
		// avoid documenting this is even an option.
		if clusterID, ok := a.config.ConnectCAConfig["cluster_id"]; ok {
			if cIDStr, ok := clusterID.(string); ok {
				if _, err := uuid.ParseUUID(cIDStr); err == nil {
					// Valid UUID configured, use that
					base.CAConfig.ClusterID = cIDStr
				}
			}
			if base.CAConfig.ClusterID == "" {
				// If the tried to specify an ID but typoed it don't ignore as they will
				// then bootstrap with a new ID and have to throw away the whole cluster
				// and start again.
				a.logger.Error("connect CA config cluster_id specified but " +
					"is not a valid UUID, aborting startup")
				return nil, fmt.Errorf("cluster_id was supplied but was not a valid UUID")
			}
		}

		if a.config.ConnectCAProvider != "" {
			base.CAConfig.Provider = a.config.ConnectCAProvider
		}

		// Merge connect CA Config regardless of provider (since there are some
		// common config options valid to all like leaf TTL).
		for k, v := range a.config.ConnectCAConfig {
			base.CAConfig.Config[k] = v
		}
	}

	// copy over auto config settings
	base.AutoConfigEnabled = a.config.AutoConfig.Enabled
	base.AutoConfigIntroToken = a.config.AutoConfig.IntroToken
	base.AutoConfigIntroTokenFile = a.config.AutoConfig.IntroTokenFile
	base.AutoConfigServerAddresses = a.config.AutoConfig.ServerAddresses
	base.AutoConfigDNSSANs = a.config.AutoConfig.DNSSANs
	base.AutoConfigIPSANs = a.config.AutoConfig.IPSANs
	base.AutoConfigAuthzEnabled = a.config.AutoConfig.Authorizer.Enabled
	base.AutoConfigAuthzAuthMethod = a.config.AutoConfig.Authorizer.AuthMethod
	base.AutoConfigAuthzClaimAssertions = a.config.AutoConfig.Authorizer.ClaimAssertions
	base.AutoConfigAuthzAllowReuse = a.config.AutoConfig.Authorizer.AllowReuse

	// Setup the user event callback
	base.UserEventHandler = func(e serf.UserEvent) {
		select {
		case a.eventCh <- e:
		case <-a.shutdownCh:
		}
	}

	// Setup the loggers
	base.LogLevel = a.config.LogLevel
	base.LogOutput = a.LogOutput

	// This will set up the LAN keyring, as well as the WAN and any segments
	// for servers.
	if err := a.setupKeyrings(base); err != nil {
		return nil, fmt.Errorf("Failed to configure keyring: %v", err)
	}

	base.ConfigEntryBootstrap = a.config.ConfigEntryBootstrap

	return a.enterpriseConsulConfig(base)
}

// Setup the serf and memberlist config for any defined network segments.
func (a *Agent) segmentConfig() ([]consul.NetworkSegment, error) {
	var segments []consul.NetworkSegment
	config := a.config

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
				Port: a.config.ServerPort,
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

// makeRandomID will generate a random UUID for a node.
func (a *Agent) makeRandomID() (string, error) {
	id, err := uuid.GenerateUUID()
	if err != nil {
		return "", err
	}

	a.logger.Debug("Using random ID as node ID", "id", id)
	return id, nil
}

// makeNodeID will try to find a host-specific ID, or else will generate a
// random ID. The returned ID will always be formatted as a GUID. We don't tell
// the caller whether this ID is random or stable since the consequences are
// high for us if this changes, so we will persist it either way. This will let
// gopsutil change implementations without affecting in-place upgrades of nodes.
func (a *Agent) makeNodeID() (string, error) {
	// If they've disabled host-based IDs then just make a random one.
	if a.config.DisableHostNodeID {
		return a.makeRandomID()
	}

	// Try to get a stable ID associated with the host itself.
	info, err := host.Info()
	if err != nil {
		a.logger.Debug("Couldn't get a unique ID from the host", "error", err)
		return a.makeRandomID()
	}

	// Make sure the host ID parses as a UUID, since we don't have complete
	// control over this process.
	id := strings.ToLower(info.HostID)
	if _, err := uuid.ParseUUID(id); err != nil {
		a.logger.Debug("Unique ID from host isn't formatted as a UUID",
			"id", id,
			"error", err,
		)
		return a.makeRandomID()
	}

	// Hash the input to make it well distributed. The reported Host UUID may be
	// similar across nodes if they are on a cloud provider or on motherboards
	// created from the same batch.
	buf := sha512.Sum512([]byte(id))
	id = fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])

	a.logger.Debug("Using unique ID from host as node ID", "id", id)
	return id, nil
}

// setupNodeID will pull the persisted node ID, if any, or create a random one
// and persist it.
func (a *Agent) setupNodeID(config *config.RuntimeConfig) error {
	// If they've configured a node ID manually then just use that, as
	// long as it's valid.
	if config.NodeID != "" {
		config.NodeID = types.NodeID(strings.ToLower(string(config.NodeID)))
		if _, err := uuid.ParseUUID(string(config.NodeID)); err != nil {
			return err
		}

		return nil
	}

	// For dev mode we have no filesystem access so just make one.
	if a.config.DataDir == "" {
		id, err := a.makeNodeID()
		if err != nil {
			return err
		}

		config.NodeID = types.NodeID(id)
		return nil
	}

	// Load saved state, if any. Since a user could edit this, we also
	// validate it.
	fileID := filepath.Join(config.DataDir, "node-id")
	if _, err := os.Stat(fileID); err == nil {
		rawID, err := ioutil.ReadFile(fileID)
		if err != nil {
			return err
		}

		nodeID := strings.TrimSpace(string(rawID))
		nodeID = strings.ToLower(nodeID)
		if _, err := uuid.ParseUUID(nodeID); err != nil {
			return err
		}

		config.NodeID = types.NodeID(nodeID)
	}

	// If we still don't have a valid node ID, make one.
	if config.NodeID == "" {
		id, err := a.makeNodeID()
		if err != nil {
			return err
		}
		if err := lib.EnsurePath(fileID, false); err != nil {
			return err
		}
		if err := ioutil.WriteFile(fileID, []byte(id), 0600); err != nil {
			return err
		}

		config.NodeID = types.NodeID(id)
	}
	return nil
}

// setupBaseKeyrings configures the LAN and WAN keyrings.
func (a *Agent) setupBaseKeyrings(config *consul.Config) error {
	// If the keyring file is disabled then just poke the provided key
	// into the in-memory keyring.
	federationEnabled := config.SerfWANConfig != nil
	if a.config.DisableKeyringFile {
		if a.config.EncryptKey == "" {
			return nil
		}

		keys := []string{a.config.EncryptKey}
		if err := loadKeyring(config.SerfLANConfig, keys); err != nil {
			return err
		}
		if a.config.ServerMode && federationEnabled {
			if err := loadKeyring(config.SerfWANConfig, keys); err != nil {
				return err
			}
		}
		return nil
	}

	// Otherwise, we need to deal with the keyring files.
	fileLAN := filepath.Join(a.config.DataDir, SerfLANKeyring)
	fileWAN := filepath.Join(a.config.DataDir, SerfWANKeyring)

	var existingLANKeyring, existingWANKeyring bool
	if a.config.EncryptKey == "" {
		goto LOAD
	}
	if _, err := os.Stat(fileLAN); err != nil {
		if err := initKeyring(fileLAN, a.config.EncryptKey); err != nil {
			return err
		}
	} else {
		existingLANKeyring = true
	}
	if a.config.ServerMode && federationEnabled {
		if _, err := os.Stat(fileWAN); err != nil {
			if err := initKeyring(fileWAN, a.config.EncryptKey); err != nil {
				return err
			}
		} else {
			existingWANKeyring = true
		}
	}

LOAD:
	if _, err := os.Stat(fileLAN); err == nil {
		config.SerfLANConfig.KeyringFile = fileLAN
	}
	if err := loadKeyringFile(config.SerfLANConfig); err != nil {
		return err
	}
	if a.config.ServerMode && federationEnabled {
		if _, err := os.Stat(fileWAN); err == nil {
			config.SerfWANConfig.KeyringFile = fileWAN
		}
		if err := loadKeyringFile(config.SerfWANConfig); err != nil {
			return err
		}
	}

	// Only perform the following checks if there was an encrypt_key
	// provided in the configuration.
	if a.config.EncryptKey != "" {
		msg := " keyring doesn't include key provided with -encrypt, using keyring"
		if existingLANKeyring &&
			keyringIsMissingKey(
				config.SerfLANConfig.MemberlistConfig.Keyring,
				a.config.EncryptKey,
			) {
			a.logger.Warn(msg, "keyring", "LAN")
		}
		if existingWANKeyring &&
			keyringIsMissingKey(
				config.SerfWANConfig.MemberlistConfig.Keyring,
				a.config.EncryptKey,
			) {
			a.logger.Warn(msg, "keyring", "WAN")
		}
	}

	return nil
}

// setupKeyrings is used to initialize and load keyrings during agent startup.
func (a *Agent) setupKeyrings(config *consul.Config) error {
	// First set up the LAN and WAN keyrings.
	if err := a.setupBaseKeyrings(config); err != nil {
		return err
	}

	// If there's no LAN keyring then there's nothing else to set up for
	// any segments.
	lanKeyring := config.SerfLANConfig.MemberlistConfig.Keyring
	if lanKeyring == nil {
		return nil
	}

	// Copy the initial state of the LAN keyring into each segment config.
	// Segments don't have their own keyring file, they rely on the LAN
	// holding the state so things can't get out of sync.
	k, pk := lanKeyring.GetKeys(), lanKeyring.GetPrimaryKey()
	for _, segment := range config.Segments {
		keyring, err := memberlist.NewKeyring(k, pk)
		if err != nil {
			return err
		}
		segment.SerfConfig.MemberlistConfig.Keyring = keyring
	}
	return nil
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
func (a *Agent) ShutdownEndpoints() {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if len(a.dnsServers) == 0 && len(a.httpServers) == 0 {
		return
	}

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

	for _, srv := range a.httpServers {
		a.logger.Info("Stopping server",
			"protocol", strings.ToUpper(srv.proto),
			"address", srv.ln.Addr().String(),
			"network", srv.ln.Addr().Network(),
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		if ctx.Err() == context.DeadlineExceeded {
			a.logger.Warn("Timeout stopping server",
				"protocol", strings.ToUpper(srv.proto),
				"address", srv.ln.Addr().String(),
				"network", srv.ln.Addr().Network(),
			)
		}
	}
	a.httpServers = nil

	a.logger.Info("Waiting for endpoints to shut down")
	a.wgServers.Wait()
	a.logger.Info("Endpoints down")
}

// ReloadCh is used to return a channel that can be
// used for triggering reloads and returning a response.
func (a *Agent) ReloadCh() chan chan error {
	return a.reloadCh
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
}

// persistService saves a service definition to a JSON file in the data dir
func (a *Agent) persistService(service *structs.NodeService, source configSource) error {
	svcID := service.CompoundServiceID()
	svcPath := filepath.Join(a.config.DataDir, servicesDir, svcID.StringHash())

	wrapped := persistedService{
		Token:   a.State.ServiceToken(service.CompoundServiceID()),
		Service: service,
		Source:  source.String(),
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
	}, a.snapshotCheckState())
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
	}, a.snapshotCheckState())
}

// addServiceLocked adds a service entry to the service manager if enabled, or directly
// to the local state if it is not. This function assumes the state lock is already held.
func (a *Agent) addServiceLocked(req *addServiceRequest, snap map[structs.CheckID]*structs.HealthCheck) error {
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

	return a.addServiceInternal(req, snap)
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
func (a *Agent) addServiceInternal(req *addServiceRequest, snap map[structs.CheckID]*structs.HealthCheck) error {
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
		if err := a.addCheck(checks[i], chkTypes[i], service, persist, token, source); err != nil {
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
	if InvalidDnsRe.MatchString(service.Service) {
		a.logger.Warn("Service name will not be discoverable "+
			"via DNS due to invalid characters. Valid characters include "+
			"all alpha-numerics and dashes.",
			"service", service.Service,
		)
	} else if len(service.Service) > MaxDNSLabelLength {
		a.logger.Warn("Service name will not be discoverable "+
			"via DNS due to it being too long. Valid lengths are between "+
			"1 and 63 bytes.",
			"service", service.Service,
		)
	}

	// Warn if any tags are incompatible with DNS
	for _, tag := range service.Tags {
		if InvalidDnsRe.MatchString(tag) {
			a.logger.Debug("Service tag will not be discoverable "+
				"via DNS due to invalid characters. Valid characters include "+
				"all alpha-numerics and dashes.",
				"tag", tag,
			)
		} else if len(tag) > MaxDNSLabelLength {
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

	err := a.addCheck(check, chkType, service, persist, token, source)
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

func (a *Agent) addCheck(check *structs.HealthCheck, chkType *structs.CheckType, service *structs.NodeService, persist bool, token string, source configSource) error {
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
		}, snap)
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
			}, snap)
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
			}, snap)
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

type persistedTokens struct {
	Replication string `json:"replication,omitempty"`
	AgentMaster string `json:"agent_master,omitempty"`
	Default     string `json:"default,omitempty"`
	Agent       string `json:"agent,omitempty"`
}

func (a *Agent) getPersistedTokens() (*persistedTokens, error) {
	persistedTokens := &persistedTokens{}
	if !a.config.ACLEnableTokenPersistence {
		return persistedTokens, nil
	}

	a.persistedTokensLock.RLock()
	defer a.persistedTokensLock.RUnlock()

	tokensFullPath := filepath.Join(a.config.DataDir, tokensPath)

	buf, err := ioutil.ReadFile(tokensFullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// non-existence is not an error we care about
			return persistedTokens, nil
		}
		return persistedTokens, fmt.Errorf("failed reading tokens file %q: %s", tokensFullPath, err)
	}

	if err := json.Unmarshal(buf, persistedTokens); err != nil {
		return persistedTokens, fmt.Errorf("failed to decode tokens file %q: %s", tokensFullPath, err)
	}

	return persistedTokens, nil
}

// CheckSecurity Performs security checks in Consul Configuration
// It might return an error if configuration is considered too dangerous
func (a *Agent) CheckSecurity(conf *config.RuntimeConfig) error {
	if conf.EnableRemoteScriptChecks {
		if !conf.ACLsEnabled {
			if len(conf.AllowWriteHTTPFrom) == 0 {
				err := fmt.Errorf("using enable-script-checks without ACLs and without allow_write_http_from is DANGEROUS, use enable-local-script-checks instead, see https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations/")
				a.logger.Error("[SECURITY] issue", "error", err)
				// TODO: return the error in future Consul versions
			}
		}
	}
	return nil
}

func (a *Agent) loadTokens(conf *config.RuntimeConfig) error {
	persistedTokens, persistenceErr := a.getPersistedTokens()

	if persistenceErr != nil {
		a.logger.Warn("unable to load persisted tokens", "error", persistenceErr)
	}

	if persistedTokens.Default != "" {
		a.tokens.UpdateUserToken(persistedTokens.Default, token.TokenSourceAPI)

		if conf.ACLToken != "" {
			a.logger.Warn("\"default\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateUserToken(conf.ACLToken, token.TokenSourceConfig)
	}

	if persistedTokens.Agent != "" {
		a.tokens.UpdateAgentToken(persistedTokens.Agent, token.TokenSourceAPI)

		if conf.ACLAgentToken != "" {
			a.logger.Warn("\"agent\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateAgentToken(conf.ACLAgentToken, token.TokenSourceConfig)
	}

	if persistedTokens.AgentMaster != "" {
		a.tokens.UpdateAgentMasterToken(persistedTokens.AgentMaster, token.TokenSourceAPI)

		if conf.ACLAgentMasterToken != "" {
			a.logger.Warn("\"agent_master\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateAgentMasterToken(conf.ACLAgentMasterToken, token.TokenSourceConfig)
	}

	if persistedTokens.Replication != "" {
		a.tokens.UpdateReplicationToken(persistedTokens.Replication, token.TokenSourceAPI)

		if conf.ACLReplicationToken != "" {
			a.logger.Warn("\"replication\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateReplicationToken(conf.ACLReplicationToken, token.TokenSourceConfig)
	}

	return persistenceErr
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

// ReloadConfig will atomically reload all configs from the given newCfg,
// including all services, checks, tokens, metadata, dnsServer configs, etc.
// It will also reload all ongoing watches.
func (a *Agent) ReloadConfig(newCfg *config.RuntimeConfig) error {
	if err := a.CheckSecurity(newCfg); err != nil {
		a.logger.Error("Security error while reloading configuration: %#v", err)
		return err
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
	a.loadTokens(newCfg)
	a.loadEnterpriseTokens(newCfg)

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
	consulCfg, err := a.consulConfig()
	if err != nil {
		return err
	}

	if err := a.delegate.ReloadConfig(consulCfg); err != nil {
		return err
	}

	// Update filtered metrics
	metrics.UpdateFilter(newCfg.Telemetry.AllowedPrefixes,
		newCfg.Telemetry.BlockedPrefixes)

	a.State.SetDiscardCheckOutput(newCfg.DiscardCheckOutput)

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
	var timeout *time.Timer

	if alwaysBlock || hash != "" {
		if wait == 0 {
			wait = defaultQueryTime
		}
		if wait > 10*time.Minute {
			wait = maxQueryTime
		}
		// Apply a small amount of jitter to the request.
		wait += lib.RandomStagger(wait / 16)
		timeout = time.NewTimer(wait)
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
		if timeout == nil || hash != curHash || ws.Watch(timeout.C) {
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
			case <-timeout.C:
			}
		}
	}
}

// registerCache configures the cache and registers all the supported
// types onto the cache. This is NOT safe to call multiple times so
// care should be taken to call this exactly once after the cache
// field has been initialized.
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

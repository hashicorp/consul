package agent

import (
	"context"
	"crypto/sha512"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/ae"
	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/checks"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/local"
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
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"
	multierror "github.com/hashicorp/go-multierror"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"github.com/shirou/gopsutil/host"
	"golang.org/x/net/http2"
)

const (
	// Path to save agent service definitions
	servicesDir = "services"

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
)

type configSource int

const (
	ConfigSourceLocal configSource = iota
	ConfigSourceRemote
)

// delegate defines the interface shared by both
// consul.Client and consul.Server.
type delegate interface {
	Encrypted() bool
	GetLANCoordinate() (lib.CoordinateSet, error)
	Leave() error
	LANMembers() []serf.Member
	LANMembersAllSegments() ([]serf.Member, error)
	LANSegmentMembers(segment string) ([]serf.Member, error)
	LocalMember() serf.Member
	JoinLAN(addrs []string) (n int, err error)
	RemoveFailedNode(node string) error
	ResolveToken(secretID string) (acl.Authorizer, error)
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

// The agent is the long running process that is run on every machine.
// It exposes an RPC interface that is used by the CLI to control the
// agent. The agent runs the query interfaces like HTTP, DNS, and RPC.
// However, it can run in either a client, or server mode. In server
// mode, it runs a full Consul server. In client-only mode, it only forwards
// requests to other Consul servers.
type Agent struct {
	// config is the agent configuration.
	config *config.RuntimeConfig

	// Used for writing our logs
	logger *log.Logger

	// Output sink for logs
	LogOutput io.Writer

	// Used for streaming logs to
	LogWriter *logger.LogWriter

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
	checkReapAfter map[types.CheckID]time.Duration

	// checkMonitors maps the check ID to an associated monitor
	checkMonitors map[types.CheckID]*checks.CheckMonitor

	// checkHTTPs maps the check ID to an associated HTTP check
	checkHTTPs map[types.CheckID]*checks.CheckHTTP

	// checkTCPs maps the check ID to an associated TCP check
	checkTCPs map[types.CheckID]*checks.CheckTCP

	// checkGRPCs maps the check ID to an associated GRPC check
	checkGRPCs map[types.CheckID]*checks.CheckGRPC

	// checkTTLs maps the check ID to an associated check TTL
	checkTTLs map[types.CheckID]*checks.CheckTTL

	// checkDockers maps the check ID to an associated Docker Exec based check
	checkDockers map[types.CheckID]*checks.CheckDocker

	// checkAliases maps the check ID to an associated Alias checks
	checkAliases map[types.CheckID]*checks.CheckAlias

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

	// xdsServer is the Server instance that serves xDS gRPC API.
	xdsServer *xds.Server

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
}

func New(c *config.RuntimeConfig, logger *log.Logger) (*Agent, error) {
	if c.Datacenter == "" {
		return nil, fmt.Errorf("Must configure a Datacenter")
	}
	if c.DataDir == "" && !c.DevMode {
		return nil, fmt.Errorf("Must configure a DataDir")
	}

	a := Agent{
		config:           c,
		checkReapAfter:   make(map[types.CheckID]time.Duration),
		checkMonitors:    make(map[types.CheckID]*checks.CheckMonitor),
		checkTTLs:        make(map[types.CheckID]*checks.CheckTTL),
		checkHTTPs:       make(map[types.CheckID]*checks.CheckHTTP),
		checkTCPs:        make(map[types.CheckID]*checks.CheckTCP),
		checkGRPCs:       make(map[types.CheckID]*checks.CheckGRPC),
		checkDockers:     make(map[types.CheckID]*checks.CheckDocker),
		checkAliases:     make(map[types.CheckID]*checks.CheckAlias),
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

func (a *Agent) Start() error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	c := a.config

	// Warn if the node name is incompatible with DNS
	if InvalidDnsRe.MatchString(a.config.NodeName) {
		a.logger.Printf("[WARN] agent: Node name %q will not be discoverable "+
			"via DNS due to invalid characters. Valid characters include "+
			"all alpha-numerics and dashes.", a.config.NodeName)
	} else if len(a.config.NodeName) > MaxDNSLabelLength {
		a.logger.Printf("[WARN] agent: Node name %q will not be discoverable "+
			"via DNS due to it being too long. Valid lengths are between "+
			"1 and 63 bytes.", a.config.NodeName)
	}

	// load the tokens - this requires the logger to be setup
	// which is why we can't do this in New
	a.loadTokens(a.config)

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

	tlsConfigurator, err := tlsutil.NewConfigurator(c.ToTLSUtilConfig(), a.logger)
	if err != nil {
		return err
	}
	a.tlsConfigurator = tlsConfigurator

	// Setup either the client or the server.
	if c.ServerMode {
		server, err := consul.NewServerLogger(consulCfg, a.logger, a.tokens, a.tlsConfigurator)
		if err != nil {
			return fmt.Errorf("Failed to start Consul server: %v", err)
		}
		a.delegate = server
	} else {
		client, err := consul.NewClientLogger(consulCfg, a.logger, a.tlsConfigurator)
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
		a.logger.Printf("[INFO] AutoEncrypt: upgraded to TLS")
	}

	// Load checks/services/metadata.
	if err := a.loadServices(c); err != nil {
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
		Logger: a.logger,
		State:  a.State,
		Source: &structs.QuerySource{
			Node:       a.config.NodeName,
			Datacenter: a.config.Datacenter,
			Segment:    a.config.SegmentName,
		},
	})
	if err != nil {
		return err
	}
	go func() {
		if err := a.proxyConfig.Run(); err != nil {
			a.logger.Printf("[ERR] Proxy Config Manager exited: %s", err)
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
	go a.retryJoinWAN()

	return nil
}

func (a *Agent) setupClientAutoEncrypt() (*structs.SignedResponse, error) {
	client := a.delegate.(*consul.Client)

	addrs := a.config.StartJoinAddrsLAN
	disco, err := newDiscover()
	if err != nil && len(addrs) == 0 {
		return nil, err
	}
	addrs = append(addrs, retryJoinAddrs(disco, "LAN", a.config.RetryJoinLAN, a.logger)...)

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
						a.logger.Printf("[ERR] %s watch error: %s", u.CorrelationID, err)
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
						a.logger.Printf("[ERR] %s watch error: %s", u.CorrelationID, err)
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
			interval := a.tlsConfigurator.AutoEncryptCertNotAfter().Sub(time.Now().Add(10 * time.Second))
			a.logger.Printf("[DEBUG] AutoEncrypt: client certificate expiration check in %s", interval)
			select {
			case <-a.shutdownCh:
				return
			case <-time.After(interval):
				// check auto encrypt client cert expiration
				if a.tlsConfigurator.AutoEncryptCertExpired() {
					a.logger.Printf("[DEBUG] AutoEncrypt: client certificate expired.")
					reply, err := a.setupClientAutoEncrypt()
					if err != nil {
						a.logger.Printf("[ERR] AutoEncrypt: client certificate expired, failed to renew: %s", err)
						// in case of an error, try again in one minute
						interval = time.Minute
						continue
					}
					_, _, err = a.setupClientAutoEncryptCache(reply)
					if err != nil {
						a.logger.Printf("[ERR] AutoEncrypt: client certificate expired, failed to populate cache: %s", err)
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

	a.xdsServer = &xds.Server{
		Logger:       a.logger,
		CfgMgr:       a.proxyConfig,
		Authz:        a,
		ResolveToken: a.resolveToken,
	}
	a.xdsServer.Initialize()

	var err error
	if a.config.HTTPSPort > 0 {
		// gRPC uses the same TLS settings as the HTTPS API. If HTTPS is
		// enabled then gRPC will require HTTPS as well.
		a.grpcServer, err = a.xdsServer.GRPCServer(a.config.CertFile, a.config.KeyFile)
	} else {
		a.grpcServer, err = a.xdsServer.GRPCServer("", "")
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
			a.logger.Printf("[INFO] agent: Started gRPC server on %s (%s)",
				innerL.Addr().String(), innerL.Addr().Network())
			err := a.grpcServer.Serve(innerL)
			if err != nil {
				a.logger.Printf("[ERR] gRPC server failed: %s", err)
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
			a.logger.Printf("[INFO] agent: Started DNS server %s (%s)", addr.String(), addr.Network())

		case err := <-errCh:
			merr = multierror.Append(merr, err)
		case <-timeout:
			merr = multierror.Append(merr, fmt.Errorf("agent: timeout starting DNS servers"))
			break
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
				ln:        l,
				agent:     a,
				blacklist: NewBlacklist(a.config.HTTPBlockEndpoints),
				proto:     proto,
			}
			srv.Server.Handler = srv.handler(a.config.EnableDebug)

			// This will enable upgrading connections to HTTP/2 as
			// part of TLS negotiation.
			if proto == "https" {
				err = http2.ConfigureServer(srv.Server, nil)
				if err != nil {
					return err
				}
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
		a.logger.Printf("[WARN] agent: Replacing socket %q", path)
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
			a.logger.Print(err)
		}
	}()

	select {
	case addr := <-notif:
		if srv.proto == "https" {
			a.logger.Printf("[INFO] agent: Started HTTPS server on %s (%s)", addr.String(), addr.Network())
		} else {
			a.logger.Printf("[INFO] agent: Started HTTP server on %s (%s)", addr.String(), addr.Network())
		}
		return nil
	case <-time.After(time.Second):
		return fmt.Errorf("agent: timeout starting HTTP servers")
	}
}

// reloadWatches stops any existing watch plans and attempts to load the given
// set of watches.
func (a *Agent) reloadWatches(cfg *config.RuntimeConfig) error {
	// Stop the current watches.
	for _, wp := range a.watchPlans {
		wp.Stop()
	}
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
			a.logger.Printf("[WARN] agent: The 'handler' field in watches has been deprecated " +
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
			a.logger.Printf("[ERR] agent: Failed to run watch: %v", err)
			continue
		}

		a.watchPlans = append(a.watchPlans, wp)
		go func(wp *watch.Plan) {
			if h, ok := wp.Exempt["handler"]; ok {
				wp.Handler = makeWatchHandler(a.LogOutput, h)
			} else if h, ok := wp.Exempt["args"]; ok {
				wp.Handler = makeWatchHandler(a.LogOutput, h)
			} else {
				httpConfig := wp.Exempt["http_handler_config"].(*watch.HttpHandlerConfig)
				wp.Handler = makeHTTPWatchHandler(a.LogOutput, httpConfig)
			}
			wp.LogOutput = a.LogOutput

			addr := config.Address
			if config.Scheme == "https" {
				addr = "https://" + addr
			}

			if err := wp.RunWithConfig(addr, config); err != nil {
				a.logger.Printf("[ERR] agent: Failed to run watch: %v", err)
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
	base.ACLEnforceVersion8 = a.config.ACLEnforceVersion8
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

	// RPC-related performance configs.
	if a.config.RPCHoldTimeout > 0 {
		base.RPCHoldTimeout = a.config.RPCHoldTimeout
	}
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

	base.AutoEncryptAllowTLS = a.config.AutoEncryptAllowTLS

	// Copy the Connect CA bootstrap config
	if a.config.ConnectEnabled {
		base.ConnectEnabled = true

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
				a.logger.Println("[ERR] connect CA config cluster_id specified but " +
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

	return base, nil
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

	a.logger.Printf("[DEBUG] agent: Using random ID %q as node ID", id)
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
		a.logger.Printf("[DEBUG] agent: Couldn't get a unique ID from the host: %v", err)
		return a.makeRandomID()
	}

	// Make sure the host ID parses as a UUID, since we don't have complete
	// control over this process.
	id := strings.ToLower(info.HostID)
	if _, err := uuid.ParseUUID(id); err != nil {
		a.logger.Printf("[DEBUG] agent: Unique ID %q from host isn't formatted as a UUID: %v",
			id, err)
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

	a.logger.Printf("[DEBUG] agent: Using unique ID %q from host as node ID", id)
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

	if a.config.EncryptKey == "" {
		goto LOAD
	}
	if _, err := os.Stat(fileLAN); err != nil {
		if err := initKeyring(fileLAN, a.config.EncryptKey); err != nil {
			return err
		}
	}
	if a.config.ServerMode && federationEnabled {
		if _, err := os.Stat(fileWAN); err != nil {
			if err := initKeyring(fileWAN, a.config.EncryptKey); err != nil {
				return err
			}
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

// SnapshotRPC performs the requested snapshot RPC against the Consul server in
// a streaming manner. The contents of in will be read and passed along as the
// payload, and the response message will determine the error status, and any
// return payload will be written to out.
func (a *Agent) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer,
	replyFn structs.SnapshotReplyFn) error {
	return a.delegate.SnapshotRPC(args, in, out, replyFn)
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
	a.logger.Println("[INFO] agent: Requesting shutdown")

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
			a.logger.Print("[INFO] agent: consul server down")
		} else {
			a.logger.Print("[INFO] agent: consul client down")
		}
	}

	pidErr := a.deletePid()
	if pidErr != nil {
		a.logger.Println("[WARN] agent: could not delete pid file ", pidErr)
	}

	a.logger.Println("[INFO] agent: shutdown complete")
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
			a.logger.Printf("[INFO] agent: Stopping DNS server %s (%s)", srv.Server.Addr, srv.Server.Net)
			srv.Shutdown()
		}
	}
	a.dnsServers = nil

	for _, srv := range a.httpServers {
		a.logger.Printf("[INFO] agent: Stopping %s server %s (%s)", strings.ToUpper(srv.proto), srv.ln.Addr().String(), srv.ln.Addr().Network())
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		if ctx.Err() == context.DeadlineExceeded {
			a.logger.Printf("[WARN] agent: Timeout stopping %s server %s (%s)", strings.ToUpper(srv.proto), srv.ln.Addr().String(), srv.ln.Addr().Network())
		}
	}
	a.httpServers = nil

	a.logger.Println("[INFO] agent: Waiting for endpoints to shut down")
	a.wgServers.Wait()
	a.logger.Print("[INFO] agent: Endpoints down")
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
	a.logger.Printf("[INFO] agent: (LAN) joining: %v", addrs)
	n, err = a.delegate.JoinLAN(addrs)
	if err == nil {
		a.logger.Printf("[INFO] agent: (LAN) joined: %d", n)
		if a.joinLANNotifier != nil {
			if notifErr := a.joinLANNotifier.Notify(systemd.Ready); notifErr != nil {
				a.logger.Printf("[DEBUG] agent: systemd notify failed: %v", notifErr)
			}
		}
	} else {
		a.logger.Printf("[WARN] agent: (LAN) couldn't join: %d Err: %v", n, err)
	}
	return
}

// JoinWAN is used to have the agent join a WAN cluster
func (a *Agent) JoinWAN(addrs []string) (n int, err error) {
	a.logger.Printf("[INFO] agent: (WAN) joining: %v", addrs)
	if srv, ok := a.delegate.(*consul.Server); ok {
		n, err = srv.JoinWAN(addrs)
	} else {
		err = fmt.Errorf("Must be a server to join WAN cluster")
	}
	if err == nil {
		a.logger.Printf("[INFO] agent: (WAN) joined: %d", n)
	} else {
		a.logger.Printf("[WARN] agent: (WAN) couldn't join: %d Err: %v", n, err)
	}
	return
}

// ForceLeave is used to remove a failed node from the cluster
func (a *Agent) ForceLeave(node string) (err error) {
	a.logger.Printf("[INFO] agent: Force leaving node: %v", node)
	err = a.delegate.RemoveFailedNode(node)
	if err != nil {
		a.logger.Printf("[WARN] agent: Failed to remove node: %v", err)
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

// StartSync is called once Services and Checks are registered.
// This is called to prevent a race between clients and the anti-entropy routines
func (a *Agent) StartSync() {
	go a.sync.Run()
	a.logger.Printf("[INFO] agent: started state syncer")
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

// syncPausedCh returns either a channel or nil. If nil sync is not paused. If
// non-nil, the channel will be closed when sync resumes.
func (a *Agent) syncPausedCh() <-chan struct{} {
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
				a.logger.Printf("[ERR] agent: Failed to check servers: %s", err)
				continue
			}
			if !grok {
				a.logger.Printf("[DEBUG] agent: Skipping coordinate updates until servers are upgraded")
				continue
			}

			cs, err := a.GetLANCoordinate()
			if err != nil {
				a.logger.Printf("[ERR] agent: Failed to get coordinate: %s", err)
				continue
			}

			for segment, coord := range cs {
				req := structs.CoordinateUpdateRequest{
					Datacenter:   a.config.Datacenter,
					Node:         a.config.NodeName,
					Segment:      segment,
					Coord:        coord,
					WriteRequest: structs.WriteRequest{Token: a.tokens.AgentToken()},
				}
				var reply struct{}
				if err := a.RPC("Coordinate.Update", &req, &reply); err != nil {
					if acl.IsErrPermissionDenied(err) {
						a.logger.Printf("[WARN] agent: Coordinate update blocked by ACLs")
					} else {
						a.logger.Printf("[ERR] agent: Coordinate update error: %v", err)
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
	reaped := make(map[string]bool)
	for checkID, cs := range a.State.CriticalCheckStates() {
		serviceID := cs.Check.ServiceID

		// There's nothing to do if there's no service.
		if serviceID == "" {
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
			if err := a.RemoveService(serviceID, true); err != nil {
				a.logger.Printf("[ERR] agent: unable to deregister service %q after check %q has been critical for too long: %s",
					serviceID, checkID, err)
			} else {
				a.logger.Printf("[INFO] agent: Check %q for service %q has been critical for too long; deregistered service",
					checkID, serviceID)
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
}

// persistService saves a service definition to a JSON file in the data dir
func (a *Agent) persistService(service *structs.NodeService) error {
	svcPath := filepath.Join(a.config.DataDir, servicesDir, stringHash(service.ID))

	wrapped := persistedService{
		Token:   a.State.ServiceToken(service.ID),
		Service: service,
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	return file.WriteAtomic(svcPath, encoded)
}

// purgeService removes a persisted service definition file from the data dir
func (a *Agent) purgeService(serviceID string) error {
	svcPath := filepath.Join(a.config.DataDir, servicesDir, stringHash(serviceID))
	if _, err := os.Stat(svcPath); err == nil {
		return os.Remove(svcPath)
	}
	return nil
}

// persistCheck saves a check definition to the local agent's state directory
func (a *Agent) persistCheck(check *structs.HealthCheck, chkType *structs.CheckType) error {
	checkPath := filepath.Join(a.config.DataDir, checksDir, checkIDHash(check.CheckID))

	// Create the persisted check
	wrapped := persistedCheck{
		Check:   check,
		ChkType: chkType,
		Token:   a.State.CheckToken(check.CheckID),
	}

	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return err
	}

	return file.WriteAtomic(checkPath, encoded)
}

// purgeCheck removes a persisted check definition file from the data dir
func (a *Agent) purgeCheck(checkID types.CheckID) error {
	checkPath := filepath.Join(a.config.DataDir, checksDir, checkIDHash(checkID))
	if _, err := os.Stat(checkPath); err == nil {
		return os.Remove(checkPath)
	}
	return nil
}

// AddServiceAndReplaceChecks is used to add a service entry and its check. Any check for this service missing from chkTypes will be deleted.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddServiceAndReplaceChecks(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.addServiceLocked(service, chkTypes, persist, token, true, source)
}

// AddService is used to add a service entry.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddService(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, source configSource) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.addServiceLocked(service, chkTypes, persist, token, false, source)
}

// addServiceLocked adds a service entry to the service manager if enabled, or directly
// to the local state if it is not. This function assumes the state lock is already held.
func (a *Agent) addServiceLocked(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, replaceExistingChecks bool, source configSource) error {
	if err := a.validateService(service, chkTypes); err != nil {
		return err
	}

	if a.config.EnableCentralServiceConfig {
		return a.serviceManager.AddService(service, chkTypes, persist, token, source)
	}

	return a.addServiceInternal(service, chkTypes, persist, token, replaceExistingChecks, source)
}

// addServiceInternal adds the given service and checks to the local state.
func (a *Agent) addServiceInternal(service *structs.NodeService, chkTypes []*structs.CheckType, persist bool, token string, replaceExistingChecks bool, source configSource) error {
	// Pause the service syncs during modification
	a.PauseSync()
	defer a.ResumeSync()

	// Take a snapshot of the current state of checks (if any), and when adding
	// a check that already existed carry over the state before resuming
	// anti-entropy.
	snap := a.snapshotCheckState()

	var checks []*structs.HealthCheck

	existingChecks := map[types.CheckID]bool{}
	for _, check := range a.State.Checks() {
		if check.ServiceID == service.ID {
			existingChecks[check.CheckID] = false
		}
	}

	// Create an associated health check
	for i, chkType := range chkTypes {
		existingChecks[chkType.CheckID] = true

		checkID := string(chkType.CheckID)
		if checkID == "" {
			checkID = fmt.Sprintf("service:%s", service.ID)
			if len(chkTypes) > 1 {
				checkID += fmt.Sprintf(":%d", i+1)
			}
		}
		name := chkType.Name
		if name == "" {
			name = fmt.Sprintf("Service '%s' check", service.Service)
		}
		check := &structs.HealthCheck{
			Node:        a.config.NodeName,
			CheckID:     types.CheckID(checkID),
			Name:        name,
			Status:      api.HealthCritical,
			Notes:       chkType.Notes,
			ServiceID:   service.ID,
			ServiceName: service.Service,
			ServiceTags: service.Tags,
		}
		if chkType.Status != "" {
			check.Status = chkType.Status
		}

		// Restore the fields from the snapshot.
		prev, ok := snap[check.CheckID]
		if ok {
			check.Output = prev.Output
			check.Status = prev.Status
		}

		checks = append(checks, check)
	}

	// cleanup, store the ids of services and checks that weren't previously
	// registered so we clean them up if somthing fails halfway through the
	// process.
	var cleanupServices []string
	var cleanupChecks []types.CheckID

	if s := a.State.Service(service.ID); s == nil {
		cleanupServices = append(cleanupServices, service.ID)
	}

	for _, check := range checks {
		if c := a.State.Check(check.CheckID); c == nil {
			cleanupChecks = append(cleanupChecks, check.CheckID)
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
			if err := a.persistCheck(checks[i], chkTypes[i]); err != nil {
				a.cleanupRegistration(cleanupServices, cleanupChecks)
				return err

			}
		}
	}

	// Persist the service to a file
	if persist && a.config.DataDir != "" {
		if err := a.persistService(service); err != nil {
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
		a.logger.Printf("[WARN] agent: Service name %q will not be discoverable "+
			"via DNS due to invalid characters. Valid characters include "+
			"all alpha-numerics and dashes.", service.Service)
	} else if len(service.Service) > MaxDNSLabelLength {
		a.logger.Printf("[WARN] agent: Service name %q will not be discoverable "+
			"via DNS due to it being too long. Valid lengths are between "+
			"1 and 63 bytes.", service.Service)
	}

	// Warn if any tags are incompatible with DNS
	for _, tag := range service.Tags {
		if InvalidDnsRe.MatchString(tag) {
			a.logger.Printf("[DEBUG] agent: Service tag %q will not be discoverable "+
				"via DNS due to invalid characters. Valid characters include "+
				"all alpha-numerics and dashes.", tag)
		} else if len(tag) > MaxDNSLabelLength {
			a.logger.Printf("[DEBUG] agent: Service tag %q will not be discoverable "+
				"via DNS due to it being too long. Valid lengths are between "+
				"1 and 63 bytes.", tag)
		}
	}

	return nil
}

// cleanupRegistration is called on  registration error to ensure no there are no
// leftovers after a partial failure
func (a *Agent) cleanupRegistration(serviceIDs []string, checksIDs []types.CheckID) {
	for _, s := range serviceIDs {
		if err := a.State.RemoveService(s); err != nil {
			a.logger.Printf("[ERR] consul: service registration: cleanup: failed to remove service %s: %s", s, err)
		}
		if err := a.purgeService(s); err != nil {
			a.logger.Printf("[ERR] consul: service registration: cleanup: failed to purge service %s file: %s", s, err)
		}
	}

	for _, c := range checksIDs {
		a.cancelCheckMonitors(c)
		if err := a.State.RemoveCheck(c); err != nil {
			a.logger.Printf("[ERR] consul: service registration: cleanup: failed to remove check %s: %s", c, err)
		}
		if err := a.purgeCheck(c); err != nil {
			a.logger.Printf("[ERR] consul: service registration: cleanup: failed to purge check %s file: %s", c, err)
		}
	}
}

// RemoveService is used to remove a service entry.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveService(serviceID string, persist bool) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.removeServiceLocked(serviceID, persist)
}

// removeServiceLocked is used to remove a service entry.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) removeServiceLocked(serviceID string, persist bool) error {
	// Validate ServiceID
	if serviceID == "" {
		return fmt.Errorf("ServiceID missing")
	}

	// Shut down the config watch in the service manager if enabled.
	if a.config.EnableCentralServiceConfig {
		a.serviceManager.RemoveService(serviceID)
	}

	checks := a.State.Checks()
	var checkIDs []types.CheckID
	for id, check := range checks {
		if check.ServiceID != serviceID {
			continue
		}
		checkIDs = append(checkIDs, id)
	}

	// Remove service immediately
	if err := a.State.RemoveServiceWithChecks(serviceID, checkIDs); err != nil {
		a.logger.Printf("[WARN] agent: Failed to deregister service %q: %s", serviceID, err)
		return nil
	}

	// Remove the service from the data dir
	if persist {
		if err := a.purgeService(serviceID); err != nil {
			return err
		}
	}

	// Deregister any associated health checks
	for checkID, check := range checks {
		if check.ServiceID != serviceID {
			continue
		}
		if err := a.removeCheckLocked(checkID, persist); err != nil {
			return err
		}
	}

	a.logger.Printf("[DEBUG] agent: removed service %q", serviceID)

	// If any Sidecar services exist for the removed service ID, remove them too.
	if sidecar := a.State.Service(a.sidecarServiceID(serviceID)); sidecar != nil {
		// Double check that it's not just an ID collision and we actually added
		// this from a sidecar.
		if sidecar.LocallyRegisteredAsSidecar {
			// Remove it!
			err := a.removeServiceLocked(a.sidecarServiceID(serviceID), persist)
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

	if check.ServiceID != "" {
		service = a.State.Service(check.ServiceID)
		if service == nil {
			return fmt.Errorf("ServiceID %q does not exist", check.ServiceID)
		}
	}

	// snapshot the current state of the health check to avoid potential flapping
	existing := a.State.Check(check.CheckID)
	defer func() {
		if existing != nil {
			a.State.UpdateCheck(check.CheckID, existing.Status, existing.Output)
		}
	}()

	err := a.addCheck(check, chkType, service, persist, token, source)
	if err != nil {
		a.State.RemoveCheck(check.CheckID)
		return err
	}

	// Add to the local state for anti-entropy
	err = a.State.AddCheck(check, token)
	if err != nil {
		return err
	}

	// Persist the check
	if persist && a.config.DataDir != "" {
		return a.persistCheck(check, chkType)
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
		switch {

		case chkType.IsTTL():
			if existing, ok := a.checkTTLs[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkTTLs, check.CheckID)
			}

			ttl := &checks.CheckTTL{
				Notify:        a.State,
				CheckID:       check.CheckID,
				TTL:           chkType.TTL,
				Logger:        a.logger,
				OutputMaxSize: maxOutputSize,
			}

			// Restore persisted state, if any
			if err := a.loadCheckState(check); err != nil {
				a.logger.Printf("[WARN] agent: failed restoring state for check %q: %s",
					check.CheckID, err)
			}

			ttl.Start()
			a.checkTTLs[check.CheckID] = ttl

		case chkType.IsHTTP():
			if existing, ok := a.checkHTTPs[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkHTTPs, check.CheckID)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Println(fmt.Sprintf("[WARN] agent: check '%s' has interval below minimum of %v",
					check.CheckID, checks.MinInterval))
				chkType.Interval = checks.MinInterval
			}

			tlsClientConfig := a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify)

			http := &checks.CheckHTTP{
				Notify:          a.State,
				CheckID:         check.CheckID,
				HTTP:            chkType.HTTP,
				Header:          chkType.Header,
				Method:          chkType.Method,
				Interval:        chkType.Interval,
				Timeout:         chkType.Timeout,
				Logger:          a.logger,
				OutputMaxSize:   maxOutputSize,
				TLSClientConfig: tlsClientConfig,
			}
			http.Start()
			a.checkHTTPs[check.CheckID] = http

		case chkType.IsTCP():
			if existing, ok := a.checkTCPs[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkTCPs, check.CheckID)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Println(fmt.Sprintf("[WARN] agent: check '%s' has interval below minimum of %v",
					check.CheckID, checks.MinInterval))
				chkType.Interval = checks.MinInterval
			}

			tcp := &checks.CheckTCP{
				Notify:   a.State,
				CheckID:  check.CheckID,
				TCP:      chkType.TCP,
				Interval: chkType.Interval,
				Timeout:  chkType.Timeout,
				Logger:   a.logger,
			}
			tcp.Start()
			a.checkTCPs[check.CheckID] = tcp

		case chkType.IsGRPC():
			if existing, ok := a.checkGRPCs[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkGRPCs, check.CheckID)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Println(fmt.Sprintf("[WARN] agent: check '%s' has interval below minimum of %v",
					check.CheckID, checks.MinInterval))
				chkType.Interval = checks.MinInterval
			}

			var tlsClientConfig *tls.Config
			if chkType.GRPCUseTLS {
				tlsClientConfig = a.tlsConfigurator.OutgoingTLSConfigForCheck(chkType.TLSSkipVerify)
			}

			grpc := &checks.CheckGRPC{
				Notify:          a.State,
				CheckID:         check.CheckID,
				GRPC:            chkType.GRPC,
				Interval:        chkType.Interval,
				Timeout:         chkType.Timeout,
				Logger:          a.logger,
				TLSClientConfig: tlsClientConfig,
			}
			grpc.Start()
			a.checkGRPCs[check.CheckID] = grpc

		case chkType.IsDocker():
			if existing, ok := a.checkDockers[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkDockers, check.CheckID)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Println(fmt.Sprintf("[WARN] agent: check '%s' has interval below minimum of %v",
					check.CheckID, checks.MinInterval))
				chkType.Interval = checks.MinInterval
			}

			if a.dockerClient == nil {
				dc, err := checks.NewDockerClient(os.Getenv("DOCKER_HOST"), int64(maxOutputSize))
				if err != nil {
					a.logger.Printf("[ERR] agent: error creating docker client: %s", err)
					return err
				}
				a.logger.Printf("[DEBUG] agent: created docker client for %s", dc.Host())
				a.dockerClient = dc
			}

			dockerCheck := &checks.CheckDocker{
				Notify:            a.State,
				CheckID:           check.CheckID,
				DockerContainerID: chkType.DockerContainerID,
				Shell:             chkType.Shell,
				ScriptArgs:        chkType.ScriptArgs,
				Interval:          chkType.Interval,
				Logger:            a.logger,
				Client:            a.dockerClient,
			}
			if prev := a.checkDockers[check.CheckID]; prev != nil {
				prev.Stop()
			}
			dockerCheck.Start()
			a.checkDockers[check.CheckID] = dockerCheck

		case chkType.IsMonitor():
			if existing, ok := a.checkMonitors[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkMonitors, check.CheckID)
			}
			if chkType.Interval < checks.MinInterval {
				a.logger.Printf("[WARN] agent: check '%s' has interval below minimum of %v",
					check.CheckID, checks.MinInterval)
				chkType.Interval = checks.MinInterval
			}
			monitor := &checks.CheckMonitor{
				Notify:        a.State,
				CheckID:       check.CheckID,
				ScriptArgs:    chkType.ScriptArgs,
				Interval:      chkType.Interval,
				Timeout:       chkType.Timeout,
				Logger:        a.logger,
				OutputMaxSize: maxOutputSize,
			}
			monitor.Start()
			a.checkMonitors[check.CheckID] = monitor

		case chkType.IsAlias():
			if existing, ok := a.checkAliases[check.CheckID]; ok {
				existing.Stop()
				delete(a.checkAliases, check.CheckID)
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

			chkImpl := &checks.CheckAlias{
				Notify:    a.State,
				RPC:       a.delegate,
				RPCReq:    rpcReq,
				CheckID:   check.CheckID,
				Node:      chkType.AliasNode,
				ServiceID: chkType.AliasService,
			}
			chkImpl.Start()
			a.checkAliases[check.CheckID] = chkImpl

		default:
			return fmt.Errorf("Check type is not valid")
		}

		if chkType.DeregisterCriticalServiceAfter > 0 {
			timeout := chkType.DeregisterCriticalServiceAfter
			if timeout < a.config.CheckDeregisterIntervalMin {
				timeout = a.config.CheckDeregisterIntervalMin
				a.logger.Println(fmt.Sprintf("[WARN] agent: check '%s' has deregister interval below minimum of %v",
					check.CheckID, a.config.CheckDeregisterIntervalMin))
			}
			a.checkReapAfter[check.CheckID] = timeout
		} else {
			delete(a.checkReapAfter, check.CheckID)
		}
	}

	return nil
}

// RemoveCheck is used to remove a health check.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveCheck(checkID types.CheckID, persist bool) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()
	return a.removeCheckLocked(checkID, persist)
}

// removeCheckLocked is used to remove a health check.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) removeCheckLocked(checkID types.CheckID, persist bool) error {
	// Validate CheckID
	if checkID == "" {
		return fmt.Errorf("CheckID missing")
	}

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
	a.logger.Printf("[DEBUG] agent: removed check %q", checkID)
	return nil
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

func (a *Agent) cancelCheckMonitors(checkID types.CheckID) {
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
func (a *Agent) updateTTLCheck(checkID types.CheckID, status, output string) error {
	a.stateLock.Lock()
	defer a.stateLock.Unlock()

	// Grab the TTL check.
	check, ok := a.checkTTLs[checkID]
	if !ok {
		return fmt.Errorf("CheckID %q does not have associated TTL", checkID)
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
		return fmt.Errorf("failed persisting state for check %q: %s", checkID, err)
	}

	return nil
}

// persistCheckState is used to record the check status into the data dir.
// This allows the state to be restored on a later agent start. Currently
// only useful for TTL based checks.
func (a *Agent) persistCheckState(check *checks.CheckTTL, status, output string) error {
	// Create the persisted state
	state := persistedCheckState{
		CheckID: check.CheckID,
		Status:  status,
		Output:  output,
		Expires: time.Now().Add(check.TTL).Unix(),
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
	file := filepath.Join(dir, checkIDHash(check.CheckID))

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
	// Try to read the persisted state for this check
	file := filepath.Join(a.config.DataDir, checkStateDir, checkIDHash(check.CheckID))
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
		a.logger.Printf("[ERR] agent: failed decoding check state: %s", err)
		return a.purgeCheckState(check.CheckID)
	}

	// Check if the state has expired
	if time.Now().Unix() >= p.Expires {
		a.logger.Printf("[DEBUG] agent: check state expired for %q, not restoring", check.CheckID)
		return a.purgeCheckState(check.CheckID)
	}

	// Restore the fields from the state
	check.Output = p.Output
	check.Status = p.Status
	return nil
}

// purgeCheckState is used to purge the state of a check from the data dir
func (a *Agent) purgeCheckState(checkID types.CheckID) error {
	file := filepath.Join(a.config.DataDir, checkStateDir, checkIDHash(checkID))
	err := os.Remove(file)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (a *Agent) GossipEncrypted() bool {
	return a.delegate.Encrypted()
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
func (a *Agent) loadServices(conf *config.RuntimeConfig) error {
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

		if err := a.addServiceLocked(ns, chkTypes, false, service.Token, false, ConfigSourceLocal); err != nil {
			return fmt.Errorf("Failed to register service %q: %v", service.Name, err)
		}

		// If there is a sidecar service, register that too.
		if sidecar != nil {
			if err := a.addServiceLocked(sidecar, sidecarChecks, false, sidecarToken, false, ConfigSourceLocal); err != nil {
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
			a.logger.Printf("[WARN] agent: Ignoring temporary service file %v", fi.Name())
			continue
		}

		// Open the file for reading
		file := filepath.Join(svcDir, fi.Name())
		fh, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed opening service file %q: %s", file, err)
		}

		// Read the contents into a buffer
		buf, err := ioutil.ReadAll(fh)
		fh.Close()
		if err != nil {
			return fmt.Errorf("failed reading service file %q: %s", file, err)
		}

		// Try decoding the service definition
		var p persistedService
		if err := json.Unmarshal(buf, &p); err != nil {
			// Backwards-compatibility for pre-0.5.1 persisted services
			if err := json.Unmarshal(buf, &p.Service); err != nil {
				a.logger.Printf("[ERR] agent: Failed decoding service file %q: %s", file, err)
				continue
			}
		}
		serviceID := p.Service.ID

		if a.State.Service(serviceID) != nil {
			// Purge previously persisted service. This allows config to be
			// preferred over services persisted from the API.
			a.logger.Printf("[DEBUG] agent: service %q exists, not restoring from %q",
				serviceID, file)
			if err := a.purgeService(serviceID); err != nil {
				return fmt.Errorf("failed purging service %q: %s", serviceID, err)
			}
		} else {
			a.logger.Printf("[DEBUG] agent: restored service definition %q from %q",
				serviceID, file)
			if err := a.addServiceLocked(p.Service, nil, false, p.Token, false, ConfigSourceLocal); err != nil {
				return fmt.Errorf("failed adding service %q: %s", serviceID, err)
			}
		}
	}

	return nil
}

// unloadServices will deregister all services.
func (a *Agent) unloadServices() error {
	for id := range a.State.Services() {
		if err := a.removeServiceLocked(id, false); err != nil {
			return fmt.Errorf("Failed deregistering service '%s': %v", id, err)
		}
	}
	return nil
}

// loadChecks loads check definitions and/or persisted check definitions from
// disk and re-registers them with the local agent.
func (a *Agent) loadChecks(conf *config.RuntimeConfig, snap map[types.CheckID]*structs.HealthCheck) error {
	// Register the checks from config
	for _, check := range conf.Checks {
		health := check.HealthCheck(conf.NodeName)

		// Restore the fields from the snapshot.
		if prev, ok := snap[health.CheckID]; ok {
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

		// Open the file for reading
		file := filepath.Join(checkDir, fi.Name())
		fh, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("Failed opening check file %q: %s", file, err)
		}

		// Read the contents into a buffer
		buf, err := ioutil.ReadAll(fh)
		fh.Close()
		if err != nil {
			return fmt.Errorf("failed reading check file %q: %s", file, err)
		}

		// Decode the check
		var p persistedCheck
		if err := json.Unmarshal(buf, &p); err != nil {
			a.logger.Printf("[ERR] agent: Failed decoding check file %q: %s", file, err)
			continue
		}
		checkID := p.Check.CheckID

		if a.State.Check(checkID) != nil {
			// Purge previously persisted check. This allows config to be
			// preferred over persisted checks from the API.
			a.logger.Printf("[DEBUG] agent: check %q exists, not restoring from %q",
				checkID, file)
			if err := a.purgeCheck(checkID); err != nil {
				return fmt.Errorf("Failed purging check %q: %s", checkID, err)
			}
		} else {
			// Default check to critical to avoid placing potentially unhealthy
			// services into the active pool
			p.Check.Status = api.HealthCritical

			// Restore the fields from the snapshot.
			if prev, ok := snap[p.Check.CheckID]; ok {
				p.Check.Output = prev.Output
				p.Check.Status = prev.Status
			}

			if err := a.addCheckLocked(p.Check, p.ChkType, false, p.Token, ConfigSourceLocal); err != nil {
				// Purge the check if it is unable to be restored.
				a.logger.Printf("[WARN] agent: Failed to restore check %q: %s",
					checkID, err)
				if err := a.purgeCheck(checkID); err != nil {
					return fmt.Errorf("Failed purging check %q: %s", checkID, err)
				}
			}
			a.logger.Printf("[DEBUG] agent: restored health check %q from %q",
				p.Check.CheckID, file)
		}
	}

	return nil
}

// unloadChecks will deregister all checks known to the local agent.
func (a *Agent) unloadChecks() error {
	for id := range a.State.Checks() {
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

func (a *Agent) loadTokens(conf *config.RuntimeConfig) error {
	persistedTokens, persistenceErr := a.getPersistedTokens()

	if persistenceErr != nil {
		a.logger.Printf("[WARN] unable to load persisted tokens: %v", persistenceErr)
	}

	if persistedTokens.Default != "" {
		a.tokens.UpdateUserToken(persistedTokens.Default, token.TokenSourceAPI)

		if conf.ACLToken != "" {
			a.logger.Printf("[WARN] \"default\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateUserToken(conf.ACLToken, token.TokenSourceConfig)
	}

	if persistedTokens.Agent != "" {
		a.tokens.UpdateAgentToken(persistedTokens.Agent, token.TokenSourceAPI)

		if conf.ACLAgentToken != "" {
			a.logger.Printf("[WARN] \"agent\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateAgentToken(conf.ACLAgentToken, token.TokenSourceConfig)
	}

	if persistedTokens.AgentMaster != "" {
		a.tokens.UpdateAgentMasterToken(persistedTokens.AgentMaster, token.TokenSourceAPI)

		if conf.ACLAgentMasterToken != "" {
			a.logger.Printf("[WARN] \"agent_master\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateAgentMasterToken(conf.ACLAgentMasterToken, token.TokenSourceConfig)
	}

	if persistedTokens.Replication != "" {
		a.tokens.UpdateReplicationToken(persistedTokens.Replication, token.TokenSourceAPI)

		if conf.ACLReplicationToken != "" {
			a.logger.Printf("[WARN] \"replication\" token present in both the configuration and persisted token store, using the persisted token")
		}
	} else {
		a.tokens.UpdateReplicationToken(conf.ACLReplicationToken, token.TokenSourceConfig)
	}

	return persistenceErr
}

// snapshotCheckState is used to snapshot the current state of the health
// checks. This is done before we reload our checks, so that we can properly
// restore into the same state.
func (a *Agent) snapshotCheckState() map[types.CheckID]*structs.HealthCheck {
	return a.State.Checks()
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
func serviceMaintCheckID(serviceID string) types.CheckID {
	return types.CheckID(structs.ServiceMaintPrefix + serviceID)
}

// EnableServiceMaintenance will register a false health check against the given
// service ID with critical status. This will exclude the service from queries.
func (a *Agent) EnableServiceMaintenance(serviceID, reason, token string) error {
	service, ok := a.State.Services()[serviceID]
	if !ok {
		return fmt.Errorf("No service registered with ID %q", serviceID)
	}

	// Check if maintenance mode is not already enabled
	checkID := serviceMaintCheckID(serviceID)
	if _, ok := a.State.Checks()[checkID]; ok {
		return nil
	}

	// Use default notes if no reason provided
	if reason == "" {
		reason = defaultServiceMaintReason
	}

	// Create and register the critical health check
	check := &structs.HealthCheck{
		Node:        a.config.NodeName,
		CheckID:     checkID,
		Name:        "Service Maintenance Mode",
		Notes:       reason,
		ServiceID:   service.ID,
		ServiceName: service.Service,
		Status:      api.HealthCritical,
	}
	a.AddCheck(check, nil, true, token, ConfigSourceLocal)
	a.logger.Printf("[INFO] agent: Service %q entered maintenance mode", serviceID)

	return nil
}

// DisableServiceMaintenance will deregister the fake maintenance mode check
// if the service has been marked as in maintenance.
func (a *Agent) DisableServiceMaintenance(serviceID string) error {
	if _, ok := a.State.Services()[serviceID]; !ok {
		return fmt.Errorf("No service registered with ID %q", serviceID)
	}

	// Check if maintenance mode is enabled
	checkID := serviceMaintCheckID(serviceID)
	if _, ok := a.State.Checks()[checkID]; !ok {
		return nil
	}

	// Deregister the maintenance check
	a.RemoveCheck(checkID, true)
	a.logger.Printf("[INFO] agent: Service %q left maintenance mode", serviceID)

	return nil
}

// EnableNodeMaintenance places a node into maintenance mode.
func (a *Agent) EnableNodeMaintenance(reason, token string) {
	// Ensure node maintenance is not already enabled
	if _, ok := a.State.Checks()[structs.NodeMaint]; ok {
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
	}
	a.AddCheck(check, nil, true, token, ConfigSourceLocal)
	a.logger.Printf("[INFO] agent: Node entered maintenance mode")
}

// DisableNodeMaintenance removes a node from maintenance mode
func (a *Agent) DisableNodeMaintenance() {
	if _, ok := a.State.Checks()[structs.NodeMaint]; !ok {
		return
	}
	a.RemoveCheck(structs.NodeMaint, true)
	a.logger.Printf("[INFO] agent: Node left maintenance mode")
}

func (a *Agent) loadLimits(conf *config.RuntimeConfig) {
	a.config.RPCRateLimit = conf.RPCRateLimit
	a.config.RPCMaxBurst = conf.RPCMaxBurst
}

func (a *Agent) ReloadConfig(newCfg *config.RuntimeConfig) error {
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

	if err := a.tlsConfigurator.Update(newCfg.ToTLSUtilConfig()); err != nil {
		return fmt.Errorf("Failed reloading tls configuration: %s", err)
	}

	// Reload service/check definitions and metadata.
	if err := a.loadServices(newCfg); err != nil {
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

	for _, s := range a.dnsServers {
		if err := s.ReloadConfig(newCfg); err != nil {
			return fmt.Errorf("Failed reloading dns config : %v", err)
		}
	}

	// this only gets used by the consulConfig function and since
	// that is only ever done during init and reload here then
	// an in place modification is safe as reloads cannot be
	// concurrent due to both gaing a full lock on the stateLock
	a.config.ConfigEntryBootstrap = newCfg.ConfigEntryBootstrap

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

// registerCache configures the cache and registers all the supported
// types onto the cache. This is NOT safe to call multiple times so
// care should be taken to call this exactly once after the cache
// field has been initialized.
func (a *Agent) registerCache() {
	// Note that you should register the _agent_ as the RPC implementation and not
	// the a.delegate directly, otherwise tests that rely on overriding RPC
	// routing via a.registerEndpoint will not work.

	a.cache.RegisterType(cachetype.ConnectCARootName, &cachetype.ConnectCARoot{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.ConnectCALeafName, &cachetype.ConnectCALeaf{
		RPC:                              a,
		Cache:                            a.cache,
		Datacenter:                       a.config.Datacenter,
		TestOverrideCAChangeInitialDelay: a.config.ConnectTestCALeafRootChangeSpread,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.IntentionMatchName, &cachetype.IntentionMatch{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.CatalogServicesName, &cachetype.CatalogServices{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.HealthServicesName, &cachetype.HealthServices{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.PreparedQueryName, &cachetype.PreparedQuery{
		RPC: a,
	}, &cache.RegisterOptions{
		// Prepared queries don't support blocking
		Refresh: false,
	})

	a.cache.RegisterType(cachetype.NodeServicesName, &cachetype.NodeServices{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.ResolvedServiceConfigName, &cachetype.ResolvedServiceConfig{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.CatalogListServicesName, &cachetype.CatalogListServices{
		RPC: a,
	}, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.CatalogDatacentersName, &cachetype.CatalogDatacenters{
		RPC: a,
	}, &cache.RegisterOptions{
		Refresh: false,
	})

	a.cache.RegisterType(cachetype.InternalServiceDumpName, &cachetype.InternalServiceDump{
		RPC: a,
	}, &cache.RegisterOptions{
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.CompiledDiscoveryChainName, &cachetype.CompiledDiscoveryChain{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})

	a.cache.RegisterType(cachetype.ConfigEntriesName, &cachetype.ConfigEntries{
		RPC: a,
	}, &cache.RegisterOptions{
		// Maintain a blocking query, retry dropped connections quickly
		Refresh:        true,
		RefreshTimer:   0 * time.Second,
		RefreshTimeout: 10 * time.Minute,
	})
}

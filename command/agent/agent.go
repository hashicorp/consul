package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
)

/*
 The agent is the long running process that is run on every machine.
 It exposes an RPC interface that is used by the CLI to control the
 agent. The agent runs the query interfaces like HTTP, DNS, and RPC.
 However, it can run in either a client, or server mode. In server
 mode, it runs a full Consul server. In client-only mode, it only forwards
 requests to other Consul servers.
*/
type Agent struct {
	config *Config

	// Used for writing our logs
	logger *log.Logger

	// Output sink for logs
	logOutput io.Writer

	// We have one of a client or a server, depending
	// on our configuration
	server *consul.Server
	client *consul.Client

	// state stores a local representation of the node,
	// services and checks. Used for anti-entropy.
	state localState

	// checkMonitors maps the check ID to an associated monitor
	// checkTTLs maps the check ID to an associated check TTL
	// checkLock protects updates to either
	checkMonitors map[string]*CheckMonitor
	checkTTLs     map[string]*CheckTTL
	checkLock     sync.Mutex

	shutdown     bool
	shutdownCh   chan struct{}
	shutdownLock sync.Mutex
}

// Create is used to create a new Agent. Returns
// the agent or potentially an error.
func Create(config *Config, logOutput io.Writer) (*Agent, error) {
	// Ensure we have a log sink
	if logOutput == nil {
		logOutput = os.Stderr
	}

	// Validate the config
	if config.Datacenter == "" {
		return nil, fmt.Errorf("Must configure a Datacenter")
	}
	if config.DataDir == "" {
		return nil, fmt.Errorf("Must configure a DataDir")
	}

	// Try to get an advertise address
	if config.AdvertiseAddr != "" {
		if ip := net.ParseIP(config.AdvertiseAddr); ip == nil {
			return nil, fmt.Errorf("Failed to parse advertise address: %v", config.AdvertiseAddr)
		}
	} else if config.BindAddr != "0.0.0.0" && config.BindAddr != "" {
		config.AdvertiseAddr = config.BindAddr
	} else {
		ip, err := consul.GetPrivateIP()
		if err != nil {
			return nil, fmt.Errorf("Failed to get advertise address: %v", err)
		}
		config.AdvertiseAddr = ip.IP.String()
	}

	agent := &Agent{
		config:        config,
		logger:        log.New(logOutput, "", log.LstdFlags),
		logOutput:     logOutput,
		checkMonitors: make(map[string]*CheckMonitor),
		checkTTLs:     make(map[string]*CheckTTL),
		shutdownCh:    make(chan struct{}),
	}

	// Initialize the local state
	agent.state.Init(config, agent.logger)

	// Setup either the client or the server
	var err error
	if config.Server {
		err = agent.setupServer()
		agent.state.SetIface(agent.server)
	} else {
		err = agent.setupClient()
		agent.state.SetIface(agent.client)
	}
	if err != nil {
		return nil, err
	}

	return agent, nil
}

// consulConfig is used to return a consul configuration
func (a *Agent) consulConfig() *consul.Config {
	// Start with the provided config or default config
	var base *consul.Config
	if a.config.ConsulConfig != nil {
		base = a.config.ConsulConfig
	} else {
		base = consul.DefaultConfig()
	}

	// Override with our config
	if a.config.Datacenter != "" {
		base.Datacenter = a.config.Datacenter
	}
	if a.config.DataDir != "" {
		base.DataDir = a.config.DataDir
	}
	if a.config.EncryptKey != "" {
		key, _ := a.config.EncryptBytes()
		base.SerfLANConfig.MemberlistConfig.SecretKey = key
		base.SerfWANConfig.MemberlistConfig.SecretKey = key
	}
	if a.config.NodeName != "" {
		base.NodeName = a.config.NodeName
	}
	if a.config.BindAddr != "" {
		base.SerfLANConfig.MemberlistConfig.BindAddr = a.config.BindAddr
		base.SerfWANConfig.MemberlistConfig.BindAddr = a.config.BindAddr
	}
	if a.config.Ports.SerfLan != 0 {
		base.SerfLANConfig.MemberlistConfig.BindPort = a.config.Ports.SerfLan
		base.SerfLANConfig.MemberlistConfig.AdvertisePort = a.config.Ports.SerfLan
	}
	if a.config.Ports.SerfWan != 0 {
		base.SerfWANConfig.MemberlistConfig.BindPort = a.config.Ports.SerfWan
		base.SerfWANConfig.MemberlistConfig.AdvertisePort = a.config.Ports.SerfWan
	}
	if a.config.BindAddr != "" {
		bindAddr := &net.TCPAddr{
			IP:   net.ParseIP(a.config.BindAddr),
			Port: a.config.Ports.Server,
		}
		base.RPCAddr = bindAddr
	}
	if a.config.AdvertiseAddr != "" {
		base.SerfLANConfig.MemberlistConfig.AdvertiseAddr = a.config.AdvertiseAddr
		base.SerfWANConfig.MemberlistConfig.AdvertiseAddr = a.config.AdvertiseAddr
		base.RPCAdvertise = &net.TCPAddr{
			IP:   net.ParseIP(a.config.AdvertiseAddr),
			Port: a.config.Ports.Server,
		}
	}
	if a.config.Bootstrap {
		base.Bootstrap = true
	}
	if a.config.Protocol > 0 {
		base.ProtocolVersion = uint8(a.config.Protocol)
	}

	// Copy the TLS configuration
	base.VerifyIncoming = a.config.VerifyIncoming
	base.VerifyOutgoing = a.config.VerifyOutgoing
	base.CAFile = a.config.CAFile
	base.CertFile = a.config.CertFile
	base.KeyFile = a.config.KeyFile

	// Setup the ServerUp callback
	base.ServerUp = a.state.ConsulServerUp

	// Setup the loggers
	base.LogOutput = a.logOutput
	return base
}

// setupServer is used to initialize the Consul server
func (a *Agent) setupServer() error {
	server, err := consul.NewServer(a.consulConfig())
	if err != nil {
		return fmt.Errorf("Failed to start Consul server: %v", err)
	}
	a.server = server
	return nil
}

// setupClient is used to initialize the Consul client
func (a *Agent) setupClient() error {
	client, err := consul.NewClient(a.consulConfig())
	if err != nil {
		return fmt.Errorf("Failed to start Consul client: %v", err)
	}
	a.client = client
	return nil
}

// RPC is used to make an RPC call to the Consul servers
// This allows the agent to implement the Consul.Interface
func (a *Agent) RPC(method string, args interface{}, reply interface{}) error {
	if a.server != nil {
		return a.server.RPC(method, args, reply)
	}
	return a.client.RPC(method, args, reply)
}

// Leave is used to prepare the agent for a graceful shutdown
func (a *Agent) Leave() error {
	if a.server != nil {
		return a.server.Leave()
	} else {
		return a.client.Leave()
	}
}

// Shutdown is used to hard stop the agent. Should be
// preceeded by a call to Leave to do it gracefully.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}

	// Stop all the checks
	a.checkLock.Lock()
	defer a.checkLock.Unlock()
	for _, chk := range a.checkMonitors {
		chk.Stop()
	}
	for _, chk := range a.checkTTLs {
		chk.Stop()
	}

	a.logger.Println("[INFO] agent: requesting shutdown")
	var err error
	if a.server != nil {
		err = a.server.Shutdown()
	} else {
		err = a.client.Shutdown()
	}

	a.logger.Println("[INFO] agent: shutdown complete")
	a.shutdown = true
	close(a.shutdownCh)
	return err
}

// ShutdownCh is used to return a channel that can be
// selected to wait for the agent to perform a shutdown.
func (a *Agent) ShutdownCh() <-chan struct{} {
	return a.shutdownCh
}

// JoinLAN is used to have the agent join a LAN cluster
func (a *Agent) JoinLAN(addrs []string) (n int, err error) {
	a.logger.Printf("[INFO] agent: (LAN) joining: %v", addrs)
	if a.server != nil {
		n, err = a.server.JoinLAN(addrs)
	} else {
		n, err = a.client.JoinLAN(addrs)
	}
	a.logger.Printf("[INFO] agent: (LAN) joined: %d Err: %v", n, err)
	return
}

// JoinWAN is used to have the agent join a WAN cluster
func (a *Agent) JoinWAN(addrs []string) (n int, err error) {
	a.logger.Printf("[INFO] agent: (WAN) joining: %v", addrs)
	if a.server != nil {
		n, err = a.server.JoinWAN(addrs)
	} else {
		err = fmt.Errorf("Must be a server to join WAN cluster")
	}
	a.logger.Printf("[INFO] agent: (WAN) joined: %d Err: %v", n, err)
	return
}

// ForceLeave is used to remove a failed node from the cluster
func (a *Agent) ForceLeave(node string) (err error) {
	a.logger.Printf("[INFO] Force leaving node: %v", node)
	if a.server != nil {
		err = a.server.RemoveFailedNode(node)
	} else {
		err = a.client.RemoveFailedNode(node)
	}
	if err != nil {
		a.logger.Printf("[WARN] Failed to remove node: %v", err)
	}
	return err
}

// LANMembers is used to retrieve the LAN members
func (a *Agent) LANMembers() []serf.Member {
	if a.server != nil {
		return a.server.LANMembers()
	} else {
		return a.client.LANMembers()
	}
}

// WANMembers is used to retrieve the WAN members
func (a *Agent) WANMembers() []serf.Member {
	if a.server != nil {
		return a.server.WANMembers()
	} else {
		return nil
	}
}

// StartSync is called once Services and Checks are registered.
// This is called to prevent a race between clients and the anti-entropy routines
func (a *Agent) StartSync() {
	// Start the anti entropy routine
	go a.state.antiEntropy(a.shutdownCh)
}

// PauseSync is used to pause anti-entropy while bulk changes are make
func (a *Agent) PauseSync() {
	a.state.Pause()
}

// ResumeSync is used to unpause anti-entropy after bulk changes are make
func (a *Agent) ResumeSync() {
	a.state.Resume()
}

// AddService is used to add a service entry.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered
func (a *Agent) AddService(service *structs.NodeService, chkType *CheckType) error {
	if service.Service == "" {
		return fmt.Errorf("Service name missing")
	}
	if service.ID == "" && service.Service != "" {
		service.ID = service.Service
	}
	if chkType != nil && !chkType.Valid() {
		return fmt.Errorf("Check type is not valid")
	}

	// Add the service
	a.state.AddService(service)

	// Create an associated health check
	if chkType != nil {
		check := &structs.HealthCheck{
			Node:        a.config.NodeName,
			CheckID:     fmt.Sprintf("service:%s", service.ID),
			Name:        fmt.Sprintf("Service '%s' check", service.Service),
			Status:      structs.HealthUnknown,
			Notes:       "",
			ServiceID:   service.ID,
			ServiceName: service.Service,
		}
		if err := a.AddCheck(check, chkType); err != nil {
			a.state.RemoveService(service.ID)
			return err
		}
	}
	return nil
}

// RemoveService is used to remove a service entry.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveService(serviceID string) error {
	// Remove service immeidately
	a.state.RemoveService(serviceID)

	// Deregister any associated health checks
	checkID := fmt.Sprintf("service:%s", serviceID)
	return a.RemoveCheck(checkID)
}

// AddCheck is used to add a health check to the agent.
// This entry is persistent and the agent will make a best effort to
// ensure it is registered. The Check may include a CheckType which
// is used to automatically update the check status
func (a *Agent) AddCheck(check *structs.HealthCheck, chkType *CheckType) error {
	if check.CheckID == "" {
		return fmt.Errorf("CheckID missing")
	}
	if chkType != nil && !chkType.Valid() {
		return fmt.Errorf("Check type is not valid")
	}

	a.checkLock.Lock()
	defer a.checkLock.Unlock()

	// Check if already registered
	if chkType != nil {
		if chkType.IsTTL() {
			if _, ok := a.checkTTLs[check.CheckID]; ok {
				return fmt.Errorf("CheckID is already registered")
			}

			ttl := &CheckTTL{
				Notify:  &a.state,
				CheckID: check.CheckID,
				TTL:     chkType.TTL,
				Logger:  a.logger,
			}
			ttl.Start()
			a.checkTTLs[check.CheckID] = ttl

		} else {
			if _, ok := a.checkMonitors[check.CheckID]; ok {
				return fmt.Errorf("CheckID is already registered")
			}
			if chkType.Interval < MinInterval {
				a.logger.Println(fmt.Sprintf("[WARN] agent: check '%s' has interval below minimum of %v",
					check.CheckID, MinInterval))
				chkType.Interval = MinInterval
			}

			monitor := &CheckMonitor{
				Notify:   &a.state,
				CheckID:  check.CheckID,
				Script:   chkType.Script,
				Interval: chkType.Interval,
				Logger:   a.logger,
			}
			monitor.Start()
			a.checkMonitors[check.CheckID] = monitor
		}
	}

	// Add to the local state for anti-entropy
	a.state.AddCheck(check)
	return nil
}

// RemoveCheck is used to remove a health check.
// The agent will make a best effort to ensure it is deregistered
func (a *Agent) RemoveCheck(checkID string) error {
	// Add to the local state for anti-entropy
	a.state.RemoveCheck(checkID)

	a.checkLock.Lock()
	defer a.checkLock.Unlock()

	// Stop any monitors
	if check, ok := a.checkMonitors[checkID]; ok {
		check.Stop()
		delete(a.checkMonitors, checkID)
	}
	if check, ok := a.checkTTLs[checkID]; ok {
		check.Stop()
		delete(a.checkTTLs, checkID)
	}
	return nil
}

// UpdateCheck is used to update the status of a check.
// This can only be used with checks of the TTL type.
func (a *Agent) UpdateCheck(checkID, status, output string) error {
	a.checkLock.Lock()
	defer a.checkLock.Unlock()

	check, ok := a.checkTTLs[checkID]
	if !ok {
		return fmt.Errorf("CheckID does not have associated TTL")
	}

	// Set the status through CheckTTL to reset the TTL
	check.SetStatus(status, output)
	return nil
}

// Stats is used to get various debugging state from the sub-systems
func (a *Agent) Stats() map[string]map[string]string {
	toString := func(v uint64) string {
		return strconv.FormatUint(v, 10)
	}
	var stats map[string]map[string]string
	if a.server != nil {
		stats = a.server.Stats()
	} else {
		stats = a.client.Stats()
	}
	stats["agent"] = map[string]string{
		"check_monitors": toString(uint64(len(a.checkMonitors))),
		"check_ttls":     toString(uint64(len(a.checkTTLs))),
		"checks":         toString(uint64(len(a.state.checks))),
		"services":       toString(uint64(len(a.state.services))),
	}
	return stats
}

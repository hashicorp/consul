package agent

import (
	"flag"
	"fmt"
	"github.com/hashicorp/logutils"
	"github.com/mitchellh/cli"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// gracefulTimeout controls how long we wait before forcefully terminating
var gracefulTimeout = 5 * time.Second

// Command is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Ui         cli.Ui
	ShutdownCh <-chan struct{}
	args       []string
	logFilter  *logutils.LevelFilter
	agent      *Agent
	rpcServer  *AgentRPC
	httpServer *HTTPServer
	dnsServer  *DNSServer
}

// readConfig is responsible for setup of our configuration using
// the command line and any file configs
func (c *Command) readConfig() *Config {
	var cmdConfig Config
	var configFiles []string
	cmdFlags := flag.NewFlagSet("agent", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.StringVar(&cmdConfig.SerfBindAddr, "serf-bind", "", "address to bind serf listeners to")
	cmdFlags.StringVar(&cmdConfig.ServerAddr, "server-addr", "", "address to bind server listeners to")
	cmdFlags.Var((*AppendSliceValue)(&configFiles), "config-file",
		"json file to read config from")
	cmdFlags.Var((*AppendSliceValue)(&configFiles), "config-dir",
		"directory of json files to read")
	cmdFlags.StringVar(&cmdConfig.EncryptKey, "encrypt", "", "encryption key")
	cmdFlags.StringVar(&cmdConfig.LogLevel, "log-level", "", "log level")
	cmdFlags.StringVar(&cmdConfig.NodeName, "node", "", "node name")
	cmdFlags.StringVar(&cmdConfig.RPCAddr, "rpc-addr", "",
		"address to bind RPC listener to")
	cmdFlags.StringVar(&cmdConfig.DataDir, "data", "", "path to the data directory")
	cmdFlags.StringVar(&cmdConfig.Datacenter, "dc", "", "node datacenter")
	cmdFlags.StringVar(&cmdConfig.DNSRecursor, "recursor", "", "address of dns recursor")
	cmdFlags.StringVar(&cmdConfig.AdvertiseAddr, "advertise", "", "advertise address to use")
	cmdFlags.StringVar(&cmdConfig.HTTPAddr, "http-addr", "", "address to bind http server to")
	cmdFlags.StringVar(&cmdConfig.DNSAddr, "dns-addr", "", "address to bind dns server to")
	cmdFlags.BoolVar(&cmdConfig.Server, "server", false, "run agent as server")
	cmdFlags.BoolVar(&cmdConfig.Bootstrap, "bootstrap", false, "enable server bootstrap mode")
	if err := cmdFlags.Parse(c.args); err != nil {
		return nil
	}

	config := DefaultConfig()
	if len(configFiles) > 0 {
		fileConfig, err := ReadConfigPaths(configFiles)
		if err != nil {
			c.Ui.Error(err.Error())
			return nil
		}

		config = MergeConfig(config, fileConfig)
	}

	config = MergeConfig(config, &cmdConfig)

	if config.NodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error determining hostname: %s", err))
			return nil
		}
		config.NodeName = hostname
	}

	if config.EncryptKey != "" {
		if _, err := config.EncryptBytes(); err != nil {
			c.Ui.Error(fmt.Sprintf("Invalid encryption key: %s", err))
			return nil
		}
	}

	return config
}

// setupLoggers is used to setup the logGate, logWriter, and our logOutput
func (c *Command) setupLoggers(config *Config) (*GatedWriter, *logWriter, io.Writer) {
	// Setup logging. First create the gated log writer, which will
	// store logs until we're ready to show them. Then create the level
	// filter, filtering logs of the specified level.
	logGate := &GatedWriter{
		Writer: &cli.UiWriter{Ui: c.Ui},
	}

	c.logFilter = LevelFilter()
	c.logFilter.MinLevel = logutils.LogLevel(strings.ToUpper(config.LogLevel))
	c.logFilter.Writer = logGate
	if !ValidateLevelFilter(c.logFilter.MinLevel, c.logFilter) {
		c.Ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			c.logFilter.MinLevel, c.logFilter.Levels))
		return nil, nil, nil
	}

	// Create a log writer, and wrap a logOutput around it
	logWriter := NewLogWriter(512)
	logOutput := io.MultiWriter(c.logFilter, logWriter)
	return logGate, logWriter, logOutput
}

// setupAgent is used to start the agent and various interfaces
func (c *Command) setupAgent(config *Config, logOutput io.Writer, logWriter *logWriter) error {
	c.Ui.Output("Starting Consul agent...")
	agent, err := Create(config, logOutput)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting agent: %s", err))
		return err
	}
	c.agent = agent

	// Setup the RPC listener
	rpcListener, err := net.Listen("tcp", config.RPCAddr)
	if err != nil {
		agent.Shutdown()
		c.Ui.Error(fmt.Sprintf("Error starting RPC listener: %s", err))
		return err
	}

	// Start the IPC layer
	c.Ui.Output("Starting Consul agent RPC...")
	c.rpcServer = NewAgentRPC(agent, rpcListener, logOutput, logWriter)

	if config.HTTPAddr != "" {
		server, err := NewHTTPServer(agent, logOutput, config.HTTPAddr)
		if err != nil {
			agent.Shutdown()
			c.Ui.Error(fmt.Sprintf("Error starting http server: %s", err))
			return err
		}
		c.httpServer = server
	}

	if config.DNSAddr != "" {
		server, err := NewDNSServer(agent, logOutput, config.Domain,
			config.DNSAddr, config.DNSRecursor)
		if err != nil {
			agent.Shutdown()
			c.Ui.Error(fmt.Sprintf("Error starting dns server: %s", err))
			return err
		}
		c.dnsServer = server
	}

	return nil
}

func (c *Command) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	// Parse our configs
	c.args = args
	config := c.readConfig()
	if config == nil {
		return 1
	}
	c.args = args

	// Setup the log outputs
	logGate, logWriter, logOutput := c.setupLoggers(config)
	if logWriter == nil {
		return 1
	}

	// Create the agent
	if err := c.setupAgent(config, logOutput, logWriter); err != nil {
		return 1
	}
	defer c.agent.Shutdown()
	if c.rpcServer != nil {
		defer c.rpcServer.Shutdown()
	}
	if c.httpServer != nil {
		defer c.httpServer.Shutdown()
	}

	// Register the services
	for _, service := range config.Services {
		ns := service.NodeService()
		chkType := service.CheckType()
		if err := c.agent.AddService(ns, chkType); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to register service '%s': %v", service.Name, err))
			return 1
		}
	}

	// Register the checks
	for _, check := range config.Checks {
		health := check.HealthCheck(config.NodeName)
		chkType := &check.CheckType
		if err := c.agent.AddCheck(health, chkType); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to register check '%s': %v %v", check.Name, err, check))
			return 1
		}
	}

	// Let the agent know we've finished registration
	c.agent.StartSync()

	c.Ui.Output("Consul agent running!")
	c.Ui.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	c.Ui.Info(fmt.Sprintf("    Datacenter: '%s'", config.Datacenter))
	c.Ui.Info(fmt.Sprintf("Advertise addr: '%s'", config.AdvertiseAddr))
	c.Ui.Info(fmt.Sprintf("      RPC addr: '%s'", config.RPCAddr))
	c.Ui.Info(fmt.Sprintf("     HTTP addr: '%s'", config.HTTPAddr))
	c.Ui.Info(fmt.Sprintf("      DNS addr: '%s'", config.DNSAddr))
	c.Ui.Info(fmt.Sprintf("     Encrypted: %#v", config.EncryptKey != ""))
	c.Ui.Info(fmt.Sprintf("        Server: %v (bootstrap: %v)", config.Server, config.Bootstrap))

	// Enable log streaming
	c.Ui.Info("")
	c.Ui.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Wait for exit
	return c.handleSignals(config)
}

// handleSignals blocks until we get an exit-causing signal
func (c *Command) handleSignals(config *Config) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// Wait for a signal
WAIT:
	var sig os.Signal
	select {
	case s := <-signalCh:
		sig = s
	case <-c.ShutdownCh:
		sig = os.Interrupt
	case <-c.agent.ShutdownCh():
		// Agent is already shutdown!
		return 0
	}
	c.Ui.Output(fmt.Sprintf("Caught signal: %v", sig))

	// Check if this is a SIGHUP
	if sig == syscall.SIGHUP {
		config = c.handleReload(config)
		goto WAIT
	}

	// Check if we should do a graceful leave
	graceful := false
	if sig == os.Interrupt && !config.SkipLeaveOnInt {
		graceful = true
	} else if sig == syscall.SIGTERM && config.LeaveOnTerm {
		graceful = true
	}

	// Bail fast if not doing a graceful leave
	if !graceful {
		return 1
	}

	// Attempt a graceful leave
	gracefulCh := make(chan struct{})
	c.Ui.Output("Gracefully shutting down agent...")
	go func() {
		if err := c.agent.Leave(); err != nil {
			c.Ui.Error(fmt.Sprintf("Error: %s", err))
			return
		}
		close(gracefulCh)
	}()

	// Wait for leave or another signal
	select {
	case <-signalCh:
		return 1
	case <-time.After(gracefulTimeout):
		return 1
	case <-gracefulCh:
		return 0
	}
}

// handleReload is invoked when we should reload our configs, e.g. SIGHUP
func (c *Command) handleReload(config *Config) *Config {
	c.Ui.Output("Reloading configuration...")
	newConf := c.readConfig()
	if newConf == nil {
		c.Ui.Error(fmt.Sprintf("Failed to reload configs"))
		return config
	}

	// Change the log level
	minLevel := logutils.LogLevel(strings.ToUpper(newConf.LogLevel))
	if ValidateLevelFilter(minLevel, c.logFilter) {
		c.logFilter.SetMinLevel(minLevel)
	} else {
		c.Ui.Error(fmt.Sprintf(
			"Invalid log level: %s. Valid log levels are: %v",
			minLevel, c.logFilter.Levels))

		// Keep the current log level
		newConf.LogLevel = config.LogLevel
	}

	// Bulk update the services and checks
	c.agent.PauseSync()
	defer c.agent.ResumeSync()

	// Deregister the old services
	for _, service := range config.Services {
		ns := service.NodeService()
		c.agent.RemoveService(ns.ID)
	}

	// Deregister the old checks
	for _, check := range config.Checks {
		health := check.HealthCheck(config.NodeName)
		c.agent.RemoveCheck(health.CheckID)
	}

	// Register the services
	for _, service := range newConf.Services {
		ns := service.NodeService()
		chkType := service.CheckType()
		if err := c.agent.AddService(ns, chkType); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to register service '%s': %v", service.Name, err))
		}
	}

	// Register the checks
	for _, check := range newConf.Checks {
		health := check.HealthCheck(config.NodeName)
		chkType := &check.CheckType
		if err := c.agent.AddCheck(health, chkType); err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to register check '%s': %v %v", check.Name, err, check))
		}
	}

	return newConf
}

func (c *Command) Synopsis() string {
	return "Runs a Consul agent"
}

func (c *Command) Help() string {
	helpText := `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster. An agent can also serve
  as a server by configuraiton.

Options:

  -rpc-addr=127.0.0.1:7373 Address to bind the RPC listener.
`
	return strings.TrimSpace(helpText)
}

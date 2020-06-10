package agent

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/service_os"
	"github.com/hashicorp/go-checkpoint"
	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/mitchellh/cli"
	"google.golang.org/grpc/grpclog"
)

func New(ui cli.Ui, revision, version, versionPre, versionHuman string, shutdownCh <-chan struct{}) *cmd {
	ui = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           ui,
	}

	c := &cmd{
		UI:                ui,
		revision:          revision,
		version:           version,
		versionPrerelease: versionPre,
		versionHuman:      versionHuman,
		shutdownCh:        shutdownCh,
	}
	c.init()
	return c
}

// AgentCommand is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type cmd struct {
	UI                cli.Ui
	flags             *flag.FlagSet
	http              *flags.HTTPFlags
	help              string
	revision          string
	version           string
	versionPrerelease string
	versionHuman      string
	shutdownCh        <-chan struct{}
	flagArgs          config.Flags
	logger            hclog.InterceptLogger
}

type GatedUi struct {
	JSONoutput bool
	ui         cli.Ui
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	config.AddFlags(c.flags, &c.flagArgs)
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	code := c.run(args)
	if c.logger != nil {
		c.logger.Info("Exit code", "code", code)
	}
	return code
}

// readConfig is responsible for setup of our configuration using
// the command line and any file configs
func (c *cmd) readConfig() *config.RuntimeConfig {
	b, err := config.NewBuilder(c.flagArgs)
	if err != nil {
		c.UI.Error(err.Error())
		return nil
	}
	cfg, err := b.BuildAndValidate()
	if err != nil {
		c.UI.Error(err.Error())
		return nil
	}
	for _, w := range b.Warnings {
		c.UI.Warn(w)
	}
	return &cfg
}

// checkpointResults is used to handler periodic results from our update checker
func (c *cmd) checkpointResults(results *checkpoint.CheckResponse, err error) {
	if err != nil {
		c.logger.Error("Failed to check for updates", "error", err)
		return
	}
	if results.Outdated {
		c.logger.Info("Newer Consul version available", "new_version", results.CurrentVersion, "current_version", c.version)
	}
	for _, alert := range results.Alerts {
		switch alert.Level {
		case "info":
			c.logger.Info("Bulletin", "alert_level", alert.Level, "alert_message", alert.Message, "alert_URL", alert.URL)
		default:
			c.logger.Error("Bulletin", "alert_level", alert.Level, "alert_message", alert.Message, "alert_URL", alert.URL)
		}
	}
}

func (c *cmd) startupUpdateCheck(config *config.RuntimeConfig) {
	version := config.Version
	if config.VersionPrerelease != "" {
		version += fmt.Sprintf("-%s", config.VersionPrerelease)
	}
	updateParams := &checkpoint.CheckParams{
		Product: "consul",
		Version: version,
	}
	if !config.DisableAnonymousSignature {
		updateParams.SignatureFile = filepath.Join(config.DataDir, "checkpoint-signature")
	}

	// Schedule a periodic check with expected interval of 24 hours
	checkpoint.CheckInterval(updateParams, 24*time.Hour, c.checkpointResults)

	// Do an immediate check within the next 30 seconds
	go func() {
		time.Sleep(lib.RandomStagger(30 * time.Second))
		c.checkpointResults(checkpoint.Check(updateParams))
	}()
}

// startupJoin is invoked to handle any joins specified to take place at start time
func (c *cmd) startupJoin(agent *agent.Agent, cfg *config.RuntimeConfig) error {
	if len(cfg.StartJoinAddrsLAN) == 0 {
		return nil
	}

	c.UI.Output("Joining cluster...")
	n, err := agent.JoinLAN(cfg.StartJoinAddrsLAN)
	if err != nil {
		return err
	}

	c.logger.Info("Join completed. Initial agents synced with", "agent_count", n)
	return nil
}

// startupJoinWan is invoked to handle any joins -wan specified to take place at start time
func (c *cmd) startupJoinWan(agent *agent.Agent, cfg *config.RuntimeConfig) error {
	if len(cfg.StartJoinAddrsWAN) == 0 {
		return nil
	}

	c.UI.Output("Joining -wan cluster...")
	n, err := agent.JoinWAN(cfg.StartJoinAddrsWAN)
	if err != nil {
		return err
	}

	c.logger.Info("Join -wan completed. Initial agents synced with", "agent_count", n)
	return nil
}

func (c *cmd) run(args []string) int {
	// Parse our configs
	if err := c.flags.Parse(args); err != nil {
		if !strings.Contains(err.Error(), "help requested") {
			c.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		}
		return 1
	}
	c.flagArgs.Args = c.flags.Args()
	config := c.readConfig()
	if config == nil {
		return 1
	}

	// Setup the log outputs
	logConfig := &logging.Config{
		LogLevel:          config.LogLevel,
		LogJSON:           config.LogJSON,
		Name:              logging.Agent,
		EnableSyslog:      config.EnableSyslog,
		SyslogFacility:    config.SyslogFacility,
		LogFilePath:       config.LogFile,
		LogRotateDuration: config.LogRotateDuration,
		LogRotateBytes:    config.LogRotateBytes,
		LogRotateMaxFiles: config.LogRotateMaxFiles,
	}
	logger, logGate, logOutput, ok := logging.Setup(logConfig, c.UI)
	if !ok {
		return 1
	}

	c.logger = logger

	//Setup gate to check if we should output CLI information
	cli := GatedUi{
		JSONoutput: config.LogJSON,
		ui:         c.UI,
	}

	// Setup gRPC logger to use the same output/filtering
	grpclog.SetLoggerV2(logging.NewGRPCLogger(logConfig, c.logger))

	memSink, err := lib.InitTelemetry(config.Telemetry)
	if err != nil {
		c.logger.Error(err.Error())
		logGate.Flush()
		return 1
	}

	// Create the agent
	cli.output("Starting Consul agent...")
	agent, err := agent.NewWithOptions(config, agent.WithLogger(c.logger), agent.WithFlags(&c.flagArgs))
	if err != nil {
		c.logger.Error("Error creating agent", "error", err)
		logGate.Flush()
		return 1
	}
	agent.LogOutput = logOutput
	agent.MemSink = memSink

	segment := config.SegmentName
	if config.ServerMode {
		segment = "<all>"
	}
	cli.info(fmt.Sprintf("       Version: '%s'", c.versionHuman))
	cli.info(fmt.Sprintf("       Node ID: '%s'", config.NodeID))
	cli.info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	cli.info(fmt.Sprintf("    Datacenter: '%s' (Segment: '%s')", config.Datacenter, segment))
	cli.info(fmt.Sprintf("        Server: %v (Bootstrap: %v)", config.ServerMode, config.Bootstrap))
	cli.info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, HTTPS: %d, gRPC: %d, DNS: %d)", config.ClientAddrs,
		config.HTTPPort, config.HTTPSPort, config.GRPCPort, config.DNSPort))
	cli.info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddrLAN,
		config.SerfPortLAN, config.SerfPortWAN))
	cli.info(fmt.Sprintf("       Encrypt: Gossip: %v, TLS-Outgoing: %v, TLS-Incoming: %v, Auto-Encrypt-TLS: %t",
		config.EncryptKey != "", config.VerifyOutgoing, config.VerifyIncoming, config.AutoEncryptTLS || config.AutoEncryptAllowTLS))
	// Enable log streaming
	cli.output("")
	cli.output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// wait for signal
	signalCh := make(chan os.Signal, 10)
	stopCh := make(chan struct{})
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	go func() {
		for {
			var sig os.Signal
			select {
			case s := <-signalCh:
				sig = s
			case <-stopCh:
				return
			}

			switch sig {
			case syscall.SIGPIPE:
				continue

			case syscall.SIGHUP:
				err := fmt.Errorf("cannot reload before agent started")
				c.logger.Error("Caught", "signal", sig, "error", err)

			default:
				c.logger.Info("Caught", "signal", sig)
				agent.InterruptStartCh <- struct{}{}
				return
			}
		}
	}()

	err = agent.Start()
	signal.Stop(signalCh)
	select {
	case stopCh <- struct{}{}:
	default:
	}
	if err != nil {
		c.logger.Error("Error starting agent", "error", err)
		return 1
	}

	// shutdown agent before endpoints
	defer agent.ShutdownEndpoints()
	defer agent.ShutdownAgent()

	if !config.DisableUpdateCheck && !config.DevMode {
		c.startupUpdateCheck(config)
	}

	if err := c.startupJoin(agent, config); err != nil {
		c.logger.Error((err.Error()))
		return 1
	}

	if err := c.startupJoinWan(agent, config); err != nil {
		c.logger.Error((err.Error()))
		return 1
	}

	// Let the agent know we've finished registration
	agent.StartSync()

	cli.output("Consul agent running!")

	// wait for signal
	signalCh = make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	for {
		var sig os.Signal
		var reloadErrCh chan error
		select {
		case s := <-signalCh:
			sig = s
		case ch := <-agent.ReloadCh():
			sig = syscall.SIGHUP
			reloadErrCh = ch
		case <-service_os.Shutdown_Channel():
			sig = os.Interrupt
		case <-c.shutdownCh:
			sig = os.Interrupt
		case err := <-agent.RetryJoinCh():
			c.logger.Error("Retry join failed", "error", err)
			return 1
		case <-agent.ShutdownCh():
			// agent is already down!
			return 0
		}

		switch sig {
		case syscall.SIGPIPE:
			continue

		case syscall.SIGHUP:
			c.logger.Info("Caught", "signal", sig)

			conf, err := c.handleReload(agent, config)
			if conf != nil {
				config = conf
			}
			if err != nil {
				c.logger.Error("Reload config failed", "error", err)
			}
			// Send result back if reload was called via HTTP
			if reloadErrCh != nil {
				reloadErrCh <- err
			}

		default:
			c.logger.Info("Caught", "signal", sig)

			graceful := (sig == os.Interrupt && !(config.SkipLeaveOnInt)) || (sig == syscall.SIGTERM && (config.LeaveOnTerm))
			if !graceful {
				c.logger.Info("Graceful shutdown disabled. Exiting")
				return 1
			}

			c.logger.Info("Gracefully shutting down agent...")
			gracefulCh := make(chan struct{})
			go func() {
				if err := agent.Leave(); err != nil {
					c.logger.Error("Error on leave", "error", err)
					return
				}
				close(gracefulCh)
			}()

			gracefulTimeout := 15 * time.Second
			select {
			case <-signalCh:
				c.logger.Info("Caught second signal, Exiting", "signal", sig)
				return 1
			case <-time.After(gracefulTimeout):
				c.logger.Info("Timeout on graceful leave. Exiting")
				return 1
			case <-gracefulCh:
				c.logger.Info("Graceful exit completed")
				return 0
			}
		}
	}
}

// handleReload is invoked when we should reload our configs, e.g. SIGHUP
func (c *cmd) handleReload(agent *agent.Agent, cfg *config.RuntimeConfig) (*config.RuntimeConfig, error) {
	c.logger.Info("Reloading configuration...")
	var errs error
	// dont use the version of readConfig in this file as it doesn't take into account the
	// extra auto-config.json source
	newCfg, warnings, err := agent.ReadConfig()
	if err != nil {
		return cfg, fmt.Errorf("Failed to reload configs: %w", err)
	}

	for _, w := range warnings {
		c.UI.Warn(w)
	}

	if newCfg == nil {
		// this shouldnt happen but we don't want to panic just in case
		return cfg, fmt.Errorf("Failed to reload configs")
	}

	// Change the log level
	if logging.ValidateLogLevel(newCfg.LogLevel) {
		c.logger.SetLevel(logging.LevelFromString(newCfg.LogLevel))
	} else {
		errs = multierror.Append(fmt.Errorf(
			"Invalid log level: %s. Valid log levels are: %v",
			newCfg.LogLevel, logging.AllowedLogLevels()))

		// Keep the current log level
		newCfg.LogLevel = cfg.LogLevel

	}

	if err := agent.ReloadConfig(newCfg); err != nil {
		errs = multierror.Append(fmt.Errorf(
			"Failed to reload configs: %v", err))
	}

	return newCfg, errs
}

func (g *GatedUi) output(s string) {
	if !g.JSONoutput {
		g.ui.Output(s)
	}
}

func (g *GatedUi) info(s string) {
	if !g.JSONoutput {
		g.ui.Info(s)
	}
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return c.help
}

const synopsis = "Runs a Consul agent"
const help = `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.
`

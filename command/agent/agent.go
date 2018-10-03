package agent

import (
	"flag"
	"fmt"
	"io"
	"log"
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
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/service_os"
	"github.com/hashicorp/go-checkpoint"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/logutils"
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
	logFilter         *logutils.LevelFilter
	logOutput         io.Writer
	logger            *log.Logger
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	config.AddFlags(c.flags, &c.flagArgs)
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	code := c.run(args)
	if c.logger != nil {
		c.logger.Println("[INFO] agent: Exit code:", code)
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
		c.UI.Error(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}
	if results.Outdated {
		c.UI.Error(fmt.Sprintf("Newer Consul version available: %s (currently running: %s)", results.CurrentVersion, c.version))
	}
	for _, alert := range results.Alerts {
		switch alert.Level {
		case "info":
			c.UI.Info(fmt.Sprintf("Bulletin [%s]: %s (%s)", alert.Level, alert.Message, alert.URL))
		default:
			c.UI.Error(fmt.Sprintf("Bulletin [%s]: %s (%s)", alert.Level, alert.Message, alert.URL))
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

	c.UI.Info(fmt.Sprintf("Join completed. Synced with %d initial agents", n))
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

	c.UI.Info(fmt.Sprintf("Join -wan completed. Synced with %d initial agents", n))
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
	logConfig := &logger.Config{
		LogLevel:          config.LogLevel,
		EnableSyslog:      config.EnableSyslog,
		SyslogFacility:    config.SyslogFacility,
		LogFilePath:       config.LogFile,
		LogRotateDuration: config.LogRotateDuration,
		LogRotateBytes:    config.LogRotateBytes,
	}
	logFilter, logGate, logWriter, logOutput, ok := logger.Setup(logConfig, c.UI)
	if !ok {
		return 1
	}
	c.logFilter = logFilter
	c.logOutput = logOutput
	c.logger = log.New(logOutput, "", log.LstdFlags)

	// Setup gRPC logger to use the same output/filtering
	grpclog.SetLoggerV2(logger.NewGRPCLogger(logConfig, c.logger))

	memSink, err := lib.InitTelemetry(config.Telemetry)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// Create the agent
	c.UI.Output("Starting Consul agent...")
	agent, err := agent.New(config)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error creating agent: %s", err))
		return 1
	}
	agent.LogOutput = logOutput
	agent.LogWriter = logWriter
	agent.MemSink = memSink

	if err := agent.Start(); err != nil {
		c.UI.Error(fmt.Sprintf("Error starting agent: %s", err))
		return 1
	}

	// shutdown agent before endpoints
	defer agent.ShutdownEndpoints()
	defer agent.ShutdownAgent()

	if !config.DisableUpdateCheck {
		c.startupUpdateCheck(config)
	}

	if err := c.startupJoin(agent, config); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if err := c.startupJoinWan(agent, config); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	// Let the agent know we've finished registration
	agent.StartSync()

	segment := config.SegmentName
	if config.ServerMode {
		segment = "<all>"
	}

	c.UI.Output("Consul agent running!")
	c.UI.Info(fmt.Sprintf("       Version: '%s'", c.versionHuman))
	c.UI.Info(fmt.Sprintf("       Node ID: '%s'", config.NodeID))
	c.UI.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	c.UI.Info(fmt.Sprintf("    Datacenter: '%s' (Segment: '%s')", config.Datacenter, segment))
	c.UI.Info(fmt.Sprintf("        Server: %v (Bootstrap: %v)", config.ServerMode, config.Bootstrap))
	c.UI.Info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, HTTPS: %d, gRPC: %d, DNS: %d)", config.ClientAddrs,
		config.HTTPPort, config.HTTPSPort, config.GRPCPort, config.DNSPort))
	c.UI.Info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddrLAN,
		config.SerfPortLAN, config.SerfPortWAN))
	c.UI.Info(fmt.Sprintf("       Encrypt: Gossip: %v, TLS-Outgoing: %v, TLS-Incoming: %v",
		agent.GossipEncrypted(), config.VerifyOutgoing, config.VerifyIncoming))

	// Enable log streaming
	c.UI.Info("")
	c.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// wait for signal
	signalCh := make(chan os.Signal, 10)
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
			c.logger.Println("[ERR] agent: Retry join failed: ", err)
			return 1
		case <-agent.ShutdownCh():
			// agent is already down!
			return 0
		}

		switch sig {
		case syscall.SIGPIPE:
			continue

		case syscall.SIGHUP:
			c.logger.Println("[INFO] agent: Caught signal: ", sig)

			conf, err := c.handleReload(agent, config)
			if conf != nil {
				config = conf
			}
			if err != nil {
				c.logger.Println("[ERR] agent: Reload config failed: ", err)
			}
			// Send result back if reload was called via HTTP
			if reloadErrCh != nil {
				reloadErrCh <- err
			}

		default:
			c.logger.Println("[INFO] agent: Caught signal: ", sig)

			graceful := (sig == os.Interrupt && !(config.SkipLeaveOnInt)) || (sig == syscall.SIGTERM && (config.LeaveOnTerm))
			if !graceful {
				c.logger.Println("[INFO] agent: Graceful shutdown disabled. Exiting")
				return 1
			}

			c.logger.Println("[INFO] agent: Gracefully shutting down agent...")
			gracefulCh := make(chan struct{})
			go func() {
				if err := agent.Leave(); err != nil {
					c.logger.Println("[ERR] agent: Error on leave:", err)
					return
				}
				close(gracefulCh)
			}()

			gracefulTimeout := 15 * time.Second
			select {
			case <-signalCh:
				c.logger.Printf("[INFO] agent: Caught second signal %v. Exiting\n", sig)
				return 1
			case <-time.After(gracefulTimeout):
				c.logger.Println("[INFO] agent: Timeout on graceful leave. Exiting")
				return 1
			case <-gracefulCh:
				c.logger.Println("[INFO] agent: Graceful exit completed")
				return 0
			}
		}
	}
}

// handleReload is invoked when we should reload our configs, e.g. SIGHUP
func (c *cmd) handleReload(agent *agent.Agent, cfg *config.RuntimeConfig) (*config.RuntimeConfig, error) {
	c.logger.Println("[INFO] agent: Reloading configuration...")
	var errs error
	newCfg := c.readConfig()
	if newCfg == nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed to reload configs"))
		return cfg, errs
	}

	// Change the log level
	minLevel := logutils.LogLevel(strings.ToUpper(newCfg.LogLevel))
	if logger.ValidateLevelFilter(minLevel, c.logFilter) {
		c.logFilter.SetMinLevel(minLevel)
	} else {
		errs = multierror.Append(fmt.Errorf(
			"Invalid log level: %s. Valid log levels are: %v",
			minLevel, c.logFilter.Levels))

		// Keep the current log level
		newCfg.LogLevel = cfg.LogLevel
	}

	if err := agent.ReloadConfig(newCfg); err != nil {
		errs = multierror.Append(fmt.Errorf(
			"Failed to reload configs: %v", err))
	}

	return newCfg, errs
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

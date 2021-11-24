package agent

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-checkpoint"
	"github.com/hashicorp/go-hclog"
	mcli "github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/command/cli"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/service_os"
	consulversion "github.com/hashicorp/consul/version"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{
		ui:                ui,
		revision:          consulversion.GitCommit,
		version:           consulversion.Version,
		versionPrerelease: consulversion.VersionPrerelease,
		versionHuman:      consulversion.GetHumanVersion(),
		flags:             flag.NewFlagSet("", flag.ContinueOnError),
	}
	config.AddFlags(c.flags, &c.configLoadOpts)
	c.help = flags.Usage(help, c.flags)
	return c
}

// AgentCommand is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type cmd struct {
	ui                cli.Ui
	flags             *flag.FlagSet
	http              *flags.HTTPFlags
	help              string
	revision          string
	version           string
	versionPrerelease string
	versionHuman      string
	configLoadOpts    config.LoadOpts
	logger            hclog.InterceptLogger
}

func (c *cmd) Run(args []string) int {
	code := c.run(args)
	if c.logger != nil {
		c.logger.Info("Exit code", "code", code)
	}
	return code
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

	c.logger.Info("Joining cluster")
	// NOTE: For partitioned servers you are only capable of using start join
	// to join nodes in the default partition.
	n, err := agent.JoinLAN(cfg.StartJoinAddrsLAN, agent.AgentEnterpriseMeta())
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

	c.logger.Info("Joining wan cluster")
	n, err := agent.JoinWAN(cfg.StartJoinAddrsWAN)
	if err != nil {
		return err
	}

	c.logger.Info("Join wan completed. Initial agents synced with", "agent_count", n)
	return nil
}

func (c *cmd) run(args []string) int {
	ui := &mcli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.ui,
	}

	if err := c.flags.Parse(args); err != nil {
		if !strings.Contains(err.Error(), "help requested") {
			ui.Error(fmt.Sprintf("error parsing flags: %v", err))
		}
		return 1
	}
	if len(c.flags.Args()) > 0 {
		ui.Error(fmt.Sprintf("Unexpected extra arguments: %v", c.flags.Args()))
		return 1
	}

	// FIXME: logs should always go to stderr, but previously they were sent to
	// stdout, so continue to use Stdout for now, and fix this in a future release.
	logGate := &logging.GatedWriter{Writer: c.ui.Stdout()}
	loader := func(source config.Source) (config.LoadResult, error) {
		c.configLoadOpts.DefaultConfig = source
		return config.Load(c.configLoadOpts)
	}
	bd, err := agent.NewBaseDeps(loader, logGate)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	c.logger = bd.Logger
	agent, err := agent.New(bd)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	config := bd.RuntimeConfig
	if config.Logging.LogJSON {
		// Hide all non-error output when JSON logging is enabled.
		ui.Ui = &cli.BasicUI{
			BasicUi: mcli.BasicUi{ErrorWriter: c.ui.Stderr(), Writer: ioutil.Discard},
		}
	}

	ui.Output("Starting Consul agent...")

	segment := config.SegmentName
	if config.ServerMode {
		segment = "<all>"
	}
	ui.Info(fmt.Sprintf("       Version: '%s'", c.versionHuman))
	ui.Info(fmt.Sprintf("       Node ID: '%s'", config.NodeID))
	ui.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	if ap := config.PartitionOrEmpty(); ap != "" {
		ui.Info(fmt.Sprintf("     Partition: '%s'", ap))
	}
	ui.Info(fmt.Sprintf("    Datacenter: '%s' (Segment: '%s')", config.Datacenter, segment))
	ui.Info(fmt.Sprintf("        Server: %v (Bootstrap: %v)", config.ServerMode, config.Bootstrap))
	ui.Info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, HTTPS: %d, gRPC: %d, DNS: %d)", config.ClientAddrs,
		config.HTTPPort, config.HTTPSPort, config.GRPCPort, config.DNSPort))
	ui.Info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddrLAN,
		config.SerfPortLAN, config.SerfPortWAN))
	ui.Info(fmt.Sprintf("       Encrypt: Gossip: %v, TLS-Outgoing: %v, TLS-Incoming: %v, Auto-Encrypt-TLS: %t",
		config.EncryptKey != "", config.VerifyOutgoing, config.VerifyIncoming, config.AutoEncryptTLS || config.AutoEncryptAllowTLS))
	// Enable log streaming
	ui.Output("")
	ui.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// wait for signal
	signalCh := make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			var sig os.Signal
			select {
			case s := <-signalCh:
				sig = s
			case <-ctx.Done():
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
				cancel()
				return
			}
		}
	}()

	err = agent.Start(ctx)
	signal.Stop(signalCh)
	cancel()

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
		c.logger.Error(err.Error())
		return 1
	}

	if err := c.startupJoinWan(agent, config); err != nil {
		c.logger.Error(err.Error())
		return 1
	}

	// Let the agent know we've finished registration
	agent.StartSync()

	c.logger.Info("Consul agent running!")

	// wait for signal
	signalCh = make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	for {
		var sig os.Signal
		select {
		case s := <-signalCh:
			sig = s
		case <-service_os.Shutdown_Channel():
			sig = os.Interrupt
		case err := <-agent.RetryJoinCh():
			c.logger.Error("Retry join failed", "error", err)
			return 1
		case <-agent.Failed():
			// The deferred Shutdown method will log the appropriate error
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

			err := agent.ReloadConfig()
			if err != nil {
				c.logger.Error("Reload config failed", "error", err)
			}
			config = agent.GetConfig()
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

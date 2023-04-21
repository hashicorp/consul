// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/go-checkpoint"
	"github.com/hashicorp/go-hclog"
	mcli "github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	hcpbootstrap "github.com/hashicorp/consul/agent/hcp/bootstrap"
	"github.com/hashicorp/consul/command/cli"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/service_os"
	consulversion "github.com/hashicorp/consul/version"
)

func New(ui cli.Ui) *cmd {
	buildDate, err := time.Parse(time.RFC3339, consulversion.BuildDate)
	if err != nil {
		ui.Error(fmt.Sprintf("Fatal error with internal time set; check makefile for build date %v %v \n", buildDate, err))
		return nil
	}

	c := &cmd{
		ui:                ui,
		revision:          consulversion.GitCommit,
		version:           consulversion.Version,
		versionPrerelease: consulversion.VersionPrerelease,
		versionHuman:      consulversion.GetHumanVersion(),
		buildDate:         buildDate,
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
	buildDate         time.Time
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

func (c *cmd) run(args []string) int {
	ui := &mcli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ", // Note that startupLogger also uses this prefix
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

	// wait for signal
	signalCh := make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	ctx, cancel := context.WithCancel(context.Background())

	// startup logger is a shim since we need to be about to log both before and
	// after logging is setup properly but before agent has started fully. This
	// takes care of that!
	suLogger := newStartupLogger()
	go handleStartupSignals(ctx, cancel, signalCh, suLogger)

	// See if we need to bootstrap config from HCP before we go any further with
	// agent startup. First do a preliminary load of agent configuration using the given loader.
	// This is just to peek whether bootstrapping from HCP is enabled. The result is discarded
	// on the call to agent.NewBaseDeps so that the wrapped loader takes effect.
	res, err := loader(nil)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}
	if res.RuntimeConfig.IsCloudEnabled() {
		client, err := hcp.NewClient(res.RuntimeConfig.Cloud)
		if err != nil {
			ui.Error("error building HCP HTTP client: " + err.Error())
			return 1
		}

		// We override loader with the one returned as it was modified to include HCP-provided config.
		loader, err = hcpbootstrap.LoadConfig(ctx, client, res.RuntimeConfig.DataDir, loader, ui)
		if err != nil {
			ui.Error(err.Error())
			return 1
		}
	}

	bd, err := agent.NewBaseDeps(loader, logGate, nil)
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

	// Upgrade our startupLogger to use the real logger now we have it
	suLogger.SetLogger(c.logger)

	config := bd.RuntimeConfig
	if config.Logging.LogJSON {
		// Hide all non-error output when JSON logging is enabled.
		ui.Ui = &cli.BasicUI{
			BasicUi: mcli.BasicUi{ErrorWriter: c.ui.Stderr(), Writer: io.Discard},
		}
	}

	ui.Output("Starting Consul agent...")

	segment := config.SegmentName
	if config.ServerMode {
		segment = "<all>"
	}
	ui.Info(fmt.Sprintf("          Version: '%s'", c.versionHuman))
	if strings.Contains(c.versionHuman, "dev") {
		ui.Info(fmt.Sprintf("         Revision: '%s'", c.revision))
	}
	ui.Info(fmt.Sprintf("       Build Date: '%s'", c.buildDate))
	ui.Info(fmt.Sprintf("          Node ID: '%s'", config.NodeID))
	ui.Info(fmt.Sprintf("        Node name: '%s'", config.NodeName))
	if ap := config.PartitionOrEmpty(); ap != "" {
		ui.Info(fmt.Sprintf("        Partition: '%s'", ap))
	}
	ui.Info(fmt.Sprintf("       Datacenter: '%s' (Segment: '%s')", config.Datacenter, segment))
	ui.Info(fmt.Sprintf("           Server: %v (Bootstrap: %v)", config.ServerMode, config.Bootstrap))
	ui.Info(fmt.Sprintf("      Client Addr: %v (HTTP: %d, HTTPS: %d, gRPC: %d, gRPC-TLS: %d, DNS: %d)", config.ClientAddrs,
		config.HTTPPort, config.HTTPSPort, config.GRPCPort, config.GRPCTLSPort, config.DNSPort))
	ui.Info(fmt.Sprintf("     Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddrLAN,
		config.SerfPortLAN, config.SerfPortWAN))
	ui.Info(fmt.Sprintf("Gossip Encryption: %t", config.EncryptKey != ""))
	ui.Info(fmt.Sprintf(" Auto-Encrypt-TLS: %t", config.AutoEncryptTLS || config.AutoEncryptAllowTLS))
	ui.Info(fmt.Sprintf("     ACLs Enabled: %t", config.ACLsEnabled))
	ui.Info(fmt.Sprintf("        HTTPS TLS: Verify Incoming: %t, Verify Outgoing: %t, Min Version: %s",
		config.TLS.HTTPS.VerifyIncoming, config.TLS.HTTPS.VerifyOutgoing, config.TLS.HTTPS.TLSMinVersion))
	ui.Info(fmt.Sprintf("         gRPC TLS: Verify Incoming: %t, Min Version: %s", config.TLS.GRPC.VerifyIncoming, config.TLS.GRPC.TLSMinVersion))
	ui.Info(fmt.Sprintf(" Internal RPC TLS: Verify Incoming: %t, Verify Outgoing: %t (Verify Hostname: %t), Min Version: %s",
		config.TLS.InternalRPC.VerifyIncoming, config.TLS.InternalRPC.VerifyOutgoing, config.TLS.InternalRPC.VerifyServerHostname, config.TLS.InternalRPC.TLSMinVersion))
	// Enable log streaming
	ui.Output("")
	ui.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

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

func handleStartupSignals(ctx context.Context, cancel func(), signalCh chan os.Signal, logger *startupLogger) {
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
			logger.Error("Caught", "signal", sig, "error", err)

		default:
			logger.Info("Caught", "signal", sig)
			cancel()
			return
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

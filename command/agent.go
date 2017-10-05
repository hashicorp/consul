package command

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/go-checkpoint"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/logutils"
	"github.com/mitchellh/cli"
)

// validDatacenter is used to validate a datacenter
var validDatacenter = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

// AgentCommand is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type AgentCommand struct {
	BaseCommand
	Revision          string
	Version           string
	VersionPrerelease string
	HumanVersion      string
	ShutdownCh        <-chan struct{}
	args              []string
	logFilter         *logutils.LevelFilter
	logOutput         io.Writer
	logger            *log.Logger
}

// readConfig is responsible for setup of our configuration using
// the command line and any file configs
func (cmd *AgentCommand) readConfig() *config.RuntimeConfig {
	var flags config.Flags
	fs := cmd.BaseCommand.NewFlagSet(cmd)
	config.AddFlags(fs, &flags)

	if err := cmd.BaseCommand.Parse(cmd.args); err != nil {
		if !strings.Contains(err.Error(), "help requested") {
			cmd.UI.Error(fmt.Sprintf("error parsing flags: %v", err))
		}
		return nil
	}

	b, err := config.NewBuilder(flags)
	if err != nil {
		cmd.UI.Error(err.Error())
		return nil
	}
	cfg, err := b.BuildAndValidate()
	if err != nil {
		cmd.UI.Error(err.Error())
		return nil
	}
	for _, w := range b.Warnings {
		cmd.UI.Warn(w)
	}
	return &cfg
}

// checkpointResults is used to handler periodic results from our update checker
func (cmd *AgentCommand) checkpointResults(results *checkpoint.CheckResponse, err error) {
	if err != nil {
		cmd.UI.Error(fmt.Sprintf("Failed to check for updates: %v", err))
		return
	}
	if results.Outdated {
		cmd.UI.Error(fmt.Sprintf("Newer Consul version available: %s (currently running: %s)", results.CurrentVersion, cmd.Version))
	}
	for _, alert := range results.Alerts {
		switch alert.Level {
		case "info":
			cmd.UI.Info(fmt.Sprintf("Bulletin [%s]: %s (%s)", alert.Level, alert.Message, alert.URL))
		default:
			cmd.UI.Error(fmt.Sprintf("Bulletin [%s]: %s (%s)", alert.Level, alert.Message, alert.URL))
		}
	}
}

func (cmd *AgentCommand) startupUpdateCheck(config *config.RuntimeConfig) {
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
	checkpoint.CheckInterval(updateParams, 24*time.Hour, cmd.checkpointResults)

	// Do an immediate check within the next 30 seconds
	go func() {
		time.Sleep(lib.RandomStagger(30 * time.Second))
		cmd.checkpointResults(checkpoint.Check(updateParams))
	}()
}

// startupJoin is invoked to handle any joins specified to take place at start time
func (cmd *AgentCommand) startupJoin(agent *agent.Agent, cfg *config.RuntimeConfig) error {
	if len(cfg.StartJoinAddrsLAN) == 0 {
		return nil
	}

	cmd.UI.Output("Joining cluster...")
	n, err := agent.JoinLAN(cfg.StartJoinAddrsLAN)
	if err != nil {
		return err
	}

	cmd.UI.Info(fmt.Sprintf("Join completed. Synced with %d initial agents", n))
	return nil
}

// startupJoinWan is invoked to handle any joins -wan specified to take place at start time
func (cmd *AgentCommand) startupJoinWan(agent *agent.Agent, cfg *config.RuntimeConfig) error {
	if len(cfg.StartJoinAddrsWAN) == 0 {
		return nil
	}

	cmd.UI.Output("Joining -wan cluster...")
	n, err := agent.JoinWAN(cfg.StartJoinAddrsWAN)
	if err != nil {
		return err
	}

	cmd.UI.Info(fmt.Sprintf("Join -wan completed. Synced with %d initial agents", n))
	return nil
}

func statsiteSink(config *config.RuntimeConfig, hostname string) (metrics.MetricSink, error) {
	if config.TelemetryStatsiteAddr == "" {
		return nil, nil
	}
	return metrics.NewStatsiteSink(config.TelemetryStatsiteAddr)
}

func statsdSink(config *config.RuntimeConfig, hostname string) (metrics.MetricSink, error) {
	if config.TelemetryStatsdAddr == "" {
		return nil, nil
	}
	return metrics.NewStatsdSink(config.TelemetryStatsdAddr)
}

func dogstatdSink(config *config.RuntimeConfig, hostname string) (metrics.MetricSink, error) {
	if config.TelemetryDogstatsdAddr == "" {
		return nil, nil
	}
	sink, err := datadog.NewDogStatsdSink(config.TelemetryDogstatsdAddr, hostname)
	if err != nil {
		return nil, err
	}
	sink.SetTags(config.TelemetryDogstatsdTags)
	return sink, nil
}

func circonusSink(config *config.RuntimeConfig, hostname string) (metrics.MetricSink, error) {
	if config.TelemetryCirconusAPIToken == "" && config.TelemetryCirconusSubmissionURL == "" {
		return nil, nil
	}

	cfg := &circonus.Config{}
	cfg.Interval = config.TelemetryCirconusSubmissionInterval
	cfg.CheckManager.API.TokenKey = config.TelemetryCirconusAPIToken
	cfg.CheckManager.API.TokenApp = config.TelemetryCirconusAPIApp
	cfg.CheckManager.API.URL = config.TelemetryCirconusAPIURL
	cfg.CheckManager.Check.SubmissionURL = config.TelemetryCirconusSubmissionURL
	cfg.CheckManager.Check.ID = config.TelemetryCirconusCheckID
	cfg.CheckManager.Check.ForceMetricActivation = config.TelemetryCirconusCheckForceMetricActivation
	cfg.CheckManager.Check.InstanceID = config.TelemetryCirconusCheckInstanceID
	cfg.CheckManager.Check.SearchTag = config.TelemetryCirconusCheckSearchTag
	cfg.CheckManager.Check.DisplayName = config.TelemetryCirconusCheckDisplayName
	cfg.CheckManager.Check.Tags = config.TelemetryCirconusCheckTags
	cfg.CheckManager.Broker.ID = config.TelemetryCirconusBrokerID
	cfg.CheckManager.Broker.SelectTag = config.TelemetryCirconusBrokerSelectTag

	if cfg.CheckManager.Check.DisplayName == "" {
		cfg.CheckManager.Check.DisplayName = "Consul"
	}

	if cfg.CheckManager.API.TokenApp == "" {
		cfg.CheckManager.API.TokenApp = "consul"
	}

	if cfg.CheckManager.Check.SearchTag == "" {
		cfg.CheckManager.Check.SearchTag = "service:consul"
	}

	sink, err := circonus.NewCirconusSink(cfg)
	if err != nil {
		return nil, err
	}
	sink.Start()
	return sink, nil
}

func startupTelemetry(conf *config.RuntimeConfig) (*metrics.InmemSink, error) {
	// Setup telemetry
	// Aggregate on 10 second intervals for 1 minute. Expose the
	// metrics over stderr when there is a SIGUSR1 received.
	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)
	metricsConf := metrics.DefaultConfig(conf.TelemetryMetricsPrefix)
	metricsConf.EnableHostname = !conf.TelemetryDisableHostname
	metricsConf.FilterDefault = conf.TelemetryFilterDefault
	metricsConf.AllowedPrefixes = conf.TelemetryAllowedPrefixes
	metricsConf.BlockedPrefixes = conf.TelemetryBlockedPrefixes

	var sinks metrics.FanoutSink
	addSink := func(name string, fn func(*config.RuntimeConfig, string) (metrics.MetricSink, error)) error {
		s, err := fn(conf, metricsConf.HostName)
		if err != nil {
			return err
		}
		if s != nil {
			sinks = append(sinks, s)
		}
		return nil
	}

	if err := addSink("statsite", statsiteSink); err != nil {
		return nil, err
	}
	if err := addSink("statsd", statsdSink); err != nil {
		return nil, err
	}
	if err := addSink("dogstatd", dogstatdSink); err != nil {
		return nil, err
	}
	if err := addSink("circonus", circonusSink); err != nil {
		return nil, err
	}

	if len(sinks) > 0 {
		sinks = append(sinks, memSink)
		metrics.NewGlobal(metricsConf, sinks)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, memSink)
	}
	return memSink, nil
}

func (cmd *AgentCommand) Run(args []string) int {
	code := cmd.run(args)
	if cmd.logger != nil {
		cmd.logger.Println("[INFO] Exit code:", code)
	}
	return code
}

func (cmd *AgentCommand) run(args []string) int {
	cmd.UI = &cli.PrefixedUi{
		OutputPrefix: "==> ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           cmd.UI,
	}

	// Parse our configs
	cmd.args = args
	config := cmd.readConfig()
	if config == nil {
		return 1
	}

	// Setup the log outputs
	logConfig := &logger.Config{
		LogLevel:       config.LogLevel,
		EnableSyslog:   config.EnableSyslog,
		SyslogFacility: config.SyslogFacility,
	}
	logFilter, logGate, logWriter, logOutput, ok := logger.Setup(logConfig, cmd.UI)
	if !ok {
		return 1
	}
	cmd.logFilter = logFilter
	cmd.logOutput = logOutput
	cmd.logger = log.New(logOutput, "", log.LstdFlags)

	memSink, err := startupTelemetry(config)
	if err != nil {
		cmd.UI.Error(err.Error())
		return 1
	}

	// Create the agent
	cmd.UI.Output("Starting Consul agent...")
	agent, err := agent.New(config)
	if err != nil {
		cmd.UI.Error(fmt.Sprintf("Error creating agent: %s", err))
		return 1
	}
	agent.LogOutput = logOutput
	agent.LogWriter = logWriter
	agent.MemSink = memSink

	if err := agent.Start(); err != nil {
		cmd.UI.Error(fmt.Sprintf("Error starting agent: %s", err))
		return 1
	}

	// shutdown agent before endpoints
	defer agent.ShutdownEndpoints()
	defer agent.ShutdownAgent()

	if !config.DisableUpdateCheck {
		cmd.startupUpdateCheck(config)
	}

	if err := cmd.startupJoin(agent, config); err != nil {
		cmd.UI.Error(err.Error())
		return 1
	}

	if err := cmd.startupJoinWan(agent, config); err != nil {
		cmd.UI.Error(err.Error())
		return 1
	}

	// Let the agent know we've finished registration
	agent.StartSync()

	segment := config.SegmentName
	if config.ServerMode {
		segment = "<all>"
	}

	cmd.UI.Output("Consul agent running!")
	cmd.UI.Info(fmt.Sprintf("       Version: '%s'", cmd.HumanVersion))
	cmd.UI.Info(fmt.Sprintf("       Node ID: '%s'", config.NodeID))
	cmd.UI.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	cmd.UI.Info(fmt.Sprintf("    Datacenter: '%s' (Segment: '%s')", config.Datacenter, segment))
	cmd.UI.Info(fmt.Sprintf("        Server: %v (Bootstrap: %v)", config.ServerMode, config.Bootstrap))
	cmd.UI.Info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, HTTPS: %d, DNS: %d)", config.ClientAddrs,
		config.HTTPPort, config.HTTPSPort, config.DNSPort))
	cmd.UI.Info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddrLAN,
		config.SerfPortLAN, config.SerfPortWAN))
	cmd.UI.Info(fmt.Sprintf("       Encrypt: Gossip: %v, TLS-Outgoing: %v, TLS-Incoming: %v",
		agent.GossipEncrypted(), config.VerifyOutgoing, config.VerifyIncoming))

	// Enable log streaming
	cmd.UI.Info("")
	cmd.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// wait for signal
	signalCh := make(chan os.Signal, 10)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
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
		case <-cmd.ShutdownCh:
			sig = os.Interrupt
		case err := <-agent.RetryJoinCh():
			cmd.logger.Println("[ERR] Retry join failed: ", err)
			return 1
		case <-agent.ShutdownCh():
			// agent is already down!
			return 0
		}

		switch sig {
		case syscall.SIGPIPE:
			continue

		case syscall.SIGHUP:
			cmd.logger.Println("[INFO] Caught signal: ", sig)

			conf, err := cmd.handleReload(agent, config)
			if conf != nil {
				config = conf
			}
			if err != nil {
				cmd.logger.Println("[ERR] Reload config failed: ", err)
			}
			// Send result back if reload was called via HTTP
			if reloadErrCh != nil {
				reloadErrCh <- err
			}

		default:
			cmd.logger.Println("[INFO] Caught signal: ", sig)

			graceful := (sig == os.Interrupt && !(config.SkipLeaveOnInt)) || (sig == syscall.SIGTERM && (config.LeaveOnTerm))
			if !graceful {
				cmd.logger.Println("[INFO] Graceful shutdown disabled. Exiting")
				return 1
			}

			cmd.logger.Println("[INFO] Gracefully shutting down agent...")
			gracefulCh := make(chan struct{})
			go func() {
				if err := agent.Leave(); err != nil {
					cmd.logger.Println("[ERR] Error on leave:", err)
					return
				}
				close(gracefulCh)
			}()

			gracefulTimeout := 15 * time.Second
			select {
			case <-signalCh:
				cmd.logger.Printf("[INFO] Caught second signal %v. Exiting\n", sig)
				return 1
			case <-time.After(gracefulTimeout):
				cmd.logger.Println("[INFO] Timeout on graceful leave. Exiting")
				return 1
			case <-gracefulCh:
				cmd.logger.Println("[INFO] Graceful exit completed")
				return 0
			}
		}
	}
}

// handleReload is invoked when we should reload our configs, e.g. SIGHUP
func (cmd *AgentCommand) handleReload(agent *agent.Agent, cfg *config.RuntimeConfig) (*config.RuntimeConfig, error) {
	cmd.logger.Println("[INFO] Reloading configuration...")
	var errs error
	newCfg := cmd.readConfig()
	if newCfg == nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed to reload configs"))
		return cfg, errs
	}

	// Change the log level
	minLevel := logutils.LogLevel(strings.ToUpper(newCfg.LogLevel))
	if logger.ValidateLevelFilter(minLevel, cmd.logFilter) {
		cmd.logFilter.SetMinLevel(minLevel)
	} else {
		errs = multierror.Append(fmt.Errorf(
			"Invalid log level: %s. Valid log levels are: %v",
			minLevel, cmd.logFilter.Levels))

		// Keep the current log level
		newCfg.LogLevel = cfg.LogLevel
	}

	if err := agent.ReloadConfig(newCfg); err != nil {
		errs = multierror.Append(fmt.Errorf(
			"Failed to reload configs: %v", err))
	}

	return cfg, errs
}

func (cmd *AgentCommand) Synopsis() string {
	return "Runs a Consul agent"
}

func (cmd *AgentCommand) Help() string {
	helpText := `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.

 ` + cmd.BaseCommand.Help()

	return strings.TrimSpace(helpText)
}

func printJSON(name string, v interface{}) {
	fmt.Println(name)
	b, err := json.MarshalIndent(v, "", "   ")
	if err != nil {
		fmt.Printf("%#v\n", v)
		return
	}
	fmt.Println(string(b))
}

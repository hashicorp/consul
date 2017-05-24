package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
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
	"github.com/hashicorp/consul/command/base"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/watch"
	"github.com/hashicorp/go-checkpoint"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/logutils"
	"github.com/mitchellh/cli"
)

// gracefulTimeout controls how long we wait before forcefully terminating
var gracefulTimeout = 5 * time.Second

// validDatacenter is used to validate a datacenter
var validDatacenter = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

// Command is a Command implementation that runs a Consul agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	base.Command
	Revision          string
	Version           string
	VersionPrerelease string
	HumanVersion      string
	ShutdownCh        <-chan struct{}
	args              []string
	logFilter         *logutils.LevelFilter
	logOutput         io.Writer
	agent             *Agent
}

// readConfig is responsible for setup of our configuration using
// the command line and any file configs
func (cmd *Command) readConfig() *Config {
	var cmdCfg Config
	var cfgFiles []string
	var retryInterval string
	var retryIntervalWan string
	var dnsRecursors []string
	var dev bool
	var nodeMeta []string

	f := cmd.Command.NewFlagSet(cmd)

	f.Var((*AppendSliceValue)(&cfgFiles), "config-file",
		"Path to a JSON file to read configuration from. This can be specified multiple times.")
	f.Var((*AppendSliceValue)(&cfgFiles), "config-dir",
		"Path to a directory to read configuration files from. This will read every file ending "+
			"in '.json' as configuration in this directory in alphabetical order. This can be "+
			"specified multiple times.")
	f.Var((*AppendSliceValue)(&dnsRecursors), "recursor",
		"Address of an upstream DNS server. Can be specified multiple times.")
	f.Var((*AppendSliceValue)(&nodeMeta), "node-meta",
		"An arbitrary metadata key/value pair for this node, of the format `key:value`. Can be specified multiple times.")
	f.BoolVar(&dev, "dev", false, "Starts the agent in development mode.")

	f.StringVar(&cmdCfg.LogLevel, "log-level", "", "Log level of the agent.")
	f.StringVar(&cmdCfg.NodeName, "node", "", "Name of this node. Must be unique in the cluster.")
	f.StringVar((*string)(&cmdCfg.NodeID), "node-id", "",
		"A unique ID for this node across space and time. Defaults to a randomly-generated ID"+
			" that persists in the data-dir.")
	f.BoolVar(&cmdCfg.DisableHostNodeID, "disable-host-node-id", false,
		"Setting this to true will prevent Consul from using information from the"+
			" host to generate a node ID, and will cause Consul to generate a"+
			" random node ID instead.")
	f.StringVar(&cmdCfg.Datacenter, "datacenter", "", "Datacenter of the agent.")
	f.StringVar(&cmdCfg.DataDir, "data-dir", "", "Path to a data directory to store agent state.")
	f.BoolVar(&cmdCfg.EnableUI, "ui", false, "Enables the built-in static web UI server.")
	f.StringVar(&cmdCfg.UIDir, "ui-dir", "", "Path to directory containing the web UI resources.")
	f.StringVar(&cmdCfg.PidFile, "pid-file", "", "Path to file to store agent PID.")
	f.StringVar(&cmdCfg.EncryptKey, "encrypt", "", "Provides the gossip encryption key.")

	f.BoolVar(&cmdCfg.Server, "server", false, "Switches agent to server mode.")
	f.BoolVar(&cmdCfg.NonVotingServer, "non-voting-server", false,
		"(Enterprise-only) This flag is used to make the server not participate in the Raft quorum, "+
			"and have it only receive the data replication stream. This can be used to add read scalability "+
			"to a cluster in cases where a high volume of reads to servers are needed.")
	f.BoolVar(&cmdCfg.Bootstrap, "bootstrap", false, "Sets server to bootstrap mode.")
	f.IntVar(&cmdCfg.BootstrapExpect, "bootstrap-expect", 0, "Sets server to expect bootstrap mode.")
	f.StringVar(&cmdCfg.Domain, "domain", "", "Domain to use for DNS interface.")

	f.StringVar(&cmdCfg.ClientAddr, "client", "",
		"Sets the address to bind for client access. This includes RPC, DNS, HTTP and HTTPS (if configured).")
	f.StringVar(&cmdCfg.BindAddr, "bind", "", "Sets the bind address for cluster communication.")
	f.StringVar(&cmdCfg.SerfWanBindAddr, "serf-wan-bind", "", "Address to bind Serf WAN listeners to.")
	f.StringVar(&cmdCfg.SerfLanBindAddr, "serf-lan-bind", "", "Address to bind Serf LAN listeners to.")
	f.IntVar(&cmdCfg.Ports.HTTP, "http-port", 0, "Sets the HTTP API port to listen on.")
	f.IntVar(&cmdCfg.Ports.DNS, "dns-port", 0, "DNS port to use.")
	f.StringVar(&cmdCfg.AdvertiseAddr, "advertise", "", "Sets the advertise address to use.")
	f.StringVar(&cmdCfg.AdvertiseAddrWan, "advertise-wan", "",
		"Sets address to advertise on WAN instead of -advertise address.")

	f.IntVar(&cmdCfg.Protocol, "protocol", -1,
		"Sets the protocol version. Defaults to latest.")
	f.IntVar(&cmdCfg.RaftProtocol, "raft-protocol", -1,
		"Sets the Raft protocol version. Defaults to latest.")

	f.BoolVar(&cmdCfg.EnableSyslog, "syslog", false,
		"Enables logging to syslog.")
	f.BoolVar(&cmdCfg.RejoinAfterLeave, "rejoin", false,
		"Ignores a previous leave and attempts to rejoin the cluster.")
	f.Var((*AppendSliceValue)(&cmdCfg.StartJoin), "join",
		"Address of an agent to join at start time. Can be specified multiple times.")
	f.Var((*AppendSliceValue)(&cmdCfg.StartJoinWan), "join-wan",
		"Address of an agent to join -wan at start time. Can be specified multiple times.")
	f.Var((*AppendSliceValue)(&cmdCfg.RetryJoin), "retry-join",
		"Address of an agent to join at start time with retries enabled. Can be specified multiple times.")
	f.IntVar(&cmdCfg.RetryMaxAttempts, "retry-max", 0,
		"Maximum number of join attempts. Defaults to 0, which will retry indefinitely.")
	f.StringVar(&retryInterval, "retry-interval", "",
		"Time to wait between join attempts.")
	f.StringVar(&cmdCfg.RetryJoinEC2.Region, "retry-join-ec2-region", "",
		"EC2 Region to discover servers in.")
	f.StringVar(&cmdCfg.RetryJoinEC2.TagKey, "retry-join-ec2-tag-key", "",
		"EC2 tag key to filter on for server discovery.")
	f.StringVar(&cmdCfg.RetryJoinEC2.TagValue, "retry-join-ec2-tag-value", "",
		"EC2 tag value to filter on for server discovery.")
	f.StringVar(&cmdCfg.RetryJoinGCE.ProjectName, "retry-join-gce-project-name", "",
		"Google Compute Engine project to discover servers in.")
	f.StringVar(&cmdCfg.RetryJoinGCE.ZonePattern, "retry-join-gce-zone-pattern", "",
		"Google Compute Engine region or zone to discover servers in (regex pattern).")
	f.StringVar(&cmdCfg.RetryJoinGCE.TagValue, "retry-join-gce-tag-value", "",
		"Google Compute Engine tag value to filter on for server discovery.")
	f.StringVar(&cmdCfg.RetryJoinGCE.CredentialsFile, "retry-join-gce-credentials-file", "",
		"Path to credentials JSON file to use with Google Compute Engine.")
	f.StringVar(&cmdCfg.RetryJoinAzure.TagName, "retry-join-azure-tag-name", "",
		"Azure tag name to filter on for server discovery.")
	f.StringVar(&cmdCfg.RetryJoinAzure.TagValue, "retry-join-azure-tag-value", "",
		"Azure tag value to filter on for server discovery.")
	f.Var((*AppendSliceValue)(&cmdCfg.RetryJoinWan), "retry-join-wan",
		"Address of an agent to join -wan at start time with retries enabled. "+
			"Can be specified multiple times.")
	f.IntVar(&cmdCfg.RetryMaxAttemptsWan, "retry-max-wan", 0,
		"Maximum number of join -wan attempts. Defaults to 0, which will retry indefinitely.")
	f.StringVar(&retryIntervalWan, "retry-interval-wan", "",
		"Time to wait between join -wan attempts.")

	// deprecated flags
	var dcDeprecated string
	var atlasJoin bool
	var atlasInfrastructure, atlasToken, atlasEndpoint string
	f.StringVar(&dcDeprecated, "dc", "",
		"(deprecated) Datacenter of the agent (use 'datacenter' instead).")
	f.StringVar(&atlasInfrastructure, "atlas", "",
		"(deprecated) Sets the Atlas infrastructure name, enables SCADA.")
	f.StringVar(&atlasToken, "atlas-token", "",
		"(deprecated) Provides the Atlas API token.")
	f.BoolVar(&atlasJoin, "atlas-join", false,
		"(deprecated) Enables auto-joining the Atlas cluster.")
	f.StringVar(&atlasEndpoint, "atlas-endpoint", "",
		"(deprecated) The address of the endpoint for Atlas integration.")

	if err := cmd.Command.Parse(cmd.args); err != nil {
		return nil
	}

	// check deprecated flags
	if atlasInfrastructure != "" {
		cmd.UI.Warn("WARNING: 'atlas' is deprecated")
	}
	if atlasToken != "" {
		cmd.UI.Warn("WARNING: 'atlas-token' is deprecated")
	}
	if atlasJoin {
		cmd.UI.Warn("WARNING: 'atlas-join' is deprecated")
	}
	if atlasEndpoint != "" {
		cmd.UI.Warn("WARNING: 'atlas-endpoint' is deprecated")
	}
	if dcDeprecated != "" && cmdCfg.Datacenter == "" {
		cmd.UI.Warn("WARNING: 'dc' is deprecated. Use 'datacenter' instead")
		cmdCfg.Datacenter = dcDeprecated
	}

	if retryInterval != "" {
		dur, err := time.ParseDuration(retryInterval)
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Error: %s", err))
			return nil
		}
		cmdCfg.RetryInterval = dur
	}

	if retryIntervalWan != "" {
		dur, err := time.ParseDuration(retryIntervalWan)
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Error: %s", err))
			return nil
		}
		cmdCfg.RetryIntervalWan = dur
	}

	if len(nodeMeta) > 0 {
		cmdCfg.Meta = make(map[string]string)
		for _, entry := range nodeMeta {
			key, value := parseMetaPair(entry)
			cmdCfg.Meta[key] = value
		}
	}

	cfg := DefaultConfig()
	if dev {
		cfg = DevConfig()
	}

	if len(cfgFiles) > 0 {
		fileConfig, err := ReadConfigPaths(cfgFiles)
		if err != nil {
			cmd.UI.Error(err.Error())
			return nil
		}

		cfg = MergeConfig(cfg, fileConfig)
	}

	cmdCfg.DNSRecursors = append(cmdCfg.DNSRecursors, dnsRecursors...)

	cfg = MergeConfig(cfg, &cmdCfg)

	if cfg.NodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Error determining node name: %s", err))
			return nil
		}
		cfg.NodeName = hostname
	}
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)
	if cfg.NodeName == "" {
		cmd.UI.Error("Node name can not be empty")
		return nil
	}

	// Make sure LeaveOnTerm and SkipLeaveOnInt are set to the right
	// defaults based on the agent's mode (client or server).
	if cfg.LeaveOnTerm == nil {
		cfg.LeaveOnTerm = Bool(!cfg.Server)
	}
	if cfg.SkipLeaveOnInt == nil {
		cfg.SkipLeaveOnInt = Bool(cfg.Server)
	}

	// Ensure we have a data directory if we are not in dev mode.
	if !dev {
		if cfg.DataDir == "" {
			cmd.UI.Error("Must specify data directory using -data-dir")
			return nil
		}

		if finfo, err := os.Stat(cfg.DataDir); err != nil {
			if !os.IsNotExist(err) {
				cmd.UI.Error(fmt.Sprintf("Error getting data-dir: %s", err))
				return nil
			}
		} else if !finfo.IsDir() {
			cmd.UI.Error(fmt.Sprintf("The data-dir specified at %q is not a directory", cfg.DataDir))
			return nil
		}
	}

	// Ensure all endpoints are unique
	if err := cfg.verifyUniqueListeners(); err != nil {
		cmd.UI.Error(fmt.Sprintf("All listening endpoints must be unique: %s", err))
		return nil
	}

	// Check the data dir for signs of an un-migrated Consul 0.5.x or older
	// server. Consul refuses to start if this is present to protect a server
	// with existing data from starting on a fresh data set.
	if cfg.Server {
		mdbPath := filepath.Join(cfg.DataDir, "mdb")
		if _, err := os.Stat(mdbPath); !os.IsNotExist(err) {
			if os.IsPermission(err) {
				cmd.UI.Error(fmt.Sprintf("CRITICAL: Permission denied for data folder at %q!", mdbPath))
				cmd.UI.Error("Consul will refuse to boot without access to this directory.")
				cmd.UI.Error("Please correct permissions and try starting again.")
				return nil
			}
			cmd.UI.Error(fmt.Sprintf("CRITICAL: Deprecated data folder found at %q!", mdbPath))
			cmd.UI.Error("Consul will refuse to boot with this directory present.")
			cmd.UI.Error("See https://www.consul.io/docs/upgrade-specific.html for more information.")
			return nil
		}
	}

	// Verify DNS settings
	if cfg.DNSConfig.UDPAnswerLimit < 1 {
		cmd.UI.Error(fmt.Sprintf("dns_config.udp_answer_limit %d too low, must always be greater than zero", cfg.DNSConfig.UDPAnswerLimit))
	}

	if cfg.EncryptKey != "" {
		if _, err := cfg.EncryptBytes(); err != nil {
			cmd.UI.Error(fmt.Sprintf("Invalid encryption key: %s", err))
			return nil
		}
		keyfileLAN := filepath.Join(cfg.DataDir, serfLANKeyring)
		if _, err := os.Stat(keyfileLAN); err == nil {
			cmd.UI.Error("WARNING: LAN keyring exists but -encrypt given, using keyring")
		}
		if cfg.Server {
			keyfileWAN := filepath.Join(cfg.DataDir, serfWANKeyring)
			if _, err := os.Stat(keyfileWAN); err == nil {
				cmd.UI.Error("WARNING: WAN keyring exists but -encrypt given, using keyring")
			}
		}
	}

	// Ensure the datacenter is always lowercased. The DNS endpoints automatically
	// lowercase all queries, and internally we expect DC1 and dc1 to be the same.
	cfg.Datacenter = strings.ToLower(cfg.Datacenter)

	// Verify datacenter is valid
	if !validDatacenter.MatchString(cfg.Datacenter) {
		cmd.UI.Error("Datacenter must be alpha-numeric with underscores and hypens only")
		return nil
	}

	// If 'acl_datacenter' is set, ensure it is lowercased.
	if cfg.ACLDatacenter != "" {
		cfg.ACLDatacenter = strings.ToLower(cfg.ACLDatacenter)

		// Verify 'acl_datacenter' is valid
		if !validDatacenter.MatchString(cfg.ACLDatacenter) {
			cmd.UI.Error("ACL datacenter must be alpha-numeric with underscores and hypens only")
			return nil
		}
	}

	// Only allow bootstrap mode when acting as a server
	if cfg.Bootstrap && !cfg.Server {
		cmd.UI.Error("Bootstrap mode cannot be enabled when server mode is not enabled")
		return nil
	}

	// Expect can only work when acting as a server
	if cfg.BootstrapExpect != 0 && !cfg.Server {
		cmd.UI.Error("Expect mode cannot be enabled when server mode is not enabled")
		return nil
	}

	// Expect can only work when dev mode is off
	if cfg.BootstrapExpect > 0 && cfg.DevMode {
		cmd.UI.Error("Expect mode cannot be enabled when dev mode is enabled")
		return nil
	}

	// Expect & Bootstrap are mutually exclusive
	if cfg.BootstrapExpect != 0 && cfg.Bootstrap {
		cmd.UI.Error("Bootstrap cannot be provided with an expected server count")
		return nil
	}

	if ipaddr.IsAny(cfg.AdvertiseAddr) {
		cmd.UI.Error("Advertise address cannot be " + cfg.AdvertiseAddr)
		return nil
	}

	if ipaddr.IsAny(cfg.AdvertiseAddrWan) {
		cmd.UI.Error("Advertise WAN address cannot be " + cfg.AdvertiseAddrWan)
		return nil
	}

	// Compile all the watches
	for _, params := range cfg.Watches {
		// Parse the watches, excluding the handler
		wp, err := watch.ParseExempt(params, []string{"handler"})
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Failed to parse watch (%#v): %v", params, err))
			return nil
		}

		// Get the handler
		if err := verifyWatchHandler(wp.Exempt["handler"]); err != nil {
			cmd.UI.Error(fmt.Sprintf("Failed to setup watch handler (%#v): %v", params, err))
			return nil
		}

		// Store the watch plan
		cfg.WatchPlans = append(cfg.WatchPlans, wp)
	}

	// Warn if we are in expect mode
	if cfg.BootstrapExpect == 1 {
		cmd.UI.Error("WARNING: BootstrapExpect Mode is specified as 1; this is the same as Bootstrap mode.")
		cfg.BootstrapExpect = 0
		cfg.Bootstrap = true
	} else if cfg.BootstrapExpect > 0 {
		cmd.UI.Error(fmt.Sprintf("WARNING: Expect Mode enabled, expecting %d servers", cfg.BootstrapExpect))
	}

	// Warn if we are in bootstrap mode
	if cfg.Bootstrap {
		cmd.UI.Error("WARNING: Bootstrap mode enabled! Do not enable unless necessary")
	}

	// Need both tag key and value for EC2 discovery
	if cfg.RetryJoinEC2.TagKey != "" || cfg.RetryJoinEC2.TagValue != "" {
		if cfg.RetryJoinEC2.TagKey == "" || cfg.RetryJoinEC2.TagValue == "" {
			cmd.UI.Error("tag key and value are both required for EC2 retry-join")
			return nil
		}
	}

	// EC2 and GCE discovery are mutually exclusive
	if cfg.RetryJoinEC2.TagKey != "" && cfg.RetryJoinEC2.TagValue != "" && cfg.RetryJoinGCE.TagValue != "" {
		cmd.UI.Error("EC2 and GCE discovery are mutually exclusive. Please provide one or the other.")
		return nil
	}

	// Verify the node metadata entries are valid
	if err := structs.ValidateMetadata(cfg.Meta); err != nil {
		cmd.UI.Error(fmt.Sprintf("Failed to parse node metadata: %v", err))
	}

	// It doesn't make sense to include both UI options.
	if cfg.EnableUI == true && cfg.UIDir != "" {
		cmd.UI.Error("Both the ui and ui-dir flags were specified, please provide only one")
		cmd.UI.Error("If trying to use your own web UI resources, use the ui-dir flag")
		cmd.UI.Error("If using Consul version 0.7.0 or later, the web UI is included in the binary so use ui to enable it")
		return nil
	}

	// Set the version info
	cfg.Revision = cmd.Revision
	cfg.Version = cmd.Version
	cfg.VersionPrerelease = cmd.VersionPrerelease

	return cfg
}

// checkpointResults is used to handler periodic results from our update checker
func (cmd *Command) checkpointResults(results *checkpoint.CheckResponse, err error) {
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

// startupJoin is invoked to handle any joins specified to take place at start time
func (cmd *Command) startupJoin(cfg *Config) error {
	if len(cfg.StartJoin) == 0 {
		return nil
	}

	cmd.UI.Output("Joining cluster...")
	n, err := cmd.agent.JoinLAN(cfg.StartJoin)
	if err != nil {
		return err
	}

	cmd.UI.Info(fmt.Sprintf("Join completed. Synced with %d initial agents", n))
	return nil
}

// startupJoinWan is invoked to handle any joins -wan specified to take place at start time
func (cmd *Command) startupJoinWan(cfg *Config) error {
	if len(cfg.StartJoinWan) == 0 {
		return nil
	}

	cmd.UI.Output("Joining -wan cluster...")
	n, err := cmd.agent.JoinWAN(cfg.StartJoinWan)
	if err != nil {
		return err
	}

	cmd.UI.Info(fmt.Sprintf("Join -wan completed. Synced with %d initial agents", n))
	return nil
}

// retryJoin is used to handle retrying a join until it succeeds or all
// retries are exhausted.
func (cmd *Command) retryJoin(cfg *Config, errCh chan<- struct{}) {
	ec2Enabled := cfg.RetryJoinEC2.TagKey != "" && cfg.RetryJoinEC2.TagValue != ""
	gceEnabled := cfg.RetryJoinGCE.TagValue != ""
	azureEnabled := cfg.RetryJoinAzure.TagName != "" && cfg.RetryJoinAzure.TagValue != ""

	if len(cfg.RetryJoin) == 0 && !ec2Enabled && !gceEnabled && !azureEnabled {
		return
	}

	logger := cmd.agent.logger
	logger.Printf("[INFO] agent: Joining cluster...")

	attempt := 0
	for {
		var servers []string
		var err error
		switch {
		case ec2Enabled:
			servers, err = cfg.discoverEc2Hosts(logger)
			if err != nil {
				logger.Printf("[ERROR] agent: Unable to query EC2 instances: %s", err)
			}
			logger.Printf("[INFO] agent: Discovered %d servers from EC2", len(servers))
		case gceEnabled:
			servers, err = cfg.discoverGCEHosts(logger)
			if err != nil {
				logger.Printf("[ERROR] agent: Unable to query GCE instances: %s", err)
			}
			logger.Printf("[INFO] agent: Discovered %d servers from GCE", len(servers))
		case azureEnabled:
			servers, err = cfg.discoverAzureHosts(logger)
			if err != nil {
				logger.Printf("[ERROR] agent: Unable to query Azure instances: %s", err)
			}
			logger.Printf("[INFO] agent: Discovered %d servers from Azure", len(servers))
		}

		servers = append(servers, cfg.RetryJoin...)
		if len(servers) == 0 {
			err = fmt.Errorf("No servers to join")
		} else {
			n, err := cmd.agent.JoinLAN(servers)
			if err == nil {
				logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
				return
			}
		}

		attempt++
		if cfg.RetryMaxAttempts > 0 && attempt > cfg.RetryMaxAttempts {
			logger.Printf("[ERROR] agent: max join retry exhausted, exiting")
			close(errCh)
			return
		}

		logger.Printf("[WARN] agent: Join failed: %v, retrying in %v", err,
			cfg.RetryInterval)
		time.Sleep(cfg.RetryInterval)
	}
}

// retryJoinWan is used to handle retrying a join -wan until it succeeds or all
// retries are exhausted.
func (cmd *Command) retryJoinWan(cfg *Config, errCh chan<- struct{}) {
	if len(cfg.RetryJoinWan) == 0 {
		return
	}

	logger := cmd.agent.logger
	logger.Printf("[INFO] agent: Joining WAN cluster...")

	attempt := 0
	for {
		n, err := cmd.agent.JoinWAN(cfg.RetryJoinWan)
		if err == nil {
			logger.Printf("[INFO] agent: Join -wan completed. Synced with %d initial agents", n)
			return
		}

		attempt++
		if cfg.RetryMaxAttemptsWan > 0 && attempt > cfg.RetryMaxAttemptsWan {
			logger.Printf("[ERROR] agent: max join -wan retry exhausted, exiting")
			close(errCh)
			return
		}

		logger.Printf("[WARN] agent: Join -wan failed: %v, retrying in %v", err,
			cfg.RetryIntervalWan)
		time.Sleep(cfg.RetryIntervalWan)
	}
}

// gossipEncrypted determines if the consul instance is using symmetric
// encryption keys to protect gossip protocol messages.
func (cmd *Command) gossipEncrypted() bool {
	if cmd.agent.config.EncryptKey != "" {
		return true
	}

	server, ok := cmd.agent.delegate.(*consul.Server)
	if ok {
		return server.KeyManagerLAN() != nil || server.KeyManagerWAN() != nil
	}
	client, ok := cmd.agent.delegate.(*consul.Client)
	if ok {
		return client != nil && client.KeyManagerLAN() != nil
	}
	panic(fmt.Sprintf("delegate is neither server nor client: %T", cmd.agent.delegate))
}

func (cmd *Command) Run(args []string) int {
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

	// Setup telemetry
	// Aggregate on 10 second intervals for 1 minute. Expose the
	// metrics over stderr when there is a SIGUSR1 received.
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(inm)
	metricsConf := metrics.DefaultConfig(config.Telemetry.StatsitePrefix)
	metricsConf.EnableHostname = !config.Telemetry.DisableHostname

	// Configure the statsite sink
	var fanout metrics.FanoutSink
	if config.Telemetry.StatsiteAddr != "" {
		sink, err := metrics.NewStatsiteSink(config.Telemetry.StatsiteAddr)
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Failed to start statsite sink. Got: %s", err))
			return 1
		}
		fanout = append(fanout, sink)
	}

	// Configure the statsd sink
	if config.Telemetry.StatsdAddr != "" {
		sink, err := metrics.NewStatsdSink(config.Telemetry.StatsdAddr)
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Failed to start statsd sink. Got: %s", err))
			return 1
		}
		fanout = append(fanout, sink)
	}

	// Configure the DogStatsd sink
	if config.Telemetry.DogStatsdAddr != "" {
		var tags []string

		if config.Telemetry.DogStatsdTags != nil {
			tags = config.Telemetry.DogStatsdTags
		}

		sink, err := datadog.NewDogStatsdSink(config.Telemetry.DogStatsdAddr, metricsConf.HostName)
		if err != nil {
			cmd.UI.Error(fmt.Sprintf("Failed to start DogStatsd sink. Got: %s", err))
			return 1
		}
		sink.SetTags(tags)
		fanout = append(fanout, sink)
	}

	if config.Telemetry.CirconusAPIToken != "" || config.Telemetry.CirconusCheckSubmissionURL != "" {
		cfg := &circonus.Config{}
		cfg.Interval = config.Telemetry.CirconusSubmissionInterval
		cfg.CheckManager.API.TokenKey = config.Telemetry.CirconusAPIToken
		cfg.CheckManager.API.TokenApp = config.Telemetry.CirconusAPIApp
		cfg.CheckManager.API.URL = config.Telemetry.CirconusAPIURL
		cfg.CheckManager.Check.SubmissionURL = config.Telemetry.CirconusCheckSubmissionURL
		cfg.CheckManager.Check.ID = config.Telemetry.CirconusCheckID
		cfg.CheckManager.Check.ForceMetricActivation = config.Telemetry.CirconusCheckForceMetricActivation
		cfg.CheckManager.Check.InstanceID = config.Telemetry.CirconusCheckInstanceID
		cfg.CheckManager.Check.SearchTag = config.Telemetry.CirconusCheckSearchTag
		cfg.CheckManager.Check.DisplayName = config.Telemetry.CirconusCheckDisplayName
		cfg.CheckManager.Check.Tags = config.Telemetry.CirconusCheckTags
		cfg.CheckManager.Broker.ID = config.Telemetry.CirconusBrokerID
		cfg.CheckManager.Broker.SelectTag = config.Telemetry.CirconusBrokerSelectTag

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
			cmd.UI.Error(fmt.Sprintf("Failed to start Circonus sink. Got: %s", err))
			return 1
		}
		sink.Start()
		fanout = append(fanout, sink)
	}

	// Initialize the global sink
	if len(fanout) > 0 {
		fanout = append(fanout, inm)
		metrics.NewGlobal(metricsConf, fanout)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, inm)
	}

	// Create the agent
	cmd.UI.Output("Starting Consul agent...")
	agent, err := NewAgent(config)
	if err != nil {
		cmd.UI.Error(fmt.Sprintf("Error creating agent: %s", err))
		return 1
	}
	agent.LogOutput = logOutput
	agent.LogWriter = logWriter
	if err := agent.Start(); err != nil {
		cmd.UI.Error(fmt.Sprintf("Error starting agent: %s", err))
		return 1
	}
	cmd.agent = agent

	// Setup update checking
	if !config.DisableUpdateCheck {
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

	defer cmd.agent.Shutdown()

	// Join startup nodes if specified
	if err := cmd.startupJoin(config); err != nil {
		cmd.UI.Error(err.Error())
		return 1
	}

	// Join startup nodes if specified
	if err := cmd.startupJoinWan(config); err != nil {
		cmd.UI.Error(err.Error())
		return 1
	}

	// Get the new client http listener addr
	var httpAddr net.Addr
	if config.Ports.HTTP != -1 {
		httpAddr, err = config.ClientListener(config.Addresses.HTTP, config.Ports.HTTP)
	} else if config.Ports.HTTPS != -1 {
		httpAddr, err = config.ClientListener(config.Addresses.HTTPS, config.Ports.HTTPS)
	} else if len(config.WatchPlans) > 0 {
		cmd.UI.Error("Error: cannot use watches if both HTTP and HTTPS are disabled")
		return 1
	}
	if err != nil {
		cmd.UI.Error(fmt.Sprintf("Failed to determine HTTP address: %v", err))
	}

	// Register the watches
	for _, wp := range config.WatchPlans {
		go func(wp *watch.Plan) {
			wp.Handler = makeWatchHandler(logOutput, wp.Exempt["handler"])
			wp.LogOutput = cmd.logOutput
			addr := httpAddr.String()
			// If it's a unix socket, prefix with unix:// so the client initializes correctly
			if httpAddr.Network() == "unix" {
				addr = "unix://" + addr
			}
			if err := wp.Run(addr); err != nil {
				cmd.UI.Error(fmt.Sprintf("Error running watch: %v", err))
			}
		}(wp)
	}

	// Figure out if gossip is encrypted
	gossipEncrypted := cmd.agent.delegate.Encrypted()

	// Let the agent know we've finished registration
	cmd.agent.StartSync()

	cmd.UI.Output("Consul agent running!")
	cmd.UI.Info(fmt.Sprintf("       Version: '%s'", cmd.HumanVersion))
	cmd.UI.Info(fmt.Sprintf("       Node ID: '%s'", config.NodeID))
	cmd.UI.Info(fmt.Sprintf("     Node name: '%s'", config.NodeName))
	cmd.UI.Info(fmt.Sprintf("    Datacenter: '%s'", config.Datacenter))
	cmd.UI.Info(fmt.Sprintf("        Server: %v (bootstrap: %v)", config.Server, config.Bootstrap))
	cmd.UI.Info(fmt.Sprintf("   Client Addr: %v (HTTP: %d, HTTPS: %d, DNS: %d)", config.ClientAddr,
		config.Ports.HTTP, config.Ports.HTTPS, config.Ports.DNS))
	cmd.UI.Info(fmt.Sprintf("  Cluster Addr: %v (LAN: %d, WAN: %d)", config.AdvertiseAddr,
		config.Ports.SerfLan, config.Ports.SerfWan))
	cmd.UI.Info(fmt.Sprintf("Gossip encrypt: %v, RPC-TLS: %v, TLS-Incoming: %v",
		gossipEncrypted, config.VerifyOutgoing, config.VerifyIncoming))

	// Enable log streaming
	cmd.UI.Info("")
	cmd.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// Start retry join process
	errCh := make(chan struct{})
	go cmd.retryJoin(config, errCh)

	// Start retry -wan join process
	errWanCh := make(chan struct{})
	go cmd.retryJoinWan(config, errWanCh)

	// Wait for exit
	return cmd.handleSignals(config, errCh, errWanCh)
}

// handleSignals blocks until we get an exit-causing signal
func (cmd *Command) handleSignals(cfg *Config, retryJoin <-chan struct{}, retryJoinWan <-chan struct{}) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGPIPE)

	// Wait for a signal
WAIT:
	var sig os.Signal
	var reloadErrCh chan error
	select {
	case s := <-signalCh:
		sig = s
	case ch := <-cmd.agent.reloadCh:
		sig = syscall.SIGHUP
		reloadErrCh = ch
	case <-cmd.ShutdownCh:
		sig = os.Interrupt
	case <-retryJoin:
		return 1
	case <-retryJoinWan:
		return 1
	case <-cmd.agent.ShutdownCh():
		// Agent is already shutdown!
		return 0
	}

	// Skip SIGPIPE signals and skip logging whenever such signal is received as well
	if sig == syscall.SIGPIPE {
		goto WAIT
	}

	cmd.UI.Output(fmt.Sprintf("Caught signal: %v", sig))

	// Check if this is a SIGHUP
	if sig == syscall.SIGHUP {
		conf, err := cmd.handleReload(cfg)
		if conf != nil {
			cfg = conf
		}
		if err != nil {
			cmd.UI.Error(err.Error())
		}
		// Send result back if reload was called via HTTP
		if reloadErrCh != nil {
			reloadErrCh <- err
		}
		goto WAIT
	}

	// Check if we should do a graceful leave
	graceful := false
	if sig == os.Interrupt && !(*cfg.SkipLeaveOnInt) {
		graceful = true
	} else if sig == syscall.SIGTERM && (*cfg.LeaveOnTerm) {
		graceful = true
	}

	// Bail fast if not doing a graceful leave
	if !graceful {
		return 1
	}

	// Attempt a graceful leave
	gracefulCh := make(chan struct{})
	cmd.UI.Output("Gracefully shutting down agent...")
	go func() {
		if err := cmd.agent.Leave(); err != nil {
			cmd.UI.Error(fmt.Sprintf("Error: %s", err))
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
func (cmd *Command) handleReload(cfg *Config) (*Config, error) {
	cmd.UI.Output("Reloading configuration...")
	var errs error
	newConf := cmd.readConfig()
	if newConf == nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed to reload configs"))
		return cfg, errs
	}

	// Change the log level
	minLevel := logutils.LogLevel(strings.ToUpper(newConf.LogLevel))
	if logger.ValidateLevelFilter(minLevel, cmd.logFilter) {
		cmd.logFilter.SetMinLevel(minLevel)
	} else {
		errs = multierror.Append(fmt.Errorf(
			"Invalid log level: %s. Valid log levels are: %v",
			minLevel, cmd.logFilter.Levels))

		// Keep the current log level
		newConf.LogLevel = cfg.LogLevel
	}

	// Bulk update the services and checks
	cmd.agent.PauseSync()
	defer cmd.agent.ResumeSync()

	// Snapshot the current state, and restore it afterwards
	snap := cmd.agent.snapshotCheckState()
	defer cmd.agent.restoreCheckState(snap)

	// First unload all checks, services, and metadata. This lets us begin the reload
	// with a clean slate.
	if err := cmd.agent.unloadServices(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed unloading services: %s", err))
		return nil, errs
	}
	if err := cmd.agent.unloadChecks(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed unloading checks: %s", err))
		return nil, errs
	}
	cmd.agent.unloadMetadata()

	// Reload service/check definitions and metadata.
	if err := cmd.agent.loadServices(newConf); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed reloading services: %s", err))
		return nil, errs
	}
	if err := cmd.agent.loadChecks(newConf); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed reloading checks: %s", err))
		return nil, errs
	}
	if err := cmd.agent.loadMetadata(newConf); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed reloading metadata: %s", err))
		return nil, errs
	}

	// Get the new client listener addr
	httpAddr, err := newConf.ClientListener(cfg.Addresses.HTTP, cfg.Ports.HTTP)
	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Failed to determine HTTP address: %v", err))
	}

	// Deregister the old watches
	for _, wp := range cfg.WatchPlans {
		wp.Stop()
	}

	// Register the new watches
	for _, wp := range newConf.WatchPlans {
		go func(wp *watch.Plan) {
			wp.Handler = makeWatchHandler(cmd.logOutput, wp.Exempt["handler"])
			wp.LogOutput = cmd.logOutput
			if err := wp.Run(httpAddr.String()); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("Error running watch: %v", err))
			}
		}(wp)
	}

	return newConf, errs
}

func (cmd *Command) Synopsis() string {
	return "Runs a Consul agent"
}

func (cmd *Command) Help() string {
	helpText := `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.

 ` + cmd.Command.Help()

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

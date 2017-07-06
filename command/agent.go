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
	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/configutil"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/watch"
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
func (cmd *AgentCommand) readConfig() *agent.Config {
	var cmdCfg agent.Config
	var cfgFiles []string
	var retryInterval string
	var retryIntervalWan string
	var dnsRecursors []string
	var dev bool
	var nodeMeta []string

	f := cmd.BaseCommand.NewFlagSet(cmd)

	f.Var((*configutil.AppendSliceValue)(&cfgFiles), "config-file",
		"Path to a JSON file to read configuration from. This can be specified multiple times.")
	f.Var((*configutil.AppendSliceValue)(&cfgFiles), "config-dir",
		"Path to a directory to read configuration files from. This will read every file ending "+
			"in '.json' as configuration in this directory in alphabetical order. This can be "+
			"specified multiple times.")
	f.Var((*configutil.AppendSliceValue)(&dnsRecursors), "recursor",
		"Address of an upstream DNS server. Can be specified multiple times.")
	f.Var((*configutil.AppendSliceValue)(&nodeMeta), "node-meta",
		"An arbitrary metadata key/value pair for this node, of the format `key:value`. Can be specified multiple times.")
	f.BoolVar(&dev, "dev", false, "Starts the agent in development mode.")

	f.StringVar(&cmdCfg.LogLevel, "log-level", "", "Log level of the agent.")
	f.StringVar(&cmdCfg.NodeName, "node", "", "Name of this node. Must be unique in the cluster.")
	f.StringVar((*string)(&cmdCfg.NodeID), "node-id", "",
		"A unique ID for this node across space and time. Defaults to a randomly-generated ID"+
			" that persists in the data-dir.")

	var disableHostNodeID configutil.BoolValue
	f.Var(&disableHostNodeID, "disable-host-node-id",
		"Setting this to true will prevent Consul from using information from the"+
			" host to generate a node ID, and will cause Consul to generate a"+
			" random node ID instead.")

	f.StringVar(&cmdCfg.Datacenter, "datacenter", "", "Datacenter of the agent.")
	f.StringVar(&cmdCfg.DataDir, "data-dir", "", "Path to a data directory to store agent state.")
	f.BoolVar(&cmdCfg.EnableUI, "ui", false, "Enables the built-in static web UI server.")
	f.StringVar(&cmdCfg.UIDir, "ui-dir", "", "Path to directory containing the web UI resources.")
	f.StringVar(&cmdCfg.PidFile, "pid-file", "", "Path to file to store agent PID.")
	f.StringVar(&cmdCfg.EncryptKey, "encrypt", "", "Provides the gossip encryption key.")
	f.BoolVar(&cmdCfg.DisableKeyringFile, "disable-keyring-file", false, "Disables the backing up "+
		"of the keyring to a file.")

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
	f.Var((*configutil.AppendSliceValue)(&cmdCfg.StartJoin), "join",
		"Address of an agent to join at start time. Can be specified multiple times.")
	f.Var((*configutil.AppendSliceValue)(&cmdCfg.StartJoinWan), "join-wan",
		"Address of an agent to join -wan at start time. Can be specified multiple times.")
	f.Var((*configutil.AppendSliceValue)(&cmdCfg.RetryJoin), "retry-join",
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
	f.Var((*configutil.AppendSliceValue)(&cmdCfg.RetryJoinWan), "retry-join-wan",
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

	if err := cmd.BaseCommand.Parse(cmd.args); err != nil {
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
			key, value := agent.ParseMetaPair(entry)
			cmdCfg.Meta[key] = value
		}
	}

	cfg := agent.DefaultConfig()
	if dev {
		cfg = agent.DevConfig()
	}

	if len(cfgFiles) > 0 {
		fileConfig, err := agent.ReadConfigPaths(cfgFiles)
		if err != nil {
			cmd.UI.Error(err.Error())
			return nil
		}

		cfg = agent.MergeConfig(cfg, fileConfig)
	}

	cmdCfg.DNSRecursors = append(cmdCfg.DNSRecursors, dnsRecursors...)

	cfg = agent.MergeConfig(cfg, &cmdCfg)
	disableHostNodeID.Merge(cfg.DisableHostNodeID)

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
		cfg.LeaveOnTerm = agent.Bool(!cfg.Server)
	}
	if cfg.SkipLeaveOnInt == nil {
		cfg.SkipLeaveOnInt = agent.Bool(cfg.Server)
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
	if err := cfg.VerifyUniqueListeners(); err != nil {
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
		keyfileLAN := filepath.Join(cfg.DataDir, agent.SerfLANKeyring)
		if _, err := os.Stat(keyfileLAN); err == nil {
			cmd.UI.Error("WARNING: LAN keyring exists but -encrypt given, using keyring")
		}
		if cfg.Server {
			keyfileWAN := filepath.Join(cfg.DataDir, agent.SerfWANKeyring)
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
		h := wp.Exempt["handler"]
		if _, ok := h.(string); h == nil || !ok {
			cmd.UI.Error("Watch handler must be a string")
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

	// Warn if we are expecting an even number of servers
	if cfg.BootstrapExpect != 0 && cfg.BootstrapExpect%2 == 0 {
		if cfg.BootstrapExpect == 2 {
			cmd.UI.Error("WARNING: A cluster with 2 servers will provide no failure tolerance.  See https://www.consul.io/docs/internals/consensus.html#deployment-table")
		} else {
			cmd.UI.Error("WARNING: A cluster with an even number of servers does not achieve optimum fault tolerance.  See https://www.consul.io/docs/internals/consensus.html#deployment-table")
		}
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

func (cmd *AgentCommand) startupUpdateCheck(config *agent.Config) {
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
func (cmd *AgentCommand) startupJoin(agent *agent.Agent, cfg *agent.Config) error {
	if len(cfg.StartJoin) == 0 {
		return nil
	}

	cmd.UI.Output("Joining cluster...")
	n, err := agent.JoinLAN(cfg.StartJoin)
	if err != nil {
		return err
	}

	cmd.UI.Info(fmt.Sprintf("Join completed. Synced with %d initial agents", n))
	return nil
}

// startupJoinWan is invoked to handle any joins -wan specified to take place at start time
func (cmd *AgentCommand) startupJoinWan(agent *agent.Agent, cfg *agent.Config) error {
	if len(cfg.StartJoinWan) == 0 {
		return nil
	}

	cmd.UI.Output("Joining -wan cluster...")
	n, err := agent.JoinWAN(cfg.StartJoinWan)
	if err != nil {
		return err
	}

	cmd.UI.Info(fmt.Sprintf("Join -wan completed. Synced with %d initial agents", n))
	return nil
}

func statsiteSink(config *agent.Config, hostname string) (metrics.MetricSink, error) {
	if config.Telemetry.StatsiteAddr == "" {
		return nil, nil
	}
	return metrics.NewStatsiteSink(config.Telemetry.StatsiteAddr)
}

func statsdSink(config *agent.Config, hostname string) (metrics.MetricSink, error) {
	if config.Telemetry.StatsdAddr == "" {
		return nil, nil
	}
	return metrics.NewStatsdSink(config.Telemetry.StatsdAddr)
}

func dogstatdSink(config *agent.Config, hostname string) (metrics.MetricSink, error) {
	if config.Telemetry.DogStatsdAddr == "" {
		return nil, nil
	}
	sink, err := datadog.NewDogStatsdSink(config.Telemetry.DogStatsdAddr, hostname)
	if err != nil {
		return nil, err
	}
	sink.SetTags(config.Telemetry.DogStatsdTags)
	return sink, nil
}

func circonusSink(config *agent.Config, hostname string) (metrics.MetricSink, error) {
	if config.Telemetry.CirconusAPIToken == "" && config.Telemetry.CirconusCheckSubmissionURL == "" {
		return nil, nil
	}

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
		return nil, err
	}
	sink.Start()
	return sink, nil
}

func startupTelemetry(config *agent.Config) error {
	// Setup telemetry
	// Aggregate on 10 second intervals for 1 minute. Expose the
	// metrics over stderr when there is a SIGUSR1 received.
	memSink := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(memSink)
	metricsConf := metrics.DefaultConfig(config.Telemetry.StatsitePrefix)
	metricsConf.EnableHostname = !config.Telemetry.DisableHostname

	var sinks metrics.FanoutSink
	addSink := func(name string, fn func(*agent.Config, string) (metrics.MetricSink, error)) error {
		s, err := fn(config, metricsConf.HostName)
		if err != nil {
			return err
		}
		if s != nil {
			sinks = append(sinks, s)
		}
		return nil
	}

	if err := addSink("statsite", statsiteSink); err != nil {
		return err
	}
	if err := addSink("statsd", statsdSink); err != nil {
		return err
	}
	if err := addSink("dogstatd", dogstatdSink); err != nil {
		return err
	}
	if err := addSink("circonus", circonusSink); err != nil {
		return err
	}

	if len(sinks) > 0 {
		sinks = append(sinks, memSink)
		metrics.NewGlobal(metricsConf, sinks)
	} else {
		metricsConf.EnableHostname = false
		metrics.NewGlobal(metricsConf, memSink)
	}
	return nil
}

func (cmd *AgentCommand) Run(args []string) int {
	code := cmd.run(args)
	if cmd.logger != nil {
		cmd.logger.Println("[INFO] Exit code: ", code)
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

	if err := startupTelemetry(config); err != nil {
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
		agent.GossipEncrypted(), config.VerifyOutgoing, config.VerifyIncoming))

	// Enable log streaming
	cmd.UI.Info("")
	cmd.UI.Output("Log data will now stream in as it occurs:\n")
	logGate.Flush()

	// wait for signal
	signalCh := make(chan os.Signal, 4)
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

			graceful := (sig == os.Interrupt && !(*config.SkipLeaveOnInt)) || (sig == syscall.SIGTERM && (*config.LeaveOnTerm))
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
func (cmd *AgentCommand) handleReload(agent *agent.Agent, cfg *agent.Config) (*agent.Config, error) {
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

package config

import (
	"flag"
	"fmt"
	"time"
)

// Flags defines the command line flags.
type Flags struct {
	// Config contains the command line arguments that can also be set
	// in a config file.
	Config Config

	// ConfigFiles contains the list of config files and directories
	// that should be read.
	ConfigFiles []string

	// ConfigFormat forces all config files to be interpreted as this
	// format independent of their extension.
	ConfigFormat *string

	// HCL contains an arbitrary config in hcl format.
	// DevMode indicates whether the agent should be started in development
	// mode. This cannot be configured in a config file.
	DevMode *bool

	HCL []string

	// Args contains the remaining unparsed flags.
	Args []string
}

// AddFlags adds the command line flags for the agent.
func AddFlags(fs *flag.FlagSet, f *Flags) {
	add := func(p interface{}, name, help string) {
		switch x := p.(type) {
		case **bool:
			fs.Var(newBoolPtrValue(x), name, help)
		case **time.Duration:
			fs.Var(newDurationPtrValue(x), name, help)
		case **int:
			fs.Var(newIntPtrValue(x), name, help)
		case **string:
			fs.Var(newStringPtrValue(x), name, help)
		case *[]string:
			fs.Var(newStringSliceValue(x), name, help)
		case *map[string]string:
			fs.Var(newStringMapValue(x), name, help)
		default:
			panic(fmt.Sprintf("invalid type: %T", p))
		}
	}

	// command line flags ordered by flag name
	add(&f.Config.AdvertiseAddrLAN, "advertise", "Sets the advertise address to use.")
	add(&f.Config.AdvertiseAddrWAN, "advertise-wan", "Sets address to advertise on WAN instead of -advertise address.")
	add(&f.Config.BindAddr, "bind", "Sets the bind address for cluster communication.")
	add(&f.Config.Ports.Server, "server-port", "Sets the server port to listen on.")
	add(&f.Config.Bootstrap, "bootstrap", "Sets server to bootstrap mode.")
	add(&f.Config.BootstrapExpect, "bootstrap-expect", "Sets server to expect bootstrap mode.")
	add(&f.Config.ClientAddr, "client", "Sets the address to bind for client access. This includes RPC, DNS, HTTP, HTTPS and gRPC (if configured).")
	add(&f.Config.CheckOutputMaxSize, "check_output_max_size", "Sets the maximum output size for checks on this agent")
	add(&f.ConfigFiles, "config-dir", "Path to a directory to read configuration files from. This will read every file ending in '.json' as configuration in this directory in alphabetical order. Can be specified multiple times.")
	add(&f.ConfigFiles, "config-file", "Path to a file in JSON or HCL format with a matching file extension. Can be specified multiple times.")
	add(&f.ConfigFormat, "config-format", "Config files are in this format irrespective of their extension. Must be 'hcl' or 'json'")
	add(&f.Config.DataDir, "data-dir", "Path to a data directory to store agent state.")
	add(&f.Config.Datacenter, "datacenter", "Datacenter of the agent.")
	add(&f.Config.DefaultQueryTime, "default-query-time", "the amount of time a blocking query will wait before Consul will force a response. This value can be overridden by the 'wait' query parameter.")
	add(&f.DevMode, "dev", "Starts the agent in development mode.")
	add(&f.Config.DisableHostNodeID, "disable-host-node-id", "Setting this to true will prevent Consul from using information from the host to generate a node ID, and will cause Consul to generate a random node ID instead.")
	add(&f.Config.DisableKeyringFile, "disable-keyring-file", "Disables the backing up of the keyring to a file.")
	add(&f.Config.Ports.DNS, "dns-port", "DNS port to use.")
	add(&f.Config.DNSDomain, "domain", "Domain to use for DNS interface.")
	add(&f.Config.DNSAltDomain, "alt-domain", "Alternate domain to use for DNS interface.")
	add(&f.Config.EnableScriptChecks, "enable-script-checks", "Enables health check scripts.")
	add(&f.Config.EnableLocalScriptChecks, "enable-local-script-checks", "Enables health check scripts from configuration file.")
	add(&f.Config.HTTPConfig.AllowWriteHTTPFrom, "allow-write-http-from", "Only allow write endpoint calls from given network. CIDR format, can be specified multiple times.")
	add(&f.Config.EncryptKey, "encrypt", "Provides the gossip encryption key.")
	add(&f.Config.Ports.GRPC, "grpc-port", "Sets the gRPC API port to listen on (currently needed for Envoy xDS only).")
	add(&f.Config.Ports.HTTP, "http-port", "Sets the HTTP API port to listen on.")
	add(&f.Config.Ports.HTTPS, "https-port", "Sets the HTTPS API port to listen on.")
	add(&f.Config.StartJoinAddrsLAN, "join", "Address of an agent to join at start time. Can be specified multiple times.")
	add(&f.Config.StartJoinAddrsWAN, "join-wan", "Address of an agent to join -wan at start time. Can be specified multiple times.")
	add(&f.Config.LogLevel, "log-level", "Log level of the agent.")
	add(&f.Config.LogJSON, "log-json", "Output logs in JSON format.")
	add(&f.Config.LogFile, "log-file", "Path to the file the logs get written to")
	add(&f.Config.LogRotateBytes, "log-rotate-bytes", "Maximum number of bytes that should be written to a log file")
	add(&f.Config.LogRotateDuration, "log-rotate-duration", "Time after which log rotation needs to be performed")
	add(&f.Config.LogRotateMaxFiles, "log-rotate-max-files", "Maximum number of log file archives to keep")
	add(&f.Config.MaxQueryTime, "max-query-time", "the maximum amount of time a blocking query can wait before Consul will force a response. Consul applies jitter to the wait time. The jittered time will be capped to MaxQueryTime.")
	add(&f.Config.NodeName, "node", "Name of this node. Must be unique in the cluster.")
	add(&f.Config.NodeID, "node-id", "A unique ID for this node across space and time. Defaults to a randomly-generated ID that persists in the data-dir.")
	add(&f.Config.NodeMeta, "node-meta", "An arbitrary metadata key/value pair for this node, of the format `key:value`. Can be specified multiple times.")
	add(&f.Config.NonVotingServer, "non-voting-server", "(Enterprise-only) This flag is used to make the server not participate in the Raft quorum, and have it only receive the data replication stream. This can be used to add read scalability to a cluster in cases where a high volume of reads to servers are needed.")
	add(&f.Config.PidFile, "pid-file", "Path to file to store agent PID.")
	add(&f.Config.RPCProtocol, "protocol", "Sets the protocol version. Defaults to latest.")
	add(&f.Config.RaftProtocol, "raft-protocol", "Sets the Raft protocol version. Defaults to latest.")
	add(&f.Config.DNSRecursors, "recursor", "Address of an upstream DNS server. Can be specified multiple times.")
	add(&f.Config.PrimaryGateways, "primary-gateways", "Address of a mesh gateway in the primary datacenter to use to bootstrap WAN federation at start time with retries enabled. Can be specified multiple times.")
	add(&f.Config.PrimaryGatewaysInterval, "primary-gateways-interval", "Time to wait between discovery attempts for mesh gateways in the primary datacenter.")
	add(&f.Config.RejoinAfterLeave, "rejoin", "Ignores a previous leave and attempts to rejoin the cluster.")
	add(&f.Config.RetryJoinIntervalLAN, "retry-interval", "Time to wait between join attempts.")
	add(&f.Config.RetryJoinIntervalWAN, "retry-interval-wan", "Time to wait between join -wan attempts.")
	add(&f.Config.RetryJoinLAN, "retry-join", "Address of an agent to join at start time with retries enabled. Can be specified multiple times.")
	add(&f.Config.RetryJoinWAN, "retry-join-wan", "Address of an agent to join -wan at start time with retries enabled. Can be specified multiple times.")
	add(&f.Config.RetryJoinMaxAttemptsLAN, "retry-max", "Maximum number of join attempts. Defaults to 0, which will retry indefinitely.")
	add(&f.Config.RetryJoinMaxAttemptsWAN, "retry-max-wan", "Maximum number of join -wan attempts. Defaults to 0, which will retry indefinitely.")
	add(&f.Config.SerfBindAddrLAN, "serf-lan-bind", "Address to bind Serf LAN listeners to.")
	add(&f.Config.Ports.SerfLAN, "serf-lan-port", "Sets the Serf LAN port to listen on.")
	add(&f.Config.SegmentName, "segment", "(Enterprise-only) Sets the network segment to join.")
	add(&f.Config.SerfBindAddrWAN, "serf-wan-bind", "Address to bind Serf WAN listeners to.")
	add(&f.Config.Ports.SerfWAN, "serf-wan-port", "Sets the Serf WAN port to listen on.")
	add(&f.Config.ServerMode, "server", "Switches agent to server mode.")
	add(&f.Config.EnableSyslog, "syslog", "Enables logging to syslog.")
	add(&f.Config.UI, "ui", "Enables the built-in static web UI server.")
	add(&f.Config.UIContentPath, "ui-content-path", "Sets the external UI path to a string. Defaults to: /ui/ ")
	add(&f.Config.UIDir, "ui-dir", "Path to directory containing the web UI resources.")
	add(&f.HCL, "hcl", "hcl config fragment. Can be specified multiple times.")
}

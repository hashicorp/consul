package config

import (
	"flag"
	"fmt"
	"time"
)

// AddFlags adds the command line flags to the FlagSet, and sets the appropriate field in
// LoadOpts.FlagValues as the value receiver.
func AddFlags(fs *flag.FlagSet, f *LoadOpts) {
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
	add(&f.FlagValues.AdvertiseAddrLAN, "advertise", "Sets the advertise address to use.")
	add(&f.FlagValues.AdvertiseAddrWAN, "advertise-wan", "Sets address to advertise on WAN instead of -advertise address.")
	add(&f.FlagValues.BindAddr, "bind", "Sets the bind address for cluster communication.")
	add(&f.FlagValues.Ports.Server, "server-port", "Sets the server port to listen on.")
	add(&f.FlagValues.Bootstrap, "bootstrap", "Sets server to bootstrap mode.")
	add(&f.FlagValues.BootstrapExpect, "bootstrap-expect", "Sets server to expect bootstrap mode.")
	add(&f.FlagValues.ClientAddr, "client", "Sets the address to bind for client access. This includes RPC, DNS, HTTP, HTTPS and gRPC (if configured).")
	add(&f.FlagValues.CheckOutputMaxSize, "check_output_max_size", "Sets the maximum output size for checks on this agent")
	add(&f.ConfigFiles, "config-dir", "Path to a directory to read configuration files from. This will read every file ending in '.json' as configuration in this directory in alphabetical order. Can be specified multiple times.")
	add(&f.ConfigFiles, "config-file", "Path to a file in JSON or HCL format with a matching file extension. Can be specified multiple times.")
	fs.StringVar(&f.ConfigFormat, "config-format", "", "Config files are in this format irrespective of their extension. Must be 'hcl' or 'json'")
	add(&f.FlagValues.DataDir, "data-dir", "Path to a data directory to store agent state.")
	add(&f.FlagValues.Datacenter, "datacenter", "Datacenter of the agent.")
	add(&f.FlagValues.DefaultQueryTime, "default-query-time", "the amount of time a blocking query will wait before Consul will force a response. This value can be overridden by the 'wait' query parameter.")
	add(&f.DevMode, "dev", "Starts the agent in development mode.")
	add(&f.FlagValues.DisableHostNodeID, "disable-host-node-id", "Setting this to true will prevent Consul from using information from the host to generate a node ID, and will cause Consul to generate a random node ID instead.")
	add(&f.FlagValues.DisableKeyringFile, "disable-keyring-file", "Disables the backing up of the keyring to a file.")
	add(&f.FlagValues.Ports.DNS, "dns-port", "DNS port to use.")
	add(&f.FlagValues.DNSDomain, "domain", "Domain to use for DNS interface.")
	add(&f.FlagValues.DNSAltDomain, "alt-domain", "Alternate domain to use for DNS interface.")
	add(&f.FlagValues.EnableScriptChecks, "enable-script-checks", "Enables health check scripts.")
	add(&f.FlagValues.EnableLocalScriptChecks, "enable-local-script-checks", "Enables health check scripts from configuration file.")
	add(&f.FlagValues.HTTPConfig.AllowWriteHTTPFrom, "allow-write-http-from", "Only allow write endpoint calls from given network. CIDR format, can be specified multiple times.")
	add(&f.FlagValues.EncryptKey, "encrypt", "Provides the gossip encryption key.")
	add(&f.FlagValues.Ports.GRPC, "grpc-port", "Sets the gRPC API port to listen on.")
	add(&f.FlagValues.Ports.GRPCTLS, "grpc-tls-port", "Sets the gRPC-TLS API port to listen on.")
	add(&f.FlagValues.Ports.HTTP, "http-port", "Sets the HTTP API port to listen on.")
	add(&f.FlagValues.Ports.HTTPS, "https-port", "Sets the HTTPS API port to listen on.")
	add(&f.FlagValues.StartJoinAddrsLAN, "join", "Address of an agent to join at start time. Can be specified multiple times.")
	add(&f.FlagValues.StartJoinAddrsWAN, "join-wan", "Address of an agent to join -wan at start time. Can be specified multiple times.")
	add(&f.FlagValues.LogLevel, "log-level", "Log level of the agent.")
	add(&f.FlagValues.LogJSON, "log-json", "Output logs in JSON format.")
	add(&f.FlagValues.LogFile, "log-file", "Path to the file the logs get written to")
	add(&f.FlagValues.LogRotateBytes, "log-rotate-bytes", "Maximum number of bytes that should be written to a log file")
	add(&f.FlagValues.LogRotateDuration, "log-rotate-duration", "Time after which log rotation needs to be performed")
	add(&f.FlagValues.LogRotateMaxFiles, "log-rotate-max-files", "Maximum number of log file archives to keep")
	add(&f.FlagValues.MaxQueryTime, "max-query-time", "the maximum amount of time a blocking query can wait before Consul will force a response. Consul applies jitter to the wait time. The jittered time will be capped to MaxQueryTime.")
	add(&f.FlagValues.NodeName, "node", "Name of this node. Must be unique in the cluster.")
	add(&f.FlagValues.NodeID, "node-id", "A unique ID for this node across space and time. Defaults to a randomly-generated ID that persists in the data-dir.")
	add(&f.FlagValues.NodeMeta, "node-meta", "An arbitrary metadata key/value pair for this node, of the format `key:value`. Can be specified multiple times.")
	add(&f.FlagValues.ReadReplica, "non-voting-server", "(Enterprise-only) DEPRECATED: -read-replica should be used instead")
	add(&f.FlagValues.ReadReplica, "read-replica", "(Enterprise-only) This flag is used to make the server not participate in the Raft quorum, and have it only receive the data replication stream. This can be used to add read scalability to a cluster in cases where a high volume of reads to servers are needed.")
	add(&f.FlagValues.PidFile, "pid-file", "Path to file to store agent PID.")
	add(&f.FlagValues.RPCProtocol, "protocol", "Sets the protocol version. Defaults to latest.")
	add(&f.FlagValues.RaftProtocol, "raft-protocol", "Sets the Raft protocol version. Defaults to latest.")
	add(&f.FlagValues.DNSRecursors, "recursor", "Address of an upstream DNS server. Can be specified multiple times.")
	add(&f.FlagValues.PrimaryGateways, "primary-gateway", "Address of a mesh gateway in the primary datacenter to use to bootstrap WAN federation at start time with retries enabled. Can be specified multiple times.")
	add(&f.FlagValues.RejoinAfterLeave, "rejoin", "Ignores a previous leave and attempts to rejoin the cluster.")
	add(&f.FlagValues.AutoReloadConfig, "auto-reload-config", "Watches config files for changes and auto reloads the files when modified.")
	add(&f.FlagValues.RetryJoinIntervalLAN, "retry-interval", "Time to wait between join attempts.")
	add(&f.FlagValues.RetryJoinIntervalWAN, "retry-interval-wan", "Time to wait between join -wan attempts.")
	add(&f.FlagValues.RetryJoinLAN, "retry-join", "Address of an agent to join at start time with retries enabled. Can be specified multiple times.")
	add(&f.FlagValues.RetryJoinWAN, "retry-join-wan", "Address of an agent to join -wan at start time with retries enabled. Can be specified multiple times.")
	add(&f.FlagValues.RetryJoinMaxAttemptsLAN, "retry-max", "Maximum number of join attempts. Defaults to 0, which will retry indefinitely.")
	add(&f.FlagValues.RetryJoinMaxAttemptsWAN, "retry-max-wan", "Maximum number of join -wan attempts. Defaults to 0, which will retry indefinitely.")
	add(&f.FlagValues.SerfAllowedCIDRsLAN, "serf-lan-allowed-cidrs", "Networks (eg: 192.168.1.0/24) allowed for Serf LAN. Can be specified multiple times.")
	add(&f.FlagValues.SerfAllowedCIDRsWAN, "serf-wan-allowed-cidrs", "Networks (eg: 192.168.1.0/24) allowed for Serf WAN (other datacenters). Can be specified multiple times.")
	add(&f.FlagValues.SerfBindAddrLAN, "serf-lan-bind", "Address to bind Serf LAN listeners to.")
	add(&f.FlagValues.Ports.SerfLAN, "serf-lan-port", "Sets the Serf LAN port to listen on.")
	add(&f.FlagValues.SegmentName, "segment", "(Enterprise-only) Sets the network segment to join.")
	add(&f.FlagValues.SerfBindAddrWAN, "serf-wan-bind", "Address to bind Serf WAN listeners to.")
	add(&f.FlagValues.Ports.SerfWAN, "serf-wan-port", "Sets the Serf WAN port to listen on.")
	add(&f.FlagValues.ServerMode, "server", "Switches agent to server mode.")
	add(&f.FlagValues.EnableSyslog, "syslog", "Enables logging to syslog.")
	add(&f.FlagValues.UIConfig.Enabled, "ui", "Enables the built-in static web UI server.")
	add(&f.FlagValues.UIConfig.ContentPath, "ui-content-path", "Sets the external UI path to a string. Defaults to: /ui/ ")
	add(&f.FlagValues.UIConfig.Dir, "ui-dir", "Path to directory containing the web UI resources.")
	add(&f.HCL, "hcl", "hcl config fragment. Can be specified multiple times.")
}

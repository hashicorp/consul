---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
---

# Configuration

The agent has various configuration options that can be specified via
the command-line or via configuration files. All of the configuration
options are completely optional and their defaults will be specified
with their descriptions.

When loading configuration, Consul loads the configuration from files
and directories in the order specified. Configuration specified later
will be merged into configuration specified earlier. In most cases,
"merge" means that the later version will override the earlier. But in
some cases, such as event handlers, merging just appends the handlers.
The exact merging behavior will be specified.

Consul also supports reloading of configuration when it receives the
SIGHUP signal. Not all changes are respected, but those that are
are documented below.

## Command-line Options

The options below are all specified on the command-line.

* `-serf-bind` - The address that the underlying Serf library will bind to.
  This is an IP address that should be reachable by all other nodes in the cluster.
  By default this is "0.0.0.0", meaning Consul will use the first available private
  IP address. Consul uses both TCP and UDP and use the same port for both, so if you
  have any firewalls be sure to allow both protocols.

* `-server-addr` - The address that the agent will bind to for handling RPC calls
 if running in server mode. This does not affect clients running in client mode.
 By default this is "0.0.0.0:8300". This port is used for TCP communications so any
 firewalls must be configured to allow this.

* `-advertise` - The advertise flag is used to change the address that we
  advertise to other nodes in the cluster. By default, the `-serf-bind` address is
  advertised. However, in some cases (specifically NAT traversal), there may
  be a routable address that cannot be bound to. This flag enables gossiping
  a different address to support this. If this address is not routable, the node
  will be in a constant flapping state, as other nodes will treat the non-routability
  as a failure.

* `-config-file` - A configuration file to load. For more information on
  the format of this file, read the "Configuration Files" section below.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier, with the later values
  overriding the earlier values.

* `-config-dir` - A directory of configuration files to load. Consul will
  load all files in this directory ending in ".json" as configuration files
  in alphabetical order. For more information on the format of the configuration
  files, see the "Configuration Files" section below.

* `-encrypt` - Specifies the secret key to use for encryption of Consul
  network traffic. This key must be 16-bytes that are base64 encoded. The
  easiest way to create an encryption key is to use `consul keygen`. All
  nodes within a cluster must share the same encryption key to communicate.

* `-log-level` - The level of logging to show after the Consul agent has
  started. This defaults to "info". The available log levels are "trace",
  "debug", "info", "warn", "err". This is the log level that will be shown
  for the agent output, but note you can always connect via `consul monitor`
  to an agent at any log level. The log level can be changed during a
  config reload.

* `-node` - The name of this node in the cluster. This must be unique within
  the cluster. By default this is the hostname of the machine.

* `-rpc-addr` - The address that Consul will bind to for the agent's  RPC server.
  By default this is "127.0.0.1:8400", allowing only loopback connections.
  The RPC address is used by other Consul commands, such as  `consul members`,
  in order to query a running Consul agent. It is also used by other applications
  to control Consul using it's [RPC protocol](/docs/agent/rpc.html).

* `-data-dir` - This flag provides a data directory for the agent to store state.
  This is required for all agents. The directory should be durable across reboots.
  This is especially critical for agents that are running in server mode, as they
  must be able to persist the cluster state.

* `-dc` - This flag controls the datacenter the agent is running in. If not provided
  it defaults to "dc1". Consul has first class support for multiple data centers but
  it relies on proper configuration. Nodes in the same datacenter should be on a single
  LAN.

* `-recursor` - This flag provides an address of an upstream DNS server that is used to
  recursively resolve queries if they are not inside the service domain for consul. For example,
  a node can use Consul directly as a DNS server, and if the record is outside of the "consul." domain,
  the query will be resolved upstream using this server.

* `-http-addr` - This flag controls the address the agent listens on for HTTP requests.
  By default it is bound to "127.0.0.1:8500". This port must allow for TCP traffic.

* `-dns-addr` - This flag controls the address the agent listens on for DNS requests.
  By default it is bound to "127.0.0.1:8600". This port must allow for UDP and TCP traffic.

* `-server` - This flag is used to control if an agent is in server or client mode. When provided,
  an agent will act as a Consul server. Each Consul cluster must have at least one server, and ideally
  no more than 5 *per* datacenter. All servers participate in the Raft consensus algorithm, to ensure that
  transactions occur in a consistent, linearlizable manner. Transactions modify cluster state, which
  is maintained on all server nodes to ensure availability in the case of node failure. Server nodes also
  participate in a WAN gossip pool with server nodes in other datacenters. Servers act as gateways
  to other datacenters and forward traffic as appropriate.

* `-bootstrap` - This flag is used to control if a server is in "bootstrap" mode. It is important that
  no more than one server *per* datacenter be running in this mode. The initial server **must** be in bootstrap
  mode. Technically, a server in boostrap mode is allowed to self-elect as the Raft leader. It is important
  that only a single node is in this mode, because otherwise consistency cannot be guarenteed if multiple
  nodes are able to self-elect. Once there are multiple servers in a datacenter, it is generally a good idea
  to disable bootstrap mode on all of them.

* `-statsite` - This flag provides the address of a statsite instance. If provided Consul will stream
  various telemetry information to that instance for aggregation. This can be used to capture various
  runtime information.

* `-protocol` - The Consul protocol version to use. This defaults to the latest
  version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Consul by running `serf -v`.

## Configuration Files

In addition to the command-line options, configuration can be put into
files. This may be easier in certain situations, for example when Consul is
being configured using a configuration management system.

The configuration files are JSON formatted, making them easily readable
and editable by both humans and computers. The configuration is formatted
at a single JSON object with configuration within it.

Configuration files are used for more than just setting up the agent,
they are also used to provide check and service definitions. These are used
to announce the availability of system servers to the rest of the cluster.
They are documented seperately under [check configuration](/docs/agent/checks.html) and
[service configuration](/docs/agent/services.html) respectively. The service and check
definitions support being updated during a reload.

#### Example Configuration File

<pre class="prettyprint lang-json">
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true
}
</pre>

#### Configuration Key Reference

* `bootstrap` - Equivalent to the `-bootstrap` command-line flag.

* `datacenter` - Equivalent to the `-dc` command-line flag.

* `data_dir` - Equivalent to the `-data-dir` command-line flag.

* `dns_addr` - Equivalent to the `-dns-addr` command-line flag.

* `recursor`  - Equivalent to the `-recursor` command-line flag.

* `domain` - By default, Consul responds to DNS queries in the "consul." domain.
  This flag can be used to change that domain. All queries in this domain are assumed
  to be handled by Consul, and will not be recursively resolved.

* `encrypt` - Equivalent to the `-encrypt` command-line flag.

* `http_addr` - Equivalent to the `-http-addr` command-line flag.

* `log_level` - Equivalent to the `-log-level` command-line flag.

* `node_name` - Equivalent to the `-node` command-line flag.

* `rpc_addr` - Equivalent to the `-rpc-addr` command-line flag.

* `serf_bind_addr` - Equivalent to the `-serf-bind` command-line flag.

* `serf_lan_port` - This configures which port Serf listens on to communicate
  with nodes on the local LAN. By default this is 8301. All nodes in the datacenter
  should be able to reach this port over TCP and UDP.

* `serf_wan_port` - This configures which port Serf listens on to communicate
  with nodes on the remote WAN. By default this is 8302. All nodes in the WAN gossip
  pool should be able to reach this port over TCP and UDP. This only applies to
  agents running in server mode.

* `server_addr` - Equivalent to the `-server-addr` command-line flag.

* `advertise_addr` - Equivalent to the `-advertise` command-line flag.

* `server` - Equivalent to the `-server` command-line flag.

* `leave_on_terminate` - If enabled, when the agent receives a TERM signal,
  it will send a Leave message to the rest of the cluster and gracefully
  leave. Defaults to false.

* `skip_leave_on_interrupt` - This is the similar to`leave_on_terminate` but
  only affects interrupt handling. By default, an interrupt causes Consul to
  gracefully leave, but setting this to true disables that. Defaults to false.
  Interrupts are usually from a Control-C from a shell.

* `statsite_addr` - Equivalent to the `-statsite` command-line flag.


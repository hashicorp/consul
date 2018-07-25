---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
description: |-
  The agent has various configuration options that can be specified via the command-line or via configuration files. All of the configuration options are completely optional. Defaults are specified with their descriptions.
---

# Configuration

The agent has various configuration options that can be specified via
the command-line or via configuration files. All of the configuration
options are completely optional. Defaults are specified with their
descriptions.

Configuration precedence is evaluated in the following order:

1.  Command line arguments
2.  Environment Variables
3.  Configuration files

When loading configuration, Consul loads the configuration from files and
directories in lexical order. For example, configuration file
`basic_config.json` will be processed before `extra_config.json`. Configuration
can be in either [HCL](https://github.com/hashicorp/hcl#syntax) or JSON format.
Available in Consul 1.0 and later, the HCL support now requires an `.hcl` or
`.json` extension on all configuration files in order to specify their format.

Configuration specified later will be merged into configuration specified
earlier. In most cases, "merge" means that the later version will override the
earlier. In some cases, such as event handlers, merging appends the handlers to
the existing configuration. The exact merging behavior is specified for each
option below.

Consul also supports reloading configuration when it receives the
SIGHUP signal. Not all changes are respected, but those that are
are documented below in the
[Reloadable Configuration](#reloadable-configuration) section. The
[reload command](/docs/commands/reload.html) can also be used to trigger a
configuration reload.

## <a name="commandline_options"></a>Command-line Options

The options below are all specified on the command-line.

* <a name="_advertise"></a><a href="#_advertise">`-advertise`</a> - The
  advertise address is used to change the address that we advertise to other
  nodes in the cluster. By default, the [`-bind`](#_bind) address is advertised.
  However, in some cases, there may be a routable address that cannot be bound.
  This flag enables gossiping a different address to support this. If this
  address is not routable, the node will be in a constant flapping state as
  other nodes will treat the non-routability as a failure. In Consul 1.0 and
  later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template.

* <a name="_advertise-wan"></a><a href="#_advertise-wan">`-advertise-wan`</a> - The
  advertise WAN address is used to change the address that we advertise to server nodes
  joining through the WAN. This can also be set on client agents when used in combination
  with the <a href="#translate_wan_addrs">`translate_wan_addrs`</a> configuration
  option. By default, the [`-advertise`](#_advertise) address is advertised. However, in some
  cases all members of all datacenters cannot be on the same physical or virtual network,
  especially on hybrid setups mixing cloud and private datacenters. This flag enables server
  nodes gossiping through the public network for the WAN while using private VLANs for gossiping
  to each other and their client agents, and it allows client agents to be reached at this
  address when being accessed from a remote datacenter if the remote datacenter is configured
  with <a href="#translate_wan_addrs">`translate_wan_addrs`</a>. In Consul 1.0 and
  later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

* <a name="_bootstrap"></a><a href="#_bootstrap">`-bootstrap`</a> - This flag is used to control if a
  server is in "bootstrap" mode. It is important that
  no more than one server _per_ datacenter be running in this mode. Technically, a server in bootstrap mode
  is allowed to self-elect as the Raft leader. It is important that only a single node is in this mode;
  otherwise, consistency cannot be guaranteed as multiple nodes are able to self-elect.
  It is not recommended to use this flag after a cluster has been bootstrapped.

* <a name="_bootstrap_expect"></a><a href="#_bootstrap_expect">`-bootstrap-expect`</a> - This flag
  provides the number of expected servers in the datacenter.
  Either this value should not be provided or the value must agree with other servers in
  the cluster. When provided, Consul waits until the specified number of servers are
  available and then bootstraps the cluster. This allows an initial leader to be elected
  automatically. This cannot be used in conjunction with the legacy [`-bootstrap`](#_bootstrap) flag.
  This flag requires [`-server`](#_server) mode.

* <a name="_bind"></a><a href="#_bind">`-bind`</a> - The address that should be bound to
  for internal cluster communications.
  This is an IP address that should be reachable by all other nodes in the cluster.
  By default, this is "0.0.0.0", meaning Consul will bind to all addresses on
  the local machine and will [advertise](/docs/agent/options.html#_advertise)
  the first available private IPv4 address to the rest of the cluster. If there
  are multiple private IPv4 addresses available, Consul will exit with an error
  at startup. If you specify "[::]", Consul will
  [advertise](/docs/agent/options.html#_advertise) the first available public
  IPv6 address. If there are multiple public IPv6 addresses available, Consul
  will exit with an error at startup.
  Consul uses both TCP and UDP and the same port for both. If you
  have any firewalls, be sure to allow both protocols. In Consul 1.0 and later
  this can be set to a space-separated list of addresses to bind to, or a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) template
  that can potentially resolve to multiple addresses.

* <a name="_serf_wan_bind"></a><a href="#_serf_wan_bind">`-serf-wan-bind`</a> -
  The address that should be bound to for Serf WAN gossip communications. By
  default, the value follows the same rules as [`-bind` command-line
  flag](#_bind), and if this is not specified, the `-bind` option is used. This
  is available in Consul 0.7.1 and later. In Consul 1.0 and later this can be
  set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

* <a name="_serf_lan_bind"></a><a href="#_serf_lan_bind">`-serf-lan-bind`</a> -
  The address that should be bound to for Serf LAN gossip communications. This
  is an IP address that should be reachable by all other LAN nodes in the
  cluster. By default, the value follows the same rules as [`-bind` command-line
  flag](#_bind), and if this is not specified, the `-bind` option is used. This
  is available in Consul 0.7.1 and later. In Consul 1.0 and later this can be
  set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

* <a name="_client"></a><a href="#_client">`-client`</a> - The address to which
  Consul will bind client interfaces, including the HTTP and DNS servers. By
  default, this is "127.0.0.1", allowing only loopback connections. In Consul
  1.0 and later this can be set to a space-separated list of addresses to bind
  to, or a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template that can potentially resolve to multiple addresses.

* <a name="_config_file"></a><a href="#_config_file">`-config-file`</a> - A configuration file
  to load. For more information on
  the format of this file, read the [Configuration Files](#configuration_files) section.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier. During a config merge,
  single-value keys (string, int, bool) will simply have their values replaced
  while list types will be appended together.

* <a name="_config_dir"></a><a href="#_config_dir">`-config-dir`</a> - A directory of
  configuration files to load. Consul will
  load all files in this directory with the suffix ".json". The load order
  is alphabetical, and the the same merge routine is used as with the
  [`config-file`](#_config_file) option above. This option can be specified multiple times
  to load multiple directories. Sub-directories of the config directory are not loaded.
  For more information on the format of the configuration files, see the
  [Configuration Files](#configuration_files) section.

* <a name="_config_format"></a><a href="#_config_format">`-config-format`</a> - The format
  of the configuration files to load. Normally, Consul detects the format of the
  config files from the ".json" or ".hcl" extension. Setting this option to
  either "json" or "hcl" forces Consul to interpret any file with or without
  extension to be interpreted in that format.

* <a name="_data_dir"></a><a href="#_data_dir">`-data-dir`</a> - This flag
  provides a data directory for the agent to store state. This is required for
  all agents. The directory should be durable across reboots. This is especially
  critical for agents that are running in server mode as they must be able to
  persist cluster state. Additionally, the directory must support the use of
  filesystem locking, meaning some types of mounted folders (e.g. VirtualBox
  shared folders) may not be suitable. **Note:** both server and non-server
  agents may store ACL tokens in the state in this directory so read access may
  grant access to any tokens on servers and to any tokens used during service
  registration on non-servers. On Unix-based platforms the files are written
  with 0600 permissions so you should ensure only trusted processes can execute
  as the same user as Consul. On Windows, you should ensure the directory has
  suitable permissions configured as these will be inherited.

* <a name="_datacenter"></a><a href="#_datacenter">`-datacenter`</a> - This flag controls the datacenter in
  which the agent is running. If not provided,
  it defaults to "dc1". Consul has first-class support for multiple datacenters, but
  it relies on proper configuration. Nodes in the same datacenter should be on a single
  LAN.

* <a name="_dev"></a><a href="#_dev">`-dev`</a> - Enable development server
  mode. This is useful for quickly starting a Consul agent with all persistence
  options turned off, enabling an in-memory server which can be used for rapid
  prototyping or developing against the API. This mode is **not** intended for
  production use as it does not write any data to disk.

* <a name="_disable_host_node_id"></a><a href="#_disable_host_node_id">`-disable-host-node-id`</a> - Setting
  this to true will prevent Consul from using information from the host to generate a deterministic node ID,
  and will instead generate a random node ID which will be persisted in the data directory. This is useful
  when running multiple Consul agents on the same host for testing. This defaults to false in Consul prior
  to version 0.8.5 and in 0.8.5 and later defaults to true, so you must opt-in for host-based IDs. Host-based
  IDs are generated using https://github.com/shirou/gopsutil/tree/master/host, which is shared with HashiCorp's
  [Nomad](https://www.nomadproject.io/), so if you opt-in to host-based IDs then Consul and Nomad will use
  information on the host to automatically assign the same ID in both systems.

* <a name="_disable_keyring_file"></a><a href="#_disable_keyring_file">`-disable-keyring-file`</a> - If set,
  the keyring will not be persisted to a file. Any installed keys will be lost on shutdown, and only the given
  `-encrypt` key will be available on startup. This defaults to false.

* <a name="_dns_port"></a><a href="#_dns_port">`-dns-port`</a> - the DNS port to listen on.
  This overrides the default port 8600. This is available in Consul 0.7 and later.

* <a name="_domain"></a><a href="#_domain">`-domain`</a> - By default, Consul responds to DNS queries
  in the "consul." domain. This flag can be used to change that domain. All queries in this domain
  are assumed to be handled by Consul and will not be recursively resolved.

* <a name="_enable_script_checks"></a><a href="#_enable_script_checks">`-enable-script-checks`</a> This
  controls whether [health checks that execute scripts](/docs/agent/checks.html) are enabled on
  this agent, and defaults to `false` so operators must opt-in to allowing these. If enabled,
  it is recommended to [enable ACLs](/docs/guides/acl.html) as well to control which users are
  allowed to register new checks to execute scripts. This was added in Consul 0.9.0.

* <a name="_encrypt"></a><a href="#_encrypt">`-encrypt`</a> - Specifies the secret key to
  use for encryption of Consul
  network traffic. This key must be 16-bytes that are Base64-encoded. The
  easiest way to create an encryption key is to use
  [`consul keygen`](/docs/commands/keygen.html). All
  nodes within a cluster must share the same encryption key to communicate.
  The provided key is automatically persisted to the data directory and loaded
  automatically whenever the agent is restarted. This means that to encrypt
  Consul's gossip protocol, this option only needs to be provided once on each
  agent's initial startup sequence. If it is provided after Consul has been
  initialized with an encryption key, then the provided key is ignored and
  a warning will be displayed.

* <a name="_hcl"></a><a href="#_hcl">`-hcl`</a> - A HCL configuration fragment.
  This HCL configuration fragment is appended to the configuration and allows
  to specify the full range of options of a config file on the command line.
  This option can be specified multiple times. This was added in Consul 1.0.

* <a name="_http_port"></a><a href="#_http_port">`-http-port`</a> - the HTTP API port to listen on.
  This overrides the default port 8500. This option is very useful when deploying Consul
  to an environment which communicates the HTTP port through the environment e.g. PaaS like CloudFoundry, allowing
  you to set the port directly via a Procfile.

* <a name="_join"></a><a href="#_join">`-join`</a> - Address of another agent
  to join upon starting up. This can be
  specified multiple times to specify multiple agents to join. If Consul is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't join any nodes when it starts up.
  Note that using
  <a href="#retry_join">`retry_join`</a> could be more appropriate to help
  mitigate node startup race conditions when automating a Consul cluster
  deployment.

  In Consul 1.1.0 and later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

<a name="_retry_join"></a>

* `-retry-join` - Similar to [`-join`](#_join) but allows retrying a join if the
  first attempt fails. This is useful for cases where you know the address will
  eventually be available. The list can contain IPv4, IPv6, or DNS addresses. In
  Consul 1.1.0 and later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template. If Consul is running on the non-default Serf LAN port, this must be
  specified as well. IPv6 must use the "bracketed" syntax. If multiple values
  are given, they are tried and retried in the order listed until the first
  succeeds. Here are some examples:

  ```sh
  # Using a DNS entry
  $ consul agent -retry-join "consul.domain.internal"
  ```

  ```sh
  # Using IPv4
  $ consul agent -retry-join "10.0.4.67"
  ```

  ```sh
  # Using IPv6
  $ consul agent -retry-join "[::1]:8301"
  ```

  ### Cloud Auto-Joining

  As of Consul 0.9.1, `retry-join` accepts a unified interface using the
  [go-discover](https://github.com/hashicorp/go-discover) library for doing
  automatic cluster joining using cloud metadata. For more information, see
  the [Cloud Auto-join page](/docs/agent/cloud-auto-join.html).

  ```sh
  # Using Cloud Auto-Joining
  $ consul agent -retry-join "provider=aws tag_key=..."
  ```

* <a name="_retry_interval"></a><a href="#_retry_interval">`-retry-interval`</a> - Time
  to wait between join attempts. Defaults to 30s.

* <a name="_retry_max"></a><a href="#_retry_max">`-retry-max`</a> - The maximum number
  of [`-join`](#_join) attempts to be made before exiting
  with return code 1. By default, this is set to 0 which is interpreted as infinite
  retries.

* <a name="_join_wan"></a><a href="#_join_wan">`-join-wan`</a> - Address of
  another wan agent to join upon starting up. This can be specified multiple
  times to specify multiple WAN agents to join. If Consul is unable to join with
  any of the specified addresses, agent startup will fail. By default, the agent
  won't [`-join-wan`](#_join_wan) any nodes when it starts up.

  In Consul 1.1.0 and later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template.

* <a name="_retry_join_wan"></a><a href="#_retry_join_wan">`-retry-join-wan`</a> - Similar
  to [`retry-join`](#_retry_join) but allows retrying a wan join if the first attempt fails.
  This is useful for cases where we know the address will become available eventually.
  As of Consul 0.9.3 [Cloud Auto-Joining](#cloud-auto-joining) is supported as well.

  In Consul 1.1.0 and later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

* <a name="_retry_interval_wan"></a><a href="#_retry_interval_wan">`-retry-interval-wan`</a> - Time
  to wait between [`-join-wan`](#_join_wan) attempts.
  Defaults to 30s.

* <a name="_retry_max_wan"></a><a href="#_retry_max_wan">`-retry-max-wan`</a> - The maximum
  number of [`-join-wan`](#_join_wan) attempts to be made before exiting with return code 1.
  By default, this is set to 0 which is interpreted as infinite retries.

* <a name="_log_level"></a><a href="#_log_level">`-log-level`</a> - The level of logging to
  show after the Consul agent has started. This defaults to "info". The available log levels are
  "trace", "debug", "info", "warn", and "err". You can always connect to an
  agent via [`consul monitor`](/docs/commands/monitor.html) and use any log level. Also, the
  log level can be changed during a config reload.

* <a name="_node"></a><a href="#_node">`-node`</a> - The name of this node in the cluster.
  This must be unique within the cluster. By default this is the hostname of the machine.

* <a name="_node_id"></a><a href="#_node_id">`-node-id`</a> - Available in Consul 0.7.3 and later, this
  is a unique identifier for this node across all time, even if the name of the node or address
  changes. This must be in the form of a hex string, 36 characters long, such as
  `adf4238a-882b-9ddc-4a9d-5b6758e4159e`. If this isn't supplied, which is the most common case, then
  the agent will generate an identifier at startup and persist it in the <a href="#_data_dir">data directory</a>
  so that it will remain the same across agent restarts. Information from the host will be used to
  generate a deterministic node ID if possible, unless [`-disable-host-node-id`](#_disable_host_node_id) is
  set to true.

* <a name="_node_meta"></a><a href="#_node_meta">`-node-meta`</a> - Available in Consul 0.7.3 and later,
  this specifies an arbitrary metadata key/value pair to associate with the node, of the form `key:value`.
  This can be specified multiple times. Node metadata pairs have the following restrictions:

  * A maximum of 64 key/value pairs can be registered per node.
  * Metadata keys must be between 1 and 128 characters (inclusive) in length
  * Metadata keys must contain only alphanumeric, `-`, and `_` characters.
  * Metadata keys must not begin with the `consul-` prefix; that is reserved for internal use by Consul.
  * Metadata values must be between 0 and 512 (inclusive) characters in length.
  * Metadata values for keys beginning with `rfc1035-` are encoded verbatim in DNS TXT requests, otherwise
    the metadata kv-pair is encoded according [RFC1464](https://www.ietf.org/rfc/rfc1464.txt).

* <a name="_pid_file"></a><a href="#_pid_file">`-pid-file`</a> - This flag provides the file
  path for the agent to store its PID. This is useful for sending signals (for example, `SIGINT`
  to close the agent or `SIGHUP` to update check definite

* <a name="_protocol"></a><a href="#_protocol">`-protocol`</a> - The Consul protocol version to
  use. This defaults to the latest version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Consul by running `consul -v`.

* <a name="_raft_protocol"></a><a href="#_raft_protocol">`-raft-protocol`</a> - This controls the internal
  version of the Raft consensus protocol used for server communications. This must be set to 3 in order to
  gain access to Autopilot features, with the exception of [`cleanup_dead_servers`](#cleanup_dead_servers).
  Defaults to 3 in Consul 1.0.0 and later (defaulted to 2 previously). See
  [Raft Protocol Version Compatibility](/docs/upgrade-specific.html#raft-protocol-version-compatibility)
  for more details.

* <a name="_raft_snapshot_threshold"></a><a href="#_raft_snapshot_threshold">`-raft-snapshot-threshold`</a> - This controls the
  minimum number of raft commit entries between snapshots that are saved to disk. This is a low-level parameter that should
  rarely need to be changed. Very busy clusters experiencing excessive disk IO may increase this value to reduce disk IO, and minimize
  the chances of all servers taking snapshots at the same time. Increasing this trades off disk IO for disk space since the log will
  grow much larger and the space in the raft.db file can't be reclaimed till the next snapshot. Servers may take longer to recover from
  crashes or failover if this is increased significantly as more logs will need to be replayed. In Consul 1.1.0 and later this
  defaults to 16384, and in prior versions it was set to 8192.

* <a name="_raft_snapshot_interval"></a><a href="#_raft_snapshot_interval">`-raft-snapshot-interval`</a> - This controls how often servers
  check if they need to save a snapshot to disk. his is a low-level parameter that should rarely need to be changed. Very busy clusters
  experiencing excessive disk IO may increase this value to reduce disk IO, and minimize the chances of all servers taking snapshots at the same time.
  Increasing this trades off disk IO for disk space since the log will grow much larger and the space in the raft.db file can't be reclaimed
  till the next snapshot. Servers may take longer to recover from crashes or failover if this is increased significantly as more logs
  will need to be replayed. In Consul 1.1.0 and later this defaults to `30s`, and in prior versions it was set to `5s`.

* <a name="_recursor"></a><a href="#_recursor">`-recursor`</a> - Specifies the address of an upstream DNS
  server. This option may be provided multiple times, and is functionally
  equivalent to the [`recursors` configuration option](#recursors).

* <a name="_rejoin"></a><a href="#_rejoin">`-rejoin`</a> - When provided, Consul will ignore a
  previous leave and attempt to rejoin the cluster when starting. By default, Consul treats leave
  as a permanent intent and does not attempt to join the cluster again when starting. This flag
  allows the previous state to be used to rejoin the cluster.

* <a name="_segment"></a><a href="#_segment">`-segment`</a> - (Enterprise-only) This flag is used to set
  the name of the network segment the agent belongs to. An agent can only join and communicate with other agents
  within its network segment. See the [Network Segments Guide](/docs/guides/segments.html) for more details.
  By default, this is an empty string, which is the default network segment.

* <a name="_server"></a><a href="#_server">`-server`</a> - This flag is used to control if an
  agent is in server or client mode. When provided,
  an agent will act as a Consul server. Each Consul cluster must have at least one server and ideally
  no more than 5 per datacenter. All servers participate in the Raft consensus algorithm to ensure that
  transactions occur in a consistent, linearizable manner. Transactions modify cluster state, which
  is maintained on all server nodes to ensure availability in the case of node failure. Server nodes also
  participate in a WAN gossip pool with server nodes in other datacenters. Servers act as gateways
  to other datacenters and forward traffic as appropriate.

* <a name="_non_voting_server"></a><a href="#_non_voting_server">`-non-voting-server`</a> - (Enterprise-only)
  This flag is used to make the server not participate in the Raft quorum, and have it only receive the data
  replication stream. This can be used to add read scalability to a cluster in cases where a high volume of
  reads to servers are needed.

* <a name="_syslog"></a><a href="#_syslog">`-syslog`</a> - This flag enables logging to syslog. This
  is only supported on Linux and OSX. It will result in an error if provided on Windows.

* <a name="_ui"></a><a href="#_ui">`-ui`</a> - Enables the built-in web UI
  server and the required HTTP routes. This eliminates the need to maintain the
  Consul web UI files separately from the binary.

* <a name="_ui_dir"></a><a href="#_ui_dir">`-ui-dir`</a> - This flag provides the directory containing
  the Web UI resources for Consul. This will automatically enable the Web UI. The directory must be
  readable to the agent. Starting with Consul version 0.7.0 and later, the Web UI assets are included in the binary so this flag is no longer necessary; specifying only the `-ui` flag is enough to enable the Web UI. Specifying both the '-ui' and '-ui-dir' flags will result in an error.

## <a name="configuration_files"></a>Configuration Files

In addition to the command-line options, configuration can be put into
files. This may be easier in certain situations, for example when Consul is
being configured using a configuration management system.

The configuration files are JSON formatted, making them easily readable
and editable by both humans and computers. The configuration is formatted
as a single JSON object with configuration within it.

Configuration files are used for more than just setting up the agent,
they are also used to provide check and service definitions. These are used
to announce the availability of system servers to the rest of the cluster.
They are documented separately under [check configuration](/docs/agent/checks.html) and
[service configuration](/docs/agent/services.html) respectively. The service and check
definitions support being updated during a reload.

#### Example Configuration File

```javascript
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true,
  "watches": [
    {
        "type": "checks",
        "handler": "/usr/bin/health-check-handler.sh"
    }
  ],
  "telemetry": {
     "statsite_address": "127.0.0.1:2180"
  }
}
```

#### Example Configuration File, with TLS

```javascript
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true,
  "addresses": {
    "https": "0.0.0.0"
  },
  "ports": {
    "https": 8080
  },
  "key_file": "/etc/pki/tls/private/my.key",
  "cert_file": "/etc/pki/tls/certs/my.crt",
  "ca_file": "/etc/pki/tls/certs/ca-bundle.crt"
}
```

See, especially, the use of the `ports` setting:

```javascript
"ports": {
  "https": 8080
}
```

Consul will not enable TLS for the HTTP API unless the `https` port has been assigned a port number `> 0`.

#### Configuration Key Reference

* <a name="acl_datacenter"></a><a href="#acl_datacenter">`acl_datacenter`</a> - This designates
  the datacenter which is authoritative for ACL information. It must be provided to enable ACLs.
  All servers and datacenters must agree on the ACL datacenter. Setting it on the servers is all
  you need for cluster-level enforcement, but for the APIs to forward properly from the clients,
  it must be set on them too. In Consul 0.8 and later, this also enables agent-level enforcement
  of ACLs. Please see the [ACL Guide](/docs/guides/acl.html) for more details.

* <a name="acl_default_policy"></a><a href="#acl_default_policy">`acl_default_policy`</a> - Either
  "allow" or "deny"; defaults to "allow". The default policy controls the behavior of a token when
  there is no matching rule. In "allow" mode, ACLs are a blacklist: any operation not specifically
  prohibited is allowed. In "deny" mode, ACLs are a whitelist: any operation not
  specifically allowed is blocked. _Note_: this will not take effect until you've set `acl_datacenter`
  to enable ACL support.

* <a name="acl_down_policy"></a><a href="#acl_down_policy">`acl_down_policy`</a> - Either
  "allow", "deny" or "extend-cache"; "extend-cache" is the default. In the case that the
  policy for a token cannot be read from the [`acl_datacenter`](#acl_datacenter) or leader
  node, the down policy is applied. In "allow" mode, all actions are permitted, "deny" restricts
  all operations, and "extend-cache" allows any cached ACLs to be used, ignoring their TTL
  values. If a non-cached ACL is used, "extend-cache" acts like "deny".

* <a name="acl_agent_master_token"></a><a href="#acl_agent_master_token">`acl_agent_master_token`</a> -
  Used to access <a href="/api/agent.html">agent endpoints</a> that require agent read
  or write privileges, or node read privileges, even if Consul servers aren't present to validate
  any tokens. This should only be used by operators during outages, regular ACL tokens should normally
  be used by applications. This was added in Consul 0.7.2 and is only used when
  <a href="#acl_enforce_version_8">`acl_enforce_version_8`</a> is set to true. Please see
  [ACL Agent Master Token](/docs/guides/acl.html#acl-agent-master-token) for more details.

* <a name="acl_agent_token"></a><a href="#acl_agent_token">`acl_agent_token`</a> - Used for clients
  and servers to perform internal operations. If this isn't specified, then the
  <a href="#acl_token">`acl_token`</a> will be used. This was added in Consul 0.7.2.

  This token must at least have write access to the node name it will register as in order to set any
  of the node-level information in the catalog such as metadata, or the node's tagged addresses. There
  are other places this token is used, please see [ACL Agent Token](/docs/guides/acl.html#acl-agent-token)
  for more details.

* <a name="acl_enforce_version_8"></a><a href="#acl_enforce_version_8">`acl_enforce_version_8`</a> -
  Used for clients and servers to determine if enforcement should occur for new ACL policies being
  previewed before Consul 0.8. Added in Consul 0.7.2, this defaults to false in versions of
  Consul prior to 0.8, and defaults to true in Consul 0.8 and later. This helps ease the
  transition to the new ACL features by allowing policies to be in place before enforcement begins.
  Please see the [ACL Guide](/docs/guides/acl.html#version_8_acls) for more details.

* <a name="acl_master_token"></a><a href="#acl_master_token">`acl_master_token`</a> - Only used
  for servers in the [`acl_datacenter`](#acl_datacenter). This token will be created with management-level
  permissions if it does not exist. It allows operators to bootstrap the ACL system
  with a token ID that is well-known.

  The `acl_master_token` is only installed when a server acquires cluster leadership. If
  you would like to install or change the `acl_master_token`, set the new value for `acl_master_token`
  in the configuration for all servers. Once this is done, restart the current leader to force a
  leader election. If the `acl_master_token` is not supplied, then the servers do not create a master
  token. When you provide a value, it can be any string value. Using a UUID would ensure that it looks
  the same as the other tokens, but isn't strictly necessary.

* <a name="acl_replication_token"></a><a href="#acl_replication_token">`acl_replication_token`</a> -
  Only used for servers outside the [`acl_datacenter`](#acl_datacenter) running Consul 0.7 or later.
  When provided, this will enable [ACL replication](/docs/guides/acl.html#replication) using this
  token to retrieve and replicate the ACLs to the non-authoritative local datacenter. In Consul 0.9.1
  and later you can enable ACL replication using [`enable_acl_replication`](#enable_acl_replication)
  and then set the token later using the [agent token API](/api/agent.html#update-acl-tokens) on each
  server. If the `acl_replication_token` is set in the config, it will automatically set
  [`enable_acl_replication`](#enable_acl_replication) to true for backward compatibility.

  If there's a partition or other outage affecting the authoritative datacenter, and the
  [`acl_down_policy`](/docs/agent/options.html#acl_down_policy) is set to "extend-cache", tokens not
  in the cache can be resolved during the outage using the replicated set of ACLs. Please see the
  [ACL Guide](/docs/guides/acl.html#replication) replication section for more details.

* <a name="acl_token"></a><a href="#acl_token">`acl_token`</a> - When provided, the agent will use this
  token when making requests to the Consul servers. Clients can override this token on a per-request
  basis by providing the "?token" query parameter. When not provided, the empty token, which maps to
  the 'anonymous' ACL policy, is used.

* <a name="acl_ttl"></a><a href="#acl_ttl">`acl_ttl`</a> - Used to control Time-To-Live caching of ACLs.
  By default, this is 30 seconds. This setting has a major performance impact: reducing it will cause
  more frequent refreshes while increasing it reduces the number of refreshes. However, because the caches
  are not actively invalidated, ACL policy may be stale up to the TTL value.

* <a name="addresses"></a><a href="#addresses">`addresses`</a> - This is a nested object that allows
  setting bind addresses. In Consul 1.0 and later these can be set to a space-separated list of
  addresses to bind to, or a [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template that can potentially resolve to multiple addresses.

  `http` supports binding to a Unix domain socket. A socket can be
  specified in the form `unix:///path/to/socket`. A new domain socket will be
  created at the given path. If the specified file path already exists, Consul
  will attempt to clear the file and create the domain socket in its place. The
  permissions of the socket file are tunable via the [`unix_sockets` config construct](#unix_sockets).

  When running Consul agent commands against Unix socket interfaces, use the
  `-http-addr` argument to specify the path to the socket. You can also place
  the desired values in the `CONSUL_HTTP_ADDR` environment variable.

  For TCP addresses, the variable values should be an IP address with the port. For
  example: `10.0.0.1:8500` and not `10.0.0.1`. However, ports are set separately in the
  <a href="#ports">`ports`</a> structure when defining them in a configuration file.

  The following keys are valid:

  * `dns` - The DNS server. Defaults to `client_addr`
  * `http` - The HTTP API. Defaults to `client_addr`
  * `https` - The HTTPS API. Defaults to `client_addr`

* <a name="advertise_addr"></a><a href="#advertise_addr">`advertise_addr`</a> Equivalent to
  the [`-advertise` command-line flag](#_advertise).

* <a name="serf_wan"></a><a href="#serf_wan_bind">`serf_wan`</a> Equivalent to
  the [`-serf-wan-bind` command-line flag](#_serf_wan_bind).

* <a name="serf_lan"></a><a href="#serf_lan_bind">`serf_lan`</a> Equivalent to
  the [`-serf-lan-bind` command-line flag](#_serf_lan_bind).

* <a name="advertise_addr_wan"></a><a href="#advertise_addr_wan">`advertise_addr_wan`</a> Equivalent to
  the [`-advertise-wan` command-line flag](#_advertise-wan).

* <a name="autopilot"></a><a href="#autopilot">`autopilot`</a> Added in Consul 0.8, this object
  allows a number of sub-keys to be set which can configure operator-friendly settings for Consul servers.
  For more information about Autopilot, see the [Autopilot Guide](/docs/guides/autopilot.html).

  The following sub-keys are available:

  * <a name="cleanup_dead_servers"></a><a href="#cleanup_dead_servers">`cleanup_dead_servers`</a> - This controls
    the automatic removal of dead server nodes periodically and whenever a new server is added to the cluster.
    Defaults to `true`.

  * <a name="last_contact_threshold"></a><a href="#last_contact_threshold">`last_contact_threshold`</a> - Controls
    the maximum amount of time a server can go without contact from the leader before being considered unhealthy.
    Must be a duration value such as `10s`. Defaults to `200ms`.

  * <a name="max_trailing_logs"></a><a href="#max_trailing_logs">`max_trailing_logs`</a> - Controls
    the maximum number of log entries that a server can trail the leader by before being considered unhealthy. Defaults
    to 250.

  * <a name="server_stabilization_time"></a><a href="#server_stabilization_time">`server_stabilization_time`</a> -
    Controls the minimum amount of time a server must be stable in the 'healthy' state before being added to the
    cluster. Only takes effect if all servers are running Raft protocol version 3 or higher. Must be a duration value
    such as `30s`. Defaults to `10s`.

  * <a name="redundancy_zone_tag"></a><a href="#redundancy_zone_tag">`redundancy_zone_tag`</a> - (Enterprise-only)
    This controls the [`-node-meta`](#_node_meta) key to use when Autopilot is separating servers into zones for
    redundancy. Only one server in each zone can be a voting member at one time. If left blank (the default), this
    feature will be disabled.

  * <a name="disable_upgrade_migration"></a><a href="#disable_upgrade_migration">`disable_upgrade_migration`</a> - (Enterprise-only)
    If set to `true`, this setting will disable Autopilot's upgrade migration strategy in Consul Enterprise of waiting
    until enough newer-versioned servers have been added to the cluster before promoting any of them to voters. Defaults
    to `false`.

* <a name="bootstrap"></a><a href="#bootstrap">`bootstrap`</a> Equivalent to the
  [`-bootstrap` command-line flag](#_bootstrap).

* <a name="bootstrap_expect"></a><a href="#bootstrap_expect">`bootstrap_expect`</a> Equivalent
  to the [`-bootstrap-expect` command-line flag](#_bootstrap_expect).

* <a name="bind_addr"></a><a href="#bind_addr">`bind_addr`</a> Equivalent to the
  [`-bind` command-line flag](#_bind).

* <a name="ca_file"></a><a href="#ca_file">`ca_file`</a> This provides a file path to a PEM-encoded
  certificate authority. The certificate authority is used to check the authenticity of client and
  server connections with the appropriate [`verify_incoming`](#verify_incoming) or
  [`verify_outgoing`](#verify_outgoing) flags.

* <a name="ca_path"></a><a href="#ca_path">`ca_path`</a> This provides a path to a directory of PEM-encoded
  certificate authority files. These certificate authorities are used to check the authenticity of client and
  server connections with the appropriate [`verify_incoming`](#verify_incoming) or
  [`verify_outgoing`](#verify_outgoing) flags.

* <a name="cert_file"></a><a href="#cert_file">`cert_file`</a> This provides a file path to a
  PEM-encoded certificate. The certificate is provided to clients or servers to verify the agent's
  authenticity. It must be provided along with [`key_file`](#key_file).

* <a name="check_update_interval"></a><a href="#check_update_interval">`check_update_interval`</a>
  This interval controls how often check output from
  checks in a steady state is synchronized with the server. By default, this is
  set to 5 minutes ("5m"). Many checks which are in a steady state produce
  slightly different output per run (timestamps, etc) which cause constant writes.
  This configuration allows deferring the sync of check output for a given interval to
  reduce write pressure. If a check ever changes state, the new state and associated
  output is synchronized immediately. To disable this behavior, set the value to "0s".

* <a name="client_addr"></a><a href="#client_addr">`client_addr`</a> Equivalent to the
  [`-client` command-line flag](#_client).

* <a name="datacenter"></a><a href="#datacenter">`datacenter`</a> Equivalent to the
  [`-datacenter` command-line flag](#_datacenter).

* <a name="data_dir"></a><a href="#data_dir">`data_dir`</a> Equivalent to the
  [`-data-dir` command-line flag](#_data_dir).

* <a name="disable_anonymous_signature"></a><a href="#disable_anonymous_signature">
  `disable_anonymous_signature`</a> Disables providing an anonymous signature for de-duplication
  with the update check. See [`disable_update_check`](#disable_update_check).

* <a name="disable_host_node_id"></a><a href="#disable_host_node_id">`disable_host_node_id`</a>
  Equivalent to the [`-disable-host-node-id` command-line flag](#_disable_host_node_id).

* <a name="disable_remote_exec"></a><a href="#disable_remote_exec">`disable_remote_exec`</a>
  Disables support for remote execution. When set to true, the agent will ignore any incoming
  remote exec requests. In versions of Consul prior to 0.8, this defaulted to false. In Consul
  0.8 the default was changed to true, to make remote exec opt-in instead of opt-out.

* <a name="disable_update_check"></a><a href="#disable_update_check">`disable_update_check`</a>
  Disables automatic checking for security bulletins and new version releases. This is disabled in
  Consul Enterprise.

* <a name="discard_check_output"></a><a href="#discard_check_output">`discard_check_output`</a>
  Discards the output of health checks before storing them. This reduces the number of writes
  to the Consul raft log in environments where health checks have volatile output like
  timestamps, process ids, ...

  * <a name="discovery_max_stale"></a><a href="#discovery_max_stale">`discovery_max_stale`</a> - Enables
    stale requests for all service discovery HTTP endpoints. This is equivalent to the
    [`max_stale`](#max_stale) configuration for DNS requests. If this value is zero (default), all service
    discovery HTTP endpoints are forwarded to the leader. If this value is greater than zero, any Consul server
    can handle the service discovery request. If a Consul server is behind the leader by more than `discovery_max_stale`,
    the query will be re-evaluated on the leader to get more up-to-date results. Consul agents also add a new
    `X-Consul-Effective-Consistency` response header which indicates if the agent did a stale read. `discover-max-stale`
    was introduced in Consul 1.0.7 as a way for Consul operators to force stale requests from clients at the agent level,
    and defaults to zero which matches default consistency behavior in earlier Consul versions.

* <a name="dns_config"></a><a href="#dns_config">`dns_config`</a> This object allows a number
  of sub-keys to be set which can tune how DNS queries are serviced. See this guide on
  [DNS caching](/docs/guides/dns-cache.html) for more detail.

  The following sub-keys are available:

  * <a name="allow_stale"></a><a href="#allow_stale">`allow_stale`</a> - Enables a stale query
    for DNS information. This allows any Consul server, rather than only the leader, to service
    the request. The advantage of this is you get linear read scalability with Consul servers.
    In versions of Consul prior to 0.7, this defaulted to false, meaning all requests are serviced
    by the leader, providing stronger consistency but less throughput and higher latency. In Consul
    0.7 and later, this defaults to true for better utilization of available servers.

  * <a name="max_stale"></a><a href="#max_stale">`max_stale`</a> - When [`allow_stale`](#allow_stale)
    is specified, this is used to limit how stale results are allowed to be. If a Consul server is
    behind the leader by more than `max_stale`, the query will be re-evaluated on the leader to get
    more up-to-date results. Prior to Consul 0.7.1 this defaulted to 5 seconds; in Consul 0.7.1
    and later this defaults to 10 years ("87600h") which effectively allows DNS queries to be answered
    by any server, no matter how stale. In practice, servers are usually only milliseconds behind the
    leader, so this lets Consul continue serving requests in long outage scenarios where no leader can
    be elected.

  * <a name="node_ttl"></a><a href="#node_ttl">`node_ttl`</a> - By default, this is "0s", so all
    node lookups are served with a 0 TTL value. DNS caching for node lookups can be enabled by
    setting this value. This should be specified with the "s" suffix for second or "m" for minute.

  * <a name="service_ttl"></a><a href="#service_ttl">`service_ttl`</a> - This is a sub-object
    which allows for setting a TTL on service lookups with a per-service policy. The "\*" wildcard
    service can be used when there is no specific policy available for a service. By default, all
    services are served with a 0 TTL value. DNS caching for service lookups can be enabled by
    setting this value.

  * <a name="enable_truncate"></a><a href="#enable_truncate">`enable_truncate`</a> - If set to
    true, a UDP DNS query that would return more than 3 records, or more than would fit into a valid
    UDP response, will set the truncated flag, indicating to clients that they should re-query
    using TCP to get the full set of records.

  * <a name="only_passing"></a><a href="#only_passing">`only_passing`</a> - If set to true, any
    nodes whose health checks are warning or critical will be excluded from DNS results. If false,
    the default, only nodes whose healthchecks are failing as critical will be excluded. For
    service lookups, the health checks of the node itself, as well as the service-specific checks
    are considered. For example, if a node has a health check that is critical then all services on
    that node will be excluded because they are also considered critical.

  * <a name="recursor_timeout"></a><a href="#recursor_timeout">`recursor_timeout`</a> - Timeout used
    by Consul when recursively querying an upstream DNS server. See <a href="#recursors">`recursors`</a>
    for more details. Default is 2s. This is available in Consul 0.7 and later.

  * <a name="disable_compression"></a><a href="#disable_compression">`disable_compression`</a> - If
    set to true, DNS responses will not be compressed. Compression was added and enabled by default
    in Consul 0.7.

  * <a name="udp_answer_limit"></a><a href="#udp_answer_limit">`udp_answer_limit`</a> - Limit the number of
    resource records contained in the answer section of a UDP-based DNS
    response. This parameter applies only to UDP DNS queries that are less than 512 bytes. This setting is deprecated
    and replaced in Consul 1.0.7 by <a href="#a_record_limit">`a_record_limit`</a>.

  * <a name="a_record_limit"></a><a href="#a_record_limit">`a_record_limit`</a> - Limit the number of
    resource records contained in the answer section of a A, AAAA or ANY DNS response (both TCP and UDP).
    When answering a question, Consul will use the complete list of
    matching hosts, shuffle the list randomly, and then limit the number of
    answers to `a_record_limit` (default: no limit). This limit does not apply to SRV records.

    In environments where [RFC 3484 Section 6](https://tools.ietf.org/html/rfc3484#section-6) Rule 9
    is implemented and enforced (i.e. DNS answers are always sorted and
    therefore never random), clients may need to set this value to `1` to
    preserve the expected randomized distribution behavior (note:
    [RFC 3484](https://tools.ietf.org/html/rfc3484) has been obsoleted by
    [RFC 6724](https://tools.ietf.org/html/rfc6724) and as a result it should
    be increasingly uncommon to need to change this value with modern
    resolvers).

* <a name="domain"></a><a href="#domain">`domain`</a> Equivalent to the
  [`-domain` command-line flag](#_domain).

* <a name="enable_acl_replication"></a><a href="#enable_acl_replication">`enable_acl_replication`</a> When
  set on a Consul server, enables [ACL replication](/docs/guides/acl.html#replication) without having to set
  the replication token via [`acl_replication_token`](#acl_replication_token). Instead, enable ACL replication
  and then introduce the token using the [agent token API](/api/agent.html#update-acl-tokens) on each server.
  See [`acl_replication_token`](#acl_replication_token) for more details.

* <a name="enable_agent_tls_for_checks"></a><a href="#enable_agent_tls_for_checks">`enable_agent_tls_for_checks`</a>
  When set, uses a subset of the agent's TLS configuration (`key_file`, `cert_file`, `ca_file`, `ca_path`, and
  `server_name`) to set up the HTTP client for HTTP health checks. This allows services requiring 2-way TLS to
  be checked using the agent's credentials. This was added in Consul 1.0.1 and defaults to false.

* <a name="enable_debug"></a><a href="#enable_debug">`enable_debug`</a> When set, enables some
  additional debugging features. Currently, this is only used to set the runtime profiling HTTP endpoints.

* <a name="enable_script_checks"></a><a href="#enable_script_checks">`enable_script_checks`</a> Equivalent to the
  [`-enable-script-checks` command-line flag](#_enable_script_checks).

* <a name="enable_syslog"></a><a href="#enable_syslog">`enable_syslog`</a> Equivalent to
  the [`-syslog` command-line flag](#_syslog).

* <a name="encrypt"></a><a href="#encrypt">`encrypt`</a> Equivalent to the
  [`-encrypt` command-line flag](#_encrypt).

* <a name="encrypt_verify_incoming"></a><a href="#encrypt_verify_incoming">`encrypt_verify_incoming`</a> -
  This is an optional parameter that can be used to disable enforcing encryption for incoming gossip in order
  to upshift from unencrypted to encrypted gossip on a running cluster. See [this section](/docs/agent/encryption.html#configuring-gossip-encryption-on-an-existing-cluster) for more information.
  Defaults to true.

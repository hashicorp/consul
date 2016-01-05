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

When loading configuration, Consul loads the configuration from files
and directories in lexical order. For example, configuration file `basic_config.json`
will be processed before `extra_config.json`. Configuration specified later
will be merged into configuration specified earlier. In most cases,
"merge" means that the later version will override the earlier. In
some cases, such as event handlers, merging appends the handlers to the
existing configuration. The exact merging behavior is specified for each
option below.

Consul also supports reloading configuration when it receives the
SIGHUP signal. Not all changes are respected, but those that are
are documented below in the
[Reloadable Configuration](#reloadable-configuration) section. The
[reload command](/docs/commands/reload.html) can also be used to trigger a
configuration reload.

## Command-line Options

The options below are all specified on the command-line.

* <a name="_advertise"></a><a href="#_advertise">`-advertise`</a> - The advertise
  address is used to change the address that we
  advertise to other nodes in the cluster. By default, the [`-bind`](#_bind) address is
  advertised. However, in some cases, there may be a routable address that cannot
  be bound. This flag enables gossiping a different address to support this.
  If this address is not routable, the node will be in a constant flapping state
  as other nodes will treat the non-routability as a failure.

* <a name="_advertise-wan"></a><a href="#_advertise-wan">`-advertise-wan`</a> - The advertise wan
  address is used to change the address that we advertise to server nodes joining
  through the WAN. By default, the [`-advertise`](#_advertise) address is advertised.
  However, in some cases all members of all datacenters cannot be on the same
  physical or virtual network, especially on hybrid setups mixing cloud and private datacenters.
  This flag enables server nodes gossiping through the public network for the WAN while using
  private VLANs for gossiping to each other and their client agents.

* <a name="_atlas"></a><a href="#_atlas">`-atlas`</a> - This flag
  enables [Atlas](https://atlas.hashicorp.com) integration.
  It is used to provide the Atlas infrastructure name and the SCADA connection. The format of 
  this is `username/environment`. This enables Atlas features such as the Monitoring UI 
  and node auto joining.

* <a name="_atlas_join"></a><a href="#_atlas_join">`-atlas-join`</a> - When set, enables auto-join
  via Atlas. Atlas will track the most
  recent members to join the infrastructure named by [`-atlas`](#_atlas) and automatically
  join them on start. For servers, the LAN and WAN pool are both joined.

* <a name="_atlas_token"></a><a href="#_atlas_token">`-atlas-token`</a> - Provides the Atlas
  API authentication token. This can also be provided
  using the `ATLAS_TOKEN` environment variable. Required for use with Atlas.

* <a name="_atlas_endpoint"></a><a href="#_atlas_endpoint">`-atlas-endpoint`</a> - The endpoint
  address used for Atlas integration. Used only if the `-atlas` and
  `-atlas-token` options are specified. This is optional, and defaults to the
  public Atlas endpoints. This can also be specified using the `SCADA_ENDPOINT`
  environment variable. The CLI option takes precedence, followed by the
  configuration file directive, and lastly, the environment variable.

* <a name="_bootstrap"></a><a href="#_bootstrap">`-bootstrap`</a> - This flag is used to control if a
  server is in "bootstrap" mode. It is important that
  no more than one server *per* datacenter be running in this mode. Technically, a server in bootstrap mode
  is allowed to self-elect as the Raft leader. It is important that only a single node is in this mode;
  otherwise, consistency cannot be guaranteed as multiple nodes are able to self-elect.
  It is not recommended to use this flag after a cluster has been bootstrapped.

* <a name="_bootstrap_expect"></a><a href="#_bootstrap_expect">`-bootstrap-expect`</a> - This flag
  provides the number of expected servers in the datacenter.
  Either this value should not be provided or the value must agree with other servers in
  the cluster. When provided, Consul waits until the specified number of servers are
  available and then bootstraps the cluster. This allows an initial leader to be elected
  automatically. This cannot be used in conjunction with the legacy [`-bootstrap`](#_bootstrap) flag.

* <a name="_bind"></a><a href="#_bind">`-bind`</a> - The address that should be bound to
  for internal cluster communications.
  This is an IP address that should be reachable by all other nodes in the cluster.
  By default, this is "0.0.0.0", meaning Consul will use the first available private
  IP address. Consul uses both TCP and UDP and the same port for both. If you
  have any firewalls, be sure to allow both protocols.

* <a name="_client"></a><a href="#_client">`-client`</a> - The address to which
  Consul will bind client interfaces,
  including the HTTP, DNS, and RPC servers. By default, this is "127.0.0.1",
  allowing only loopback connections. The RPC address is used by other Consul
  commands, such as `consul members`, in order to query a running Consul agent.

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
  [`config-file`](#_config_file) option above. This option can be specified mulitple times
  to load multiple directories. Sub-directories of the config directory are not loaded.
  For more information on the format of the configuration files, see the
  [Configuration Files](#configuration_files) section.

* <a name="_data_dir"></a><a href="#_data_dir">`-data-dir`</a> - This flag provides
  a data directory for the agent to store state.
  This is required for all agents. The directory should be durable across reboots.
  This is especially critical for agents that are running in server mode as they
  must be able to persist cluster state. Additionally, the directory must support
  the use of filesystem locking, meaning some types of mounted folders (e.g. VirtualBox
  shared folders) may not be suitable.

* <a name="_dev"></a><a href="#_dev">`-dev`</a> - Enable development server
  mode. This is useful for quickly starting a Consul agent with all persistence
  options turned off, enabling an in-memory server which can be used for rapid
  prototyping or developing against the API. This mode is **not** intended for
  production use as it does not write any data to disk.

* <a name="_dc"></a><a href="#_dc">`-dc`</a> - This flag controls the datacenter in
  which the agent is running. If not provided,
  it defaults to "dc1". Consul has first-class support for multiple datacenters, but
  it relies on proper configuration. Nodes in the same datacenter should be on a single
  LAN.

* <a name="_domain"></a><a href="#_domain">`-domain`</a> - By default, Consul responds to DNS queries
  in the "consul." domain. This flag can be used to change that domain. All queries in this domain
  are assumed to be handled by Consul and will not be recursively resolved.

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

* <a name="_http_port"></a><a href="#_http_port">`-http-port`</a> - the HTTP API port to listen on.
  This overrides the default port 8500. This option is very useful when deploying Consul
  to an environment which communicates the HTTP port through the environment e.g. PaaS like CloudFoundry, allowing
  you to set the port directly via a Procfile.

* <a name="_join"></a><a href="#_join">`-join`</a> - Address of another agent
  to join upon starting up. This can be
  specified multiple times to specify multiple agents to join. If Consul is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't join any nodes when it starts up.

* <a name="_retry_join"></a><a href="#_retry_join">`-retry-join`</a> - Similar
  to [`-join`](#_join) but allows retrying a join if the first
  attempt fails. This is useful for cases where we know the address will become
  available eventually.

* <a name="_retry_interval"></a><a href="#_retry_interval">`-retry-interval`</a> - Time
  to wait between join attempts. Defaults to 30s.

* <a name="_retry_max"></a><a href="#_retry_max">`-retry-max`</a> - The maximum number
  of [`-join`](#_join) attempts to be made before exiting
  with return code 1. By default, this is set to 0 which is interpreted as infinite
  retries.

* <a name="_join_wan"></a><a href="#_join_wan">`-join-wan`</a> - Address of another
  wan agent to join upon starting up. This can be
  specified multiple times to specify multiple WAN agents to join. If Consul is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't [`-join-wan`](#_join_wan) any nodes when it starts up.

* <a name="_retry_join_wan"></a><a href="#_retry_join_wan">`-retry-join-wan`</a> - Similar
  to [`retry-join`](#_retry_join) but allows retrying a wan join if the first attempt fails.
  This is useful for cases where we know the address will become
  available eventually.

* <a name="_retry_interval_wan"></a><a href="#_retry_interval_wan">`-retry-interval-wan`</a> - Time
  to wait between [`-join-wan`](#_join_wan) attempts.
  Defaults to 30s.

* <a name="_retry_max_wan"></a><a href="#_retry_max_wan">`-retry-max-wan`</a> - The maximum
  number of [`-join-wan`](#_join_wan) attempts to be made before exiting with return code 1.
  By default, this is set to 0 which is interpreted as infinite retries.

* <a name="_log_level"></a><a href="#_log_level">`-log-level`</a> - The level of logging to
  show after the Consul agent has started. This defaults to "info". The available log levels are
  "trace", "debug", "info", "warn", and "err". Note that you can always connect to an
  agent via [`consul monitor`](/docs/commands/monitor.html) and use any log level. Also, the
  log level can be changed during a config reload.

* <a name="_node"></a><a href="#_node">`-node`</a> - The name of this node in the cluster.
  This must be unique within the cluster. By default this is the hostname of the machine.

* <a name="_pid_file"></a><a href="#_pid_file">`-pid-file`</a> - This flag provides the file
  path for the agent to store its PID. This is useful for sending signals (for example, `SIGINT`
  to close the agent or `SIGHUP` to update check definite

* <a name="_protocol"></a><a href="#_protocol">`-protocol`</a> - The Consul protocol version to
  use. This defaults to the latest version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Consul by running `consul -v`.

* <a name="_recursor"></a><a href="#_recursor">`-recursor`</a> - Specifies the address of an upstream DNS
  server. This option may be provided multiple times, and is functionally
  equivalent to the [`recursors` configuration option](#recursors).

* <a name="_rejoin"></a><a href="#_rejoin">`-rejoin`</a> - When provided, Consul will ignore a
  previous leave and attempt to rejoin the cluster when starting. By default, Consul treats leave
  as a permanent intent and does not attempt to join the cluster again when starting. This flag
  allows the previous state to be used to rejoin the cluster.

* <a name="_server"></a><a href="#_server">`-server`</a> - This flag is used to control if an
  agent is in server or client mode. When provided,
  an agent will act as a Consul server. Each Consul cluster must have at least one server and ideally
  no more than 5 per datacenter. All servers participate in the Raft consensus algorithm to ensure that
  transactions occur in a consistent, linearizable manner. Transactions modify cluster state, which
  is maintained on all server nodes to ensure availability in the case of node failure. Server nodes also
  participate in a WAN gossip pool with server nodes in other datacenters. Servers act as gateways
  to other datacenters and forward traffic as appropriate.

* <a name="_syslog"></a><a href="#_syslog">`-syslog`</a> - This flag enables logging to syslog. This
  is only supported on Linux and OSX. It will result in an error if provided on Windows.

* <a name="_ui"></a><a href="#_ui">`-ui`</a> - Enables the built-in web UI
  server and the required HTTP routes. This eliminates the need to maintain the
  Consul web UI files separately from the binary.

* <a name="_ui_dir"></a><a href="#_ui_dir">`-ui-dir`</a> - This flag provides the directory containing
  the Web UI resources for Consul. This will automatically enable the Web UI. The directory must be
  readable to the agent.

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
  ]
}
```

#### Configuration Key Reference

* <a name="acl_datacenter"></a><a href="#acl_datacenter">`acl_datacenter`</a> - Only
  used by servers. This designates the datacenter which
  is authoritative for ACL information. It must be provided to enable ACLs.
  All servers and datacenters must agree on the ACL datacenter. Setting it on
  the servers is all you need for enforcement, but for the APIs to forward properly
  from the clients, it must be set on them too. Future changes may move
  enforcement to the edges, so it's best to just set `acl_datacenter` on all nodes.

* <a name="acl_default_policy"></a><a href="#acl_default_policy">`acl_default_policy`</a> - Either
  "allow" or "deny"; defaults to "allow". The default policy controls the behavior of a token when
  there is no matching rule. In "allow" mode, ACLs are a blacklist: any operation not specifically
  prohibited is allowed. In "deny" mode, ACLs are a whitelist: any operation not
  specifically allowed is blocked.

* <a name="acl_down_policy"></a><a href="#acl_down_policy">`acl_down_policy`</a> - Either
  "allow", "deny" or "extend-cache"; "extend-cache" is the default. In the case that the
  policy for a token cannot be read from the [`acl_datacenter`](#acl_datacenter) or leader
  node, the down policy is applied. In "allow" mode, all actions are permitted, "deny" restricts
  all operations, and "extend-cache" allows any cached ACLs to be used, ignoring their TTL
  values. If a non-cached ACL is used, "extend-cache" acts like "deny".

* <a name="acl_master_token"></a><a href="#acl_master_token">`acl_master_token`</a> - Only used
  for servers in the [`acl_datacenter`](#acl_datacenter). This token will be created with management-level
  permissions if it does not exist. It allows operators to bootstrap the ACL system
  with a token ID that is well-known.
  <br><br>
  Note that the `acl_master_token` is only installed when a server acquires cluster leadership. If
  you would like to install or change the `acl_master_token`, set the new value for `acl_master_token`
  in the configuration for all servers. Once this is done, restart the current leader to force a
  leader election. If the acl_master_token is not supplied, then the servers do not create a master
  token. When you provide a value, it can be any string value. Using a UUID would ensure that it looks
  the same as the other tokens, but isn't strictly necessary.

* <a name="acl_token"></a><a href="#acl_token">`acl_token`</a> - When provided, the agent will use this
  token when making requests to the Consul servers. Clients can override this token on a per-request
  basis by providing the "?token" query parameter. When not provided, the empty token, which maps to
  the 'anonymous' ACL policy, is used.

* <a name="acl_ttl"></a><a href="#acl_ttl">`acl_ttl`</a> - Used to control Time-To-Live caching of ACLs.
  By default, this is 30 seconds. This setting has a major performance impact: reducing it will cause
  more frequent refreshes while increasing it reduces the number of caches. However, because the caches
  are not actively invalidated, ACL policy may be stale up to the TTL value.

* <a name="addresses"></a><a href="#addresses">`addresses`</a> - This is a nested object that allows
  setting bind addresses.
  <br><br>
  Both `rpc` and `http` support binding to Unix domain sockets. A socket can be
  specified in the form `unix:///path/to/socket`. A new domain socket will be
  created at the given path. If the specified file path already exists, Consul
  will attempt to clear the file and create the domain socket in its place.
  <br><br>
  The permissions of the socket file are tunable via the [`unix_sockets` config
  construct](#unix_sockets).
  <br><br>
  When running Consul agent commands against Unix socket interfaces, use the
  `-rpc-addr` or `-http-addr` arguments to specify the path to the socket. You
  can also place the desired values in `CONSUL_RPC_ADDR` and `CONSUL_HTTP_ADDR`
  environment variables. For TCP addresses, these should be in the form ip:port.
  <br><br>
  The following keys are valid:
  * `dns` - The DNS server. Defaults to `client_addr`
  * `http` - The HTTP API. Defaults to `client_addr`
  * `https` - The HTTPS API. Defaults to `client_addr`
  * `rpc` - The RPC endpoint. Defaults to `client_addr`

* <a name="advertise_addr"></a><a href="#advertise_addr">`advertise_addr`</a> Equivalent to
  the [`-advertise` command-line flag](#_advertise).

* <a name="advertise_addrs"></a><a href="#advertise_addrs">`advertise_addrs`</a> Allows to set
  the advertised addresses for SerfLan, SerfWan and RPC together with the port. This gives
  you more control than (#_advertise) or (#_advertise-wan) while it serves the same purpose.
  These settings might override (#_advertise) and (#_advertise-wan).
  <br><br>
  This is a nested setting that allows the following keys:
  * `serf_lan` - The SerfLan address. Accepts values in the form of "host:port" like "10.23.31.101:8301".
  * `serf_wan` - The SerfWan address. Accepts values in the form of "host:port" like "10.23.31.101:8302".
  * `rpc` - The RPC address. Accepts values in the form of "host:port" like "10.23.31.101:8400".

* <a name="advertise_addr_wan"></a><a href="#advertise_addr_wan">`advertise_addr_wan`</a> Equivalent to
  the [`-advertise-wan` command-line flag](#_advertise-wan).

* <a name="atlas_acl_token"></a><a href="#atlas_acl_token">`atlas_acl_token`</a> When provided,
  any requests made by Atlas will use this ACL token unless explicitly overriden. When not provided
  the [`acl_token`](#acl_token) is used. This can be set to 'anonymous' to reduce permission below
  that of [`acl_token`](#acl_token).

* <a name="atlas_infrastructure"></a><a href="#atlas_infrastructure">`atlas_infrastructure`</a>
  Equivalent to the [`-atlas` command-line flag](#_atlas).

* <a name="atlas_join"></a><a href="#atlas_join">`atlas_join`</a> Equivalent to the
  [`-atlas-join` command-line flag](#_atlas_join).

* <a name="atlas_token"></a><a href="#atlas_token">`atlas_token`</a> Equivalent to the
  [`-atlas-token` command-line flag](#_atlas_token).

* <a name="atlas_endpoint"></a><a href="#atlas_endpoint">`atlas_endpoint`</a> Equivalent to the
  [`-atlas-endpoint` command-line flag](#_atlas_endpoint).

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
  [`-dc` command-line flag](#_dc).

* <a name="data_dir"></a><a href="#data_dir">`data_dir`</a> Equivalent to the
  [`-data-dir` command-line flag](#_data_dir).

* <a name="disable_anonymous_signature"></a><a href="#disable_anonymous_signature">
  `disable_anonymous_signature`</a> Disables providing an anonymous signature for de-duplication
  with the update check. See [`disable_update_check`](#disable_update_check).

* <a name="disable_remote_exec"></a><a href="#disable_remote_exec">`disable_remote_exec`</a>
  Disables support for remote execution. When set to true, the agent will ignore any incoming
  remote exec requests.

* <a name="disable_update_check"></a><a href="#disable_update_check">`disable_update_check`</a>
  Disables automatic checking for security bulletins and new version releases.

* <a name="dns_config"></a><a href="#dns_config">`dns_config`</a> This object allows a number
  of sub-keys to be set which can tune how DNS queries are serviced. See this guide on
  [DNS caching](/docs/guides/dns-cache.html) for more detail.
  <br><br>
  The following sub-keys are available:

  * <a name="allow_stale"></a><a href="#allow_stale">`allow_stale`</a> - Enables a stale query
  for DNS information. This allows any Consul server, rather than only the leader, to service
  the request. The advantage of this is you get linear read scalability with Consul servers.
  By default, this is false, meaning all requests are serviced by the leader, providing stronger
  consistency but less throughput and higher latency.

  * <a name="max_stale"></a><a href="#max_stale">`max_stale`</a> When [`allow_stale`](#allow_stale)
  is specified, this is used to limit how
  stale results are allowed to be. By default, this is set to "5s":
  if a Consul server is more than 5 seconds behind the leader, the query will be
  re-evaluated on the leader to get more up-to-date results.

  * <a name="node_ttl"></a><a href="#node_ttl">`node_ttl`</a> By default, this is "0s", so all
  node lookups are served with a 0 TTL value. DNS caching for node lookups can be enabled by
  setting this value. This should be specified with the "s" suffix for second or "m" for minute.

  * <a name="service_ttl"></a><a href="#service_ttl">`service_ttl`</a> This is a sub-object
  which allows for setting a TTL on service lookups with a per-service policy. The "*" wildcard
  service can be used when there is no specific policy available for a service. By default, all
  services are served with a 0 TTL value. DNS caching for service lookups can be enabled by
  setting this value.

  * <a name="enable_truncate"></a><a href="#enable_truncate">`enable_truncate`</a> If set to
  true, a UDP DNS query that would return more than 3 records will set the truncated flag,
  indicating to clients that they should re-query using TCP to get the full set of records.

  * <a name="only_passing"></a><a href="#only_passing">`only_passing`</a> If set to true, any
  nodes whose healthchecks are not passing will be excluded from DNS results. By default (or
  if set to false), only nodes whose healthchecks are failing as critical will be excluded.

* <a name="domain"></a><a href="#domain">`domain`</a> Equivalent to the
  [`-domain` command-line flag](#_domain).

* <a name="enable_debug"></a><a href="#enable_debug">`enable_debug`</a> When set, enables some
  additional debugging features. Currently, this is only used to set the runtime profiling HTTP endpoints.

* <a name="enable_syslog"></a><a href="#enable_syslog">`enable_syslog`</a> Equivalent to
  the [`-syslog` command-line flag](#_syslog).

* <a name="encrypt"></a><a href="#encrypt">`encrypt`</a> Equivalent to the
  [`-encrypt` command-line flag](#_encrypt).

* <a name="key_file"></a><a href="#key_file">`key_file`</a> This provides a the file path to a
  PEM-encoded private key. The key is used with the certificate to verify the agent's authenticity.
  This must be provided along with [`cert_file`](#cert_file).

* <a name="http_api_response_headers"></a><a href="#http_api_response_headers">`http_api_response_headers`</a>
  This object allows adding headers to the HTTP API
  responses. For example, the following config can be used to enable
  [CORS](http://en.wikipedia.org/wiki/Cross-origin_resource_sharing) on
  the HTTP API endpoints:

    ```javascript
      {
        "http_api_response_headers": {
            "Access-Control-Allow-Origin": "*"
        }
      }
    ```

* <a name="leave_on_terminate"></a><a href="#leave_on_terminate">`leave_on_terminate`</a> If
  enabled, when the agent receives a TERM signal,
  it will send a `Leave` message to the rest of the cluster and gracefully
  leave. Defaults to false.

* <a name="log_level"></a><a href="#log_level">`log_level`</a> Equivalent to the
  [`-log-level` command-line flag](#_log_level).

* <a name="node_name"></a><a href="#node_name">`node_name`</a> Equivalent to the
  [`-node` command-line flag](#_node).

* <a name="ports"></a><a href="#ports">`ports`</a> This is a nested object that allows setting
  the bind ports for the following keys:
    * <a name="dns_port"></a><a href="#dns_port">`dns`</a> - The DNS server, -1 to disable. Default 8600.
    * <a name="http_port"></a><a href="#http_port">`http`</a> - The HTTP API, -1 to disable. Default 8500.
    * <a name="https_port"></a><a href="#https_port">`https`</a> - The HTTPS API, -1 to disable. Default -1 (disabled).
    * <a name="rpc_port"></a><a href="#rpc_port">`rpc`</a> - The RPC endpoint. Default 8400.
    * <a name="serf_lan_port"></a><a href="#serf_lan_port">`serf_lan`</a> - The Serf LAN port. Default 8301.
    * <a name="serf_wan_port"></a><a href="#serf_wan_port">`serf_wan`</a> - The Serf WAN port. Default 8302.
    * <a name="server_rpc_port"></a><a href="#server_rpc_port">`server`</a> - Server RPC address. Default 8300.

* <a name="protocol"></a><a href="#protocol">`protocol`</a> Equivalent to the
  [`-protocol` command-line flag](#_protocol).

* <a name="reap"></a><a href="#reap">`reap`</a> controls Consul's automatic reaping of child processes, which
  is useful if Consul is running as PID 1 in a Docker container. If this isn't specified, then Consul will
  automatically reap child processes if it detects it is running as PID 1. If this is specified, then it
  controls reaping regardless of Consul's PID.

* <a name="recursor"></a><a href="#recursor">`recursor`</a> Provides a single recursor address.
  This has been deprecated, and the value is appended to the [`recursors`](#recursors) list for
  backwards compatibility.

* <a name="recursors"></a><a href="#recursors">`recursors`</a> This flag provides addresses of
  upstream DNS servers that are used to recursively resolve queries if they are not inside the service
  domain for consul. For example, a node can use Consul directly as a DNS server, and if the record is
  outside of the "consul." domain, the query will be resolved upstream.

* <a name="rejoin_after_leave"></a><a href="#rejoin_after_leave">`rejoin_after_leave`</a> Equivalent
  to the [`-rejoin` command-line flag](#_rejoin).

* <a name="retry_join"></a><a href="#retry_join">`retry_join`</a> Equivalent to the
  [`-retry-join` command-line flag](#_retry_join). Takes a list
  of addresses to attempt joining every [`retry_interval`](#_retry_interval) until at least one
  [`-join`](#_join) works.

* <a name="retry_interval"></a><a href="#retry_interval">`retry_interval`</a> Equivalent to the
  [`-retry-interval` command-line flag](#_retry_interval).

* <a name="retry_join_wan"></a><a href="#retry_join_wan">`retry_join_wan`</a> Equivalent to the
  [`-retry-join-wan` command-line flag](#_retry_join_wan). Takes a list
  of addresses to attempt joining to WAN every [`retry_interval_wan`](#_retry_interval_wan) until at least one
  [`-join-wan`](#_join_wan) works.

* <a name="retry_interval_wan"></a><a href="#retry_interval_wan">`retry_interval_wan`</a> Equivalent to the
  [`-retry-interval-wan` command-line flag](#_retry_interval_wan).

* <a name="server"></a><a href="#server">`server`</a> Equivalent to the
  [`-server` command-line flag](#_server).

* <a name="server_name"></a><a href="#server_name">`server_name`</a> When provided, this overrides
  the [`node_name`](#_node) for the TLS certificate. It can be used to ensure that the certificate
  name matches the hostname we declare.

* <a name="session_ttl_min"></a><a href="#session_ttl_min">`session_ttl_min`</a>
  The minimum allowed session TTL. This ensures sessions are not created with
  TTL's shorter than the specified limit. It is recommended to keep this limit
  at or above the default to encourage clients to send infrequent heartbeats.
  Defaults to 10s.

* <a name="skip_leave_on_interrupt"></a><a href="#skip_leave_on_interrupt">`skip_leave_on_interrupt`</a>
  This is similar to [`leave_on_terminate`](#leave_on_terminate) but
  only affects interrupt handling. By default, an interrupt (such as hitting
  Control-C in a shell) causes Consul to gracefully leave. Setting this to true
  disables that. Defaults to false.

* <a name="start_join"></a><a href="#start_join">`start_join`</a> An array of strings specifying addresses
  of nodes to [`-join`](#_join) upon startup.

* <a name="start_join_wan"></a><a href="#start_join_wan">`start_join_wan`</a> An array of strings specifying
  addresses of WAN nodes to [`-join-wan`](#_join_wan) upon startup.

* <a name="statsd_addr"></a><a href="#statsd_addr">`statsd_addr`</a> This provides the address of a
  statsd instance in the format `host:port`.  If provided, Consul will send various telemetry information
  to that instance for aggregation. This can be used to capture runtime information. This sends UDP packets
  only and can be used with statsd or statsite.

* <a name="dogstatsd_addr"></a><a href="#dogstatsd_addr">`dogstatsd_addr`</a> This provides the
  address of a DogStatsD instance in the format `host:port`. DogStatsD is a protocol-compatible flavor of
  statsd, with the added ability to decorate metrics with tags and event information. If provided, Consul will
  send various telemetry information to that instance for aggregation. This can be used to capture runtime
  information.

* <a name="dogstatsd_tags"></a><a href="#dogstatsd_tags">`dogstatsd_tags`</a> This provides a list of global tags
  that will be added to all telemetry packets sent to DogStatsD. It is a list of strings, where each string
  looks like "my_tag_name:my_tag_value".

* <a name="statsite_addr"></a><a href="#statsite_addr">`statsite_addr`</a> This provides the address of a
  statsite instance in the format `host:port`. If provided, Consul will stream various telemetry information
  to that instance for aggregation. This can be used to capture runtime information. This streams via TCP and
  can only be used with statsite.

* <a name="statsite_prefix"></a><a href="#statsite_prefix">`statsite_prefix`</a>
  The prefix used while writing all telemetry data to statsite. By default, this
  is set to "consul".

* <a name="syslog_facility"></a><a href="#syslog_facility">`syslog_facility`</a> When
  [`enable_syslog`](#enable_syslog) is provided, this controls to which
  facility messages are sent. By default, `LOCAL0` will be used.

* <a name="ui"></a><a href="#ui">`ui`</a> - Equivalent to the [`-ui`](#_ui)
  command-line flag.

* <a name="ui_dir"></a><a href="#ui_dir">`ui_dir`</a> - Equivalent to the
  [`-ui-dir`](#_ui_dir) command-line flag.

* <a name="unix_sockets"></a><a href="#unix_sockets">`unix_sockets`</a> - This
  allows tuning the ownership and permissions of the
  Unix domain socket files created by Consul. Domain sockets are only used if
  the HTTP or RPC addresses are configured with the `unix://` prefix. The
  following options are valid within this construct and apply globally to all
  sockets created by Consul:
  <br>
  * `user` - The name or ID of the user who will own the socket file.
  * `group` - The group ID ownership of the socket file. Note that this option
    currently only supports numeric IDs.
  * `mode` - The permission bits to set on the file.
  <br>
  It is important to note that this option may have different effects on
  different operating systems. Linux generally observes socket file permissions
  while many BSD variants ignore permissions on the socket file itself. It is
  important to test this feature on your specific distribution. This feature is
  currently not functional on Windows hosts.

* <a name="verify_incoming"></a><a href="#verify_incoming">`verify_incoming`</a> - If
  set to true, Consul requires that all incoming
  connections make use of TLS and that the client provides a certificate signed
  by the Certificate Authority from the [`ca_file`](#ca_file). By default, this is false, and
  Consul will not enforce the use of TLS or verify a client's authenticity. This
  applies to both server RPC and to the HTTPS API. Note: to enable the HTTPS API, you
  must define an HTTPS port via the [`ports`](#ports) configuration. By default, HTTPS
  is disabled.

* <a name="verify_outgoing"></a><a href="#verify_outgoing">`verify_outgoing`</a> - If set to
  true, Consul requires that all outgoing connections
  make use of TLS and that the server provides a certificate that is signed by
  the Certificate Authority from the [`ca_file`](#ca_file). By default, this is false, and Consul
  will not make use of TLS for outgoing connections. This applies to clients and servers
  as both will make outgoing connections.

* <a name="verify_server_hostname"></a><a href="#verify_server_hostname">`verify_server_hostname`</a> - If set to
  true, Consul verifies for all outgoing connections that the TLS certificate presented by the servers
  matches "server.<datacenter>.<domain>" hostname. This implies `verify_outgoing`.
  By default, this is false, and Consul does not verify the hostname of the certificate, only
  that it is signed by a trusted CA. This setting is important to prevent a compromised
  client from being restarted as a server, and thus being able to perform a MITM attack
  or to be added as a Raft peer. This is new in 0.5.1.

* <a name="watches"></a><a href="#watches">`watches`</a> - Watches is a list of watch
  specifications which allow an external process to be automatically invoked when a
  particular data view is updated. See the
   [watch documentation](/docs/agent/watches.html) for more detail. Watches can be
   modified when the configuration is reloaded.

## Ports Used

Consul requires up to 5 different ports to work properly, some on
TCP, UDP, or both protocols. Below we document the requirements for each
port.

* Server RPC (Default 8300). This is used by servers to handle incoming
  requests from other agents. TCP only.

* Serf LAN (Default 8301). This is used to handle gossip in the LAN.
  Required by all agents. TCP and UDP.

* Serf WAN (Default 8302). This is used by servers to gossip over the
  WAN to other servers. TCP and UDP.

* CLI RPC (Default 8400). This is used by all agents to handle RPC
  from the CLI. TCP only.

* HTTP API (Default 8500). This is used by clients to talk to the HTTP
  API. TCP only.

* DNS Interface (Default 8600). Used to resolve DNS queries. TCP and UDP.

## <a id="reloadable-configuration"></a>Reloadable Configuration</a>

Reloading configuration does not reload all configuration items. The
items which are reloaded include:

* Log level
* Checks
* Services
* Watches
* HTTP Client Address
* Atlas Token
* Atlas Infrastructure
* Atlas Endpoint

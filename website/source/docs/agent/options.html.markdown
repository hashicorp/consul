---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
description: |-
  The agent has various configuration options that can be specified via the command-line or via configuration files. All of the configuration options are completely optional and their defaults will be specified with their descriptions.
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
are documented below. The [reload command](/docs/commands/reload.html)
can also be used to trigger a configuration reload.

## Command-line Options

The options below are all specified on the command-line.

* `-advertise` - The advertise address is used to change the address that we
  advertise to other nodes in the cluster. By default, the `-bind` address is
  advertised. However, in some cases, there may be a routable address that cannot
  be bound to. This flag enables gossiping a different address to support this.
  If this address is not routable, the node will be in a constant flapping state,
  as other nodes will treat the non-routability as a failure.

* `-bootstrap` - This flag is used to control if a server is in "bootstrap" mode. It is important that
  no more than one server *per* datacenter be running in this mode. Technically, a server in bootstrap mode
  is allowed to self-elect as the Raft leader. It is important that only a single node is in this mode,
  because otherwise consistency cannot be guaranteed if multiple nodes are able to self-elect.
  It is not recommended to use this flag after a cluster has been bootstrapped.

* `-bootstrap-expect` - This flag provides the number of expected servers in the datacenter.
  Either this value should not be provided, or the value must agree with other servers in
  the cluster. When provided, Consul waits until the specified number of servers are
  available, and then bootstraps the cluster. This allows an initial leader to be elected
  automatically. This cannot be used in conjunction with the `-bootstrap` flag.

* `-bind` - The address that should be bound to for internal cluster communications.
  This is an IP address that should be reachable by all other nodes in the cluster.
  By default this is "0.0.0.0", meaning Consul will use the first available private
  IP address. Consul uses both TCP and UDP and use the same port for both, so if you
  have any firewalls be sure to allow both protocols.

* `-client` - The address that Consul will bind to client interfaces. This
  includes the HTTP, DNS, and RPC servers. By default this is "127.0.0.1"
  allowing only loopback connections. The RPC address is used by other Consul
  commands, such as `consul members`, in order to query a running Consul agent.

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

* `-data-dir` - This flag provides a data directory for the agent to store state.
  This is required for all agents. The directory should be durable across reboots.
  This is especially critical for agents that are running in server mode, as they
  must be able to persist the cluster state. Additional, the directory must support
  the use of filesystem locking, meaning some types of mounted folders (e.g. VirtualBox
  shared folders) may not be suitable.

* `-dc` - This flag controls the datacenter the agent is running in. If not provided
  it defaults to "dc1". Consul has first class support for multiple data centers but
  it relies on proper configuration. Nodes in the same datacenter should be on a single
  LAN.

* `-encrypt` - Specifies the secret key to use for encryption of Consul
  network traffic. This key must be 16-bytes that are base64 encoded. The
  easiest way to create an encryption key is to use `consul keygen`. All
  nodes within a cluster must share the same encryption key to communicate.

* `-join` - Address of another agent to join upon starting up. This can be
  specified multiple times to specify multiple agents to join. If Consul is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't join any nodes when it starts up.

* `-retry-join` - Similar to `-join`, but allows retrying a join if the first
  attempt fails. This is useful for cases where we know the address will become
  available eventually.

* `-retry-interval` - Time to wait between join attempts. Defaults to 30s.

* `-retry-max` - The maximum number of join attempts to be made before exiting
  with return code 1. By default, this is set to 0, which will continue to
  retry the join indefinitely.

* `-log-level` - The level of logging to show after the Consul agent has
  started. This defaults to "info". The available log levels are "trace",
  "debug", "info", "warn", "err". This is the log level that will be shown
  for the agent output, but note you can always connect via `consul monitor`
  to an agent at any log level. The log level can be changed during a
  config reload.

* `-node` - The name of this node in the cluster. This must be unique within
  the cluster. By default this is the hostname of the machine.

* `-protocol` - The Consul protocol version to use. This defaults to the latest
  version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Consul by running `consul -v`.

* `-rejoin` - When provided Consul will ignore a previous leave and attempt to
  rejoin the cluster when starting. By default, Consul treats leave as a permanent
  intent, and does not attempt to join the cluster again when starting. This flag
  allows the previous state to be used to rejoin the cluster.

* `-server` - This flag is used to control if an agent is in server or client mode. When provided,
  an agent will act as a Consul server. Each Consul cluster must have at least one server, and ideally
  no more than 5 *per* datacenter. All servers participate in the Raft consensus algorithm, to ensure that
  transactions occur in a consistent, linearizable manner. Transactions modify cluster state, which
  is maintained on all server nodes to ensure availability in the case of node failure. Server nodes also
  participate in a WAN gossip pool with server nodes in other datacenters. Servers act as gateways
  to other datacenters and forward traffic as appropriate.

* `-syslog` - This flag enables logging to syslog. This is only supported on Linux
  and OSX. It will result in an error if provided on Windows.

* `-ui-dir` - This flag provides a the directory containing the Web UI resources
  for Consul. This must be provided to enable the Web UI. Directory must be readable.

* `-pid-file` - This flag provides the file path for the agent to store it's PID. This is useful for
  sending signals to the agent, such as `SIGINT` to close it or `SIGHUP` to update check definitions.

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

* `acl_datacenter` - Only used by servers. This designates the datacenter which
   is authoritative for ACL information. It must be provided to enable ACLs.
   All servers and datacenters must agree on the ACL datacenter.

* `acl_default_policy` - Either "allow" or "deny", defaults to "allow". The
  default policy controls the behavior of a token when there is no matching
  rule. In "allow" mode, ACLs are a blacklist: any operation not specifically
  prohibited is allowed. In "deny" mode, ACLs are a whilelist: any operation not
  specifically allowed is blocked.

* `acl_down_policy` - Either "allow", "deny" or "extend-cache" which is the
  default. In the case that the policy for a token cannot be read from the
  `acl_datacenter` or leader node, the down policy is applied. In "allow" mode,
  all actions are permitted, "deny" restricts all operations, and "extend-cache"
  allows any cached ACLs to be used, ignoring their TTL values. If a non-cached
  ACL is used, "extend-cache" acts like "deny".

* `acl_master_token` - Only used for servers in the `acl_datacenter`. This token
   will be created if it does not exist with management level permissions. It allows
   operators to bootstrap the ACL system with a token ID that is well-known.

* `acl_token` - When provided, the agent will use this token when making requests
   to the Consul servers. Clients can override this token on a per-request basis
   by providing the ?token parameter. When not provided, the empty token is used
   which maps to the 'anonymous' ACL policy.


* `acl_ttl` - Used to control Time-To-Live caching of ACLs. By default this
   is 30 seconds. This setting has a major performance impact: reducing it will
   cause more frequent refreshes, while increasing it reduces the number of caches.
   However, because the caches are not actively invalidated, ACL policy may be stale
   up to the TTL value.

* `addresses` - This is a nested object that allows setting the bind address
  for the following keys:
    * `dns` - The DNS server. Defaults to `client_addr`
    * `http` - The HTTP API. Defaults to `client_addr`
    * `rpc` - The RPC endpoint. Defaults to `client_addr`

* `advertise_addr` - Equivalent to the `-advertise` command-line flag.

* `bootstrap` - Equivalent to the `-bootstrap` command-line flag.

* `bootstrap_expect` - Equivalent to the `-bootstrap-expect` command-line flag.

* `bind_addr` - Equivalent to the `-bind` command-line flag.

* `ca_file` - This provides a the file path to a PEM encoded certificate authority.
  The certificate authority is used to check the authenticity of client and server
  connections with the appropriate `verify_incoming` or `verify_outgoing` flags.

* `cert_file` - This provides a the file path to a PEM encoded certificate.
  The certificate is provided to clients or servers to verify the agents authenticity.
  Must be provided along with the `key_file`.

* `check_update_interval` - This interval controls how often check output from
  checks in a steady state is syncronized with the server. By default, this is
  set to 5 minutes ("5m"). Many checks which are in a steady state produce
  slightly different output per run (timestamps, etc) which cause constant writes.
  This configuration allows defering the sync of check output for a given interval to
  reduce write pressure. If a check ever changes state, the new state and associated
  output is syncronized immediately. To disable this behavior, set the value to "0s".

* `client_addr` - Equivalent to the `-client` command-line flag.

* `datacenter` - Equivalent to the `-dc` command-line flag.

* `data_dir` - Equivalent to the `-data-dir` command-line flag.

* `disable_anonymous_signature` - Disables providing an anonymous signature for
  de-duplication with the update check. See `disable_update_check`.

* `disable_remote_exec` - Disables support for remote execution. When set to true,
  the agent will ignore any incoming remote exec requests.

* `disable_update_check` - Disables automatic checking for security bulletins and
  new version releases.

* `dns_config` - This object allows a number of sub-keys to be set which can tune
  how DNS queries are perfomed. See this guide on [DNS caching](/docs/guides/dns-cache.html).
  The following sub-keys are available:

  * `allow_stale` - Enables a stale query for DNS information. This allows any Consul
  server to service the request, instead of only the leader. The advantage of this is
  you get linear read scalability with Consul servers. By default, this is false, meaning
  all requests are serviced by the leader. This provides stronger consistency but
  with less throughput and higher latency.

  * `max_stale` - When `allow_stale` is specified, this is used to limit how
  stale of a result will be used. By default, this is set to "5s", which means
  if a Consul server is more than 5 seconds behind the leader, the query will be
  re-evaluated on the leader to get more up-to-date results.

  * `node_ttl` - By default, this is "0s", which means all node lookups are served with
  a 0 TTL value. This can be set to allow node lookups to set a TTL value, which enables
  DNS caching. This should be specified with the "s" suffix for second, or "m" for minute.

  * `service_ttl` - This is a sub-object, which allows for setting a TTL on service lookups
  with a per-service policy. The "*" wildcard service can be specified and is used when
  there is no specific policy available for a service. By default, all services are served
  with a 0 TTL value. Setting this enables DNS caching.

  * `enable_truncate` - If set to true, a UDP DNS query that would return more than 3 records
  will set the truncated flag, indicating to clients that they should re-query using TCP to
  get the full set of records.

* `domain` - By default, Consul responds to DNS queries in the "consul." domain.
  This flag can be used to change that domain. All queries in this domain are assumed
  to be handled by Consul, and will not be recursively resolved.

* `enable_debug` - When set, enables some additional debugging features. Currently,
  only used to set the runtime profiling HTTP endpoints.

* `enable_syslog` - Equivalent to the `-syslog` command-line flag.

* `encrypt` - Equivalent to the `-encrypt` command-line flag.

* `key_file` - This provides a the file path to a PEM encoded private key.
  The key is used with the certificate to verify the agents authenticity.
  Must be provided along with the `cert_file`.

* `leave_on_terminate` - If enabled, when the agent receives a TERM signal,
  it will send a Leave message to the rest of the cluster and gracefully
  leave. Defaults to false.

* `log_level` - Equivalent to the `-log-level` command-line flag.

* `node_name` - Equivalent to the `-node` command-line flag.

* `ports` - This is a nested object that allows setting the bind ports
   for the following keys:
    * `dns` - The DNS server, -1 to disable. Default 8600.
    * `http` - The HTTP api, -1 to disable. Default 8500.
    * `rpc` - The RPC endpoint. Default 8400.
    * `serf_lan` - The Serf LAN port. Default 8301.
    * `serf_wan` - The Serf WAN port. Default 8302.
    * `server` - Server RPC address. Default 8300.

* `protocol` - Equivalent to the `-protocol` command-line flag.

* `recursor` - This flag provides an address of an upstream DNS server that is used to
  recursively resolve queries if they are not inside the service domain for consul. For example,
  a node can use Consul directly as a DNS server, and if the record is outside of the "consul." domain,
  the query will be resolved upstream using this server.

* `rejoin_after_leave` - Equivalent to the `-rejoin` command-line flag.

* `server` - Equivalent to the `-server` command-line flag.

* `server_name` - When give, this overrides the `node_name` for the TLS certificate.
  It can be used to ensure that the certificate name matches the hostname we
  declare.

* `skip_leave_on_interrupt` - This is the similar to`leave_on_terminate` but
  only affects interrupt handling. By default, an interrupt causes Consul to
  gracefully leave, but setting this to true disables that. Defaults to false.
  Interrupts are usually from a Control-C from a shell.

* `start_join` - An array of strings specifying addresses of nodes to
  join upon startup.

* `statsd_addr` - This provides the address of a statsd instance.  If provided
  Consul will send various telemetry information to that instance for aggregation.
  This can be used to capture various runtime information. This sends UDP packets
  only, and can be used with statsd or statsite.

* `statsite_addr` - This provides the address of a statsite instance. If provided
  Consul will stream various telemetry information to that instance for aggregation.
  This can be used to capture various runtime information. This streams via
  TCP and can only be used with statsite.

* `syslog_facility` - When `enable_syslog` is provided, this controls which
  facility messages are sent to. By default, `LOCAL0` will be used.

* `ui_dir` - Equivalent to the `-ui-dir` command-line flag.

* `verify_incoming` - If set to True, Consul requires that all incoming
  connections make use of TLS, and that the client provides a certificate signed
  by the Certificate Authority from the `ca_file`. By default, this is false, and
  Consul will not enforce the use of TLS or verify a client's authenticity. This
  only applies to Consul servers, since a client never has an incoming connection.

* `verify_outgoing` - If set to True, Consul requires that all outgoing connections
  make use of TLS, and that the server provide a certificate that is signed by
  the Certificate Authority from the `ca_file`. By default, this is false, and Consul
  will not make use of TLS for outgoing connections. This applies to clients and servers,
  as both will make outgoing connections.

* `watches` - Watches is a list of watch specifications.
   These allow an external process to be automatically invoked when a particular
   data view is updated. See the [watch documentation](/docs/agent/watches.html) for
   more documentation. Watches can be modified when the configuration is reloaded.

## Ports Used

Consul requires up to 5 different ports to work properly, some requiring
TCP, UDP, or both protocols. Below we document the requirements for each
port.

* Server RPC (Default 8300). This is used by servers to handle incoming
  requests from other agents. TCP only.

* Serf LAN (Default 8301). This is used to handle gossip in the LAN.
  Required by all agents, TCP and UDP.

* Serf WAN( Default 8302). This is used by servers to gossip over the
  WAN to other servers. TCP and UDP.

* CLI RPC (Default 8400). This is used by all agents to handle RPC
  from the CLI. TCP only.

* HTTP API (Default 8500). This is used by clients to talk to the HTTP
  API. TCP only.

* DNS Interface (Default 8600). Used to resolve DNS queries. TCP and UDP.


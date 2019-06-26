---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-server"
description: |-
  The se options configure the servers.
---

# Server

These options configure the servers. See also the datacenter options.

* <a name="server"></a><a href="#server">`server`</a> Equivalent to the
  [`-server` command-line flag](#_server).

* <a name="_dev"></a><a href="#_dev">`-dev`</a> - Enable development server
  mode. This is useful for quickly starting a Consul agent with all persistence
  options turned off, enabling an in-memory server which can be used for rapid
  prototyping or developing against the API. In this mode, [Connect is
  enabled](/docs/connect/configuration.html) and will by default create a new
  root CA certificate on startup. This mode is **not** intended for production
  use as it does not write any data to disk. The gRPC port is also defaulted to
  `8502` in this mode.

* <a name="advertise_addr_wan"></a><a href="#_advertise-wan">`advertise_addr_wan`</a> - The command line flag is, `-advertise-wan`. The
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
  template.

*   <a name="autopilot"></a><a href="#autopilot">`autopilot`</a> Added in Consul 0.8, this object
    allows a number of sub-keys to be set which can configure operator-friendly settings for Consul servers.
    For more information about Autopilot, see the [Autopilot Guide](https://learn.hashicorp.com/consul/day-2-operations/autopilot).

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

* <a name="bootstrap"></a><a href="#bootstrap">`bootstrap`</a> Equivalent to the `-bootstrap` command-line flag. This option is used to control if a
  server is in "bootstrap" mode. It is important that
  no more than one server *per* datacenter be running in this mode. Technically, a server in bootstrap mode
  is allowed to self-elect as the Raft leader. It is important that only a single node is in this mode;
  otherwise, consistency cannot be guaranteed as multiple nodes are able to self-elect.
  It is not recommended to use this flag after a cluster has been bootstrapped.

* <a name="bootstrap_expect"></a><a href="#bootstrap_expect">`bootstrap_expect`</a> Equivalent
  to the `-bootstrap-expect` command-line flag. This options provides the number of expected servers in the datacenter.
  Either this value should not be provided or the value must agree with other servers in
  the cluster. When provided, Consul waits until the specified number of servers are
  available and then bootstraps the cluster. This allows an initial leader to be elected
  automatically. This cannot be used in conjunction with the legacy `-bootstrap` flag.
  This flag requires `-server`mode.

*   <a name="performance"></a><a href="#performance">`performance`</a> Available in Consul 0.7 and
    later, this is a nested object that allows tuning the performance of different subsystems in
    Consul. See the [Server Performance](/docs/install/performance.html) documentation for more details. The
    following parameters are available:

    *   <a name="leave_drain_time"></a><a href="#leave_drain_time">`leave_drain_time`</a> - A duration
        that a server will dwell during a graceful leave in order to allow requests to be retried against
        other Consul servers. Under normal circumstances, this can prevent clients from experiencing
        "no leader" errors when performing a rolling update of the Consul servers. This was added in
        Consul 1.0. Must be a duration value such as 10s. Defaults to 5s.

    *   <a name="raft_multiplier"></a><a href="#raft_multiplier">`raft_multiplier`</a> - An integer
        multiplier used by Consul servers to scale key Raft timing parameters. Omitting this value
        or setting it to 0 uses default timing described below. Lower values are used to tighten
        timing and increase sensitivity while higher values relax timings and reduce sensitivity.
        Tuning this affects the time it takes Consul to detect leader failures and to perform
        leader elections, at the expense of requiring more network and CPU resources for better
        performance.

        By default, Consul will use a lower-performance timing that's suitable
        for [minimal Consul servers](/docs/install/performance.html#minimum), currently equivalent
        to setting this to a value of 5 (this default may be changed in future versions of Consul,
        depending if the target minimum server profile changes). Setting this to a value of 1 will
        configure Raft to its highest-performance mode, equivalent to the default timing of Consul
        prior to 0.7, and is recommended for [production Consul servers](/docs/install/performance.html#production).
        See the note on [last contact](/docs/install/performance.html#last-contact) timing for more
        details on tuning this parameter. The maximum allowed value is 10.

    *   <a name="rpc_hold_timeout"></a><a href="#rpc_hold_timeout">`rpc_hold_timeout`</a> - A duration
        that a client or server will retry internal RPC requests during leader elections. Under normal
        circumstances, this can prevent clients from experiencing "no leader" errors. This was added in
        Consul 1.0. Must be a duration value such as 10s. Defaults to 7s.

* <a name="raft_protocol"></a><a href="#raft_protocol">`raft_protocol`</a> Equivalent to the
  [`-raft-protocol` command-line flag](#_raft_protocol).

* <a name="raft_snapshot_threshold"></a><a href="#raft_snapshot_threshold">`raft_snapshot_threshold`</a> Equivalent to the
  [`-raft-snapshot-threshold` command-line flag](#_raft_snapshot_threshold).

* <a name="raft_snapshot_interval"></a><a href="#raft_snapshot_interval">`raft_snapshot_interval`</a> Equivalent to the
  [`-raft-snapshot-interval` command-line flag](#_raft_snapshot_interval).

* <a name="retry_join_wan"></a><a href="#retry_join_wan">`retry_join_wan`</a> Equivalent to the
  [`-retry-join-wan` command-line flag](#_retry_join_wan). Takes a list
  of addresses to attempt joining to WAN every [`retry_interval_wan`](#_retry_interval_wan) until at least one
  join works.

* <a name="retry_interval_wan"></a><a href="#retry_interval_wan">`retry_interval_wan`</a> Equivalent to the
  [`-retry-interval-wan` command-line flag](#_retry_interval_wan).

* <a name="serf_wan"></a><a href="#serf_wan">`serf_wan`</a> Server only. Equivalent to
  the `-serf-wan-bind` command-line flag. The address that should be bound to for Serf WAN gossip communications. By
  default, the value follows the same rules as [`-bind` command-line
  flag](#_bind), and if this is not specified, the `-bind` option is used. This
  is available in Consul 0.7.1 and later. In Consul 1.0 and later this can be
  set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

* <a name="server_name"></a><a href="#server_name">`server_name`</a> When provided, this overrides
  the [`node_name`](#_node) for the TLS certificate. It can be used to ensure that the certificate
  name matches the hostname we declare.

  * <a name="start_join_wan"></a><a href="#start_join_wan">`start_join_wan`</a> An array of strings specifying
  addresses of WAN nodes to [`-join-wan`](#_join_wan) upon startup.

*   <a name="translate_wan_addrs"></a><a href="#translate_wan_addrs">`translate_wan_addrs`</a> If
    set to true, Consul will prefer a node's configured <a href="#_advertise-wan">WAN address</a>
    when servicing DNS and HTTP requests for a node in a remote datacenter. This allows the node to
    be reached within its own datacenter using its local address, and reached from other datacenters
    using its WAN address, which is useful in hybrid setups with mixed networks. This is disabled by
    default.

    Starting in Consul 0.7 and later, node addresses in responses to HTTP requests will also prefer a
    node's configured <a href="#_advertise-wan">WAN address</a> when querying for a node in a remote
    datacenter. An [`X-Consul-Translate-Addresses`](/api/index.html#translated-addresses) header
    will be present on all responses when translation is enabled to help clients know that the addresses
    may be translated. The `TaggedAddresses` field in responses also have a `lan` address for clients that
    need knowledge of that address, regardless of translation.

    The following endpoints translate addresses:
    - [`/v1/catalog/nodes`](/api/catalog.html#catalog_nodes)
    - [`/v1/catalog/node/<node>`](/api/catalog.html#catalog_node)
    - [`/v1/catalog/service/<service>`](/api/catalog.html#catalog_service)
    - [`/v1/health/service/<service>`](/api/health.html#health_service)
    - [`/v1/query/<query or name>/execute`](/api/query.html#execute)

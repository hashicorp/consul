---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-server"
description: |-
  The 
---

# Server

* <a name="server"></a><a href="#server">`server`</a> Equivalent to the
  [`-server` command-line flag](#_server).

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

* <a name="bootstrap"></a><a href="#bootstrap">`bootstrap`</a> Equivalent to the
  [`-bootstrap` command-line flag](#_bootstrap).

* <a name="bootstrap_expect"></a><a href="#bootstrap_expect">`bootstrap_expect`</a> Equivalent
  to the [`-bootstrap-expect` command-line flag](#_bootstrap_expect).

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
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
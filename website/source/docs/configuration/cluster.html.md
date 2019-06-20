---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-cluster"
description: |-
  The 
---

* <a name="datacenter"></a><a href="#datacenter">`datacenter`</a> Equivalent to the
  [`-datacenter` command-line flag](#_datacenter).

* <a name="data_dir"></a><a href="#data_dir">`data_dir`</a> Equivalent to the
  [`-data-dir` command-line flag](#_data_dir).

* <a name="disable_anonymous_signature"></a><a href="#disable_anonymous_signature">
  `disable_anonymous_signature`</a> Disables providing an anonymous signature for de-duplication
  with the update check. See [`disable_update_check`](#disable_update_check).

* <a name="disable_host_node_id"></a><a href="#disable_host_node_id">`disable_host_node_id`</a>
  Equivalent to the [`-disable-host-node-id` command-line flag](#_disable_host_node_id).

* <a name="disable_http_unprintable_char_filter"></a><a href="#disable_http_unprintable_char_filter">`disable_http_unprintable_char_filter`</a>
  Defaults to false. Consul 1.0.3 fixed a potential security vulnerability where
  malicious users could craft KV keys with unprintable chars that would confuse
  operators using the CLI or UI into taking wrong actions. Users who had data
  written in older versions of Consul that did not have this restriction will be
  unable to delete those values by default in 1.0.3 or later. This setting
  enables those users to _temporarily_ disable the filter such that delete
  operations can work on those keys again to get back to a healthy state. It is
  strongly recommended that this filter is not disabled permanently as it
  exposes the original security vulnerability.

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

* <a name="domain"></a><a href="#domain">`domain`</a> Equivalent to the
  [`-domain` command-line flag](#_domain).

  * <a name="enable_debug"></a><a href="#enable_debug">`enable_debug`</a> When set, enables some
  additional debugging features. Currently, this is only used to access runtime profiling HTTP endpoints, which
  are available with an `operator:read` ACL regardless of the value of `enable_debug`.

* <a name="enable_script_checks"></a><a href="#enable_script_checks">`enable_script_checks`</a> Equivalent to the
  [`-enable-script-checks` command-line flag](#_enable_script_checks).

    ~> **Security Warning:** Enabling script checks in some configurations may
  introduce a remote execution vulnerability which is known to be targeted by
  malware. We strongly recommend `enable_local_script_checks` instead. See [this
  blog post](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations)
  for more details.

* <a name="enable_local_script_checks"></a><a href="#enable_local_script_checks">`enable_local_script_checks`</a> Equivalent to the
  [`-enable-local-script-checks` command-line flag](#_enable_local_script_checks).

* <a name="enable_syslog"></a><a href="#enable_syslog">`enable_syslog`</a> Equivalent to
  the [`-syslog` command-line flag](#_syslog).

*   <a name="http_config"></a><a href="#http_config">`http_config`</a>
    This object allows setting options for the HTTP API.

    The following sub-keys are available:

    * <a name="block_endpoints"></a><a href="#block_endpoints">`block_endpoints`</a>
      This object is a list of HTTP API endpoint prefixes to block on the agent, and defaults to
      an empty list, meaning all endpoints are enabled. Any endpoint that has a common prefix
      with one of the entries on this list will be blocked and will return a 403 response code
      when accessed. For example, to block all of the V1 ACL endpoints, set this to
      `["/v1/acl"]`, which will block `/v1/acl/create`, `/v1/acl/update`, and the other ACL
      endpoints that begin with `/v1/acl`. This only works with API endpoints, not `/ui` or
      `/debug`, those must be disabled with their respective configuration options. Any CLI
      commands that use disabled endpoints will no longer function as well. For more general
      access control, Consul's [ACL system](https://learn.hashicorp.com/consul/security-networking/production-acls) should be used, but this option
      is useful for removing access to HTTP API endpoints completely, or on specific agents. This
      is available in Consul 0.9.0 and later.

    * <a name="response_headers"></a><a href="#response_headers">`response_headers`</a>
      This object allows adding headers to the HTTP API responses.
      For example, the following config can be used to enable
      [CORS](https://en.wikipedia.org/wiki/Cross-origin_resource_sharing) on
      the HTTP API endpoints:

          ```javascript
            {
              "http_config": {
                "response_headers": {
                  "Access-Control-Allow-Origin": "*"
                }
              }
            }
          ```
    * <a name="allow_write_http_from"></a><a href="#allow_write_http_from">`allow_write_http_from`</a>
      This object is a list of networks in CIDR notation (eg "127.0.0.0/8") that are allowed
      to call the agent write endpoints. It defaults to an empty list, which means all networks
      are allowed.
      This is used to make the agent read-only, except for select ip ranges.
      * To block write calls from anywhere, use `[ "255.255.255.255/32" ]`.
      * To only allow write calls from localhost, use `[ "127.0.0.0/8" ]`
      * To only allow specific IPs, use `[ "10.0.0.1/32", "10.0.0.2/32" ]`

* <a name="leave_on_terminate"></a><a href="#leave_on_terminate">`leave_on_terminate`</a> If
  enabled, when the agent receives a TERM signal, it will send a `Leave` message to the rest
  of the cluster and gracefully leave. The default behavior for this feature varies based on
  whether or not the agent is running as a client or a server (prior to Consul 0.7 the default
  value was unconditionally set to `false`). On agents in client-mode, this defaults to `true`
  and for agents in server-mode, this defaults to `false`.

* <a name="limits"></a><a href="#limits">`limits`</a> Available in Consul 0.9.3 and later, this
  is a nested object that configures limits that are enforced by the agent. Currently, this only
  applies to agents in client mode, not Consul servers. The following parameters are available:

    *   <a name="rpc_rate"></a><a href="#rpc_rate">`rpc_rate`</a> - Configures the RPC rate
        limiter by setting the maximum request rate that this agent is allowed to make for RPC
        requests to Consul servers, in requests per second. Defaults to infinite, which disables
        rate limiting.
    *   <a name="rpc_rate"></a><a href="#rpc_max_burst">`rpc_max_burst`</a> - The size of the token
        bucket used to recharge the RPC rate limiter. Defaults to 1000 tokens, and each token is
        good for a single RPC call to a Consul server. See https://en.wikipedia.org/wiki/Token_bucket
        for more details about how token bucket rate limiters operate.

* <a name="log_file"></a><a href="#log_file">`log_file`</a> Equivalent to the
  [`-log-file` command-line flag](#_log_file).

* <a name="log_level"></a><a href="#log_level">`log_level`</a> Equivalent to the
  [`-log-level` command-line flag](#_log_level).

* <a name="node_id"></a><a href="#node_id">`node_id`</a> Equivalent to the
  [`-node-id` command-line flag](#_node_id).

* <a name="node_name"></a><a href="#node_name">`node_name`</a> Equivalent to the
  [`-node` command-line flag](#_node).

* <a name="node_meta"></a><a href="#node_meta">`node_meta`</a> Available in Consul 0.7.3 and later,
  This object allows associating arbitrary metadata key/value pairs with the local node, which can
  then be used for filtering results from certain catalog endpoints. See the
  [`-node-meta` command-line flag](#_node_meta) for more information.

    ```javascript
      {
        "node_meta": {
            "instance_type": "t2.medium"
        }
      }
    ```
* <a name="protocol"></a><a href="#protocol">`protocol`</a> Equivalent to the
  [`-protocol` command-line flag](#_protocol).

* <a name="reap"></a><a href="#reap">`reap`</a> This controls Consul's automatic reaping of child processes,
  which is useful if Consul is running as PID 1 in a Docker container. If this isn't specified, then Consul will
  automatically reap child processes if it detects it is running as PID 1. If this is set to true or false, then
  it controls reaping regardless of Consul's PID (forces reaping on or off, respectively). This option was removed
  in Consul 0.7.1. For later versions of Consul, you will need to reap processes using a wrapper, please see the
  [Consul Docker image entry point script](https://github.com/hashicorp/docker-consul/blob/master/0.X/docker-entrypoint.sh)
  for an example. If you are using Docker 1.13.0 or later, you can use the new `--init` option of the `docker run` command
  and docker will enable an init process with PID 1 that reaps child processes for the container.
  More info on [Docker docs](https://docs.docker.com/engine/reference/commandline/run/#options).

* <a name="reconnect_timeout"></a><a href="#reconnect_timeout">`reconnect_timeout`</a> This controls
  how long it takes for a failed node to be completely removed from the cluster. This defaults to
  72 hours and it is recommended that this is set to at least double the maximum expected recoverable
  outage time for a node or network partition. WARNING: Setting this time too low could cause Consul
  servers to be removed from quorum during an extended node failure or partition, which could complicate
  recovery of the cluster. The value is a time with a unit suffix, which can be "s", "m", "h" for seconds,
  minutes, or hours. The value must be >= 8 hours.

* <a name="rejoin_after_leave"></a><a href="#rejoin_after_leave">`rejoin_after_leave`</a> Equivalent
  to the [`-rejoin` command-line flag](#_rejoin).

* `retry_join` - Equivalent to the [`-retry-join`](#retry-join) command-line flag.

* <a name="retry_interval"></a><a href="#retry_interval">`retry_interval`</a> Equivalent to the
  [`-retry-interval` command-line flag](#_retry_interval).

* <a name="skip_leave_on_interrupt"></a><a
  href="#skip_leave_on_interrupt">`skip_leave_on_interrupt`</a> This is
  similar to [`leave_on_terminate`](#leave_on_terminate) but only affects
  interrupt handling. When Consul receives an interrupt signal (such as
  hitting Control-C in a terminal), Consul will gracefully leave the cluster.
  Setting this to `true` disables that behavior. The default behavior for
  this feature varies based on whether or not the agent is running as a
  client or a server (prior to Consul 0.7 the default value was
  unconditionally set to `false`). On agents in client-mode, this defaults
  to `false` and for agents in server-mode, this defaults to `true`
  (i.e. Ctrl-C on a server will keep the server in the cluster and therefore
  quorum, and Ctrl-C on a client will gracefully leave).

* <a name="start_join"></a><a href="#start_join">`start_join`</a> An array of strings specifying addresses
  of nodes to [`-join`](#_join) upon startup. Note that using
  <a href="#retry_join">`retry_join`</a> could be more appropriate to help
  mitigate node startup race conditions when automating a Consul cluster
  deployment.

* <a name="syslog_facility"></a><a href="#syslog_facility">`syslog_facility`</a> When
  [`enable_syslog`](#enable_syslog) is provided, this controls to which
  facility messages are sent. By default, `LOCAL0` will be used.

  * <a name="ui"></a><a href="#ui">`ui`</a> - Equivalent to the [`-ui`](#_ui)
  command-line flag.

* <a name="ui_dir"></a><a href="#ui_dir">`ui_dir`</a> - Equivalent to the
  [`-ui-dir`](#_ui_dir) command-line flag. This configuration key is not required as of Consul version 0.7.0 and later. Specifying this configuration key will enable the web UI. There is no need to specify both ui-dir and ui. Specifying both will result in an error.

*   <a name="unix_sockets"></a><a href="#unix_sockets">`unix_sockets`</a> - This
    allows tuning the ownership and permissions of the
    Unix domain socket files created by Consul. Domain sockets are only used if
    the HTTP address is configured with the `unix://` prefix.

    It is important to note that this option may have different effects on
    different operating systems. Linux generally observes socket file permissions
    while many BSD variants ignore permissions on the socket file itself. It is
    important to test this feature on your specific distribution. This feature is
    currently not functional on Windows hosts.

    The following options are valid within this construct and apply globally to all
    sockets created by Consul:
    - `user` - The name or ID of the user who will own the socket file.
    - `group` - The group ID ownership of the socket file. This option
      currently only supports numeric IDs.
    - `mode` - The permission bits to set on the file.
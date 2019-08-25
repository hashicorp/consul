---
layout: "docs"
page_title: "Check Definition"
sidebar_current: "docs-agent-checks"
description: |-
  One of the primary roles of the agent is management of system- and application-level health checks. A health check is considered to be application-level if it is associated with a service. A check is defined in a configuration file or added at runtime over the HTTP interface.
---

# Checks

One of the primary roles of the agent is management of system-level and application-level health
checks. A health check is considered to be application-level if it is associated with a
service. If not associated with a service, the check monitors the health of the entire node.

A check is defined in a configuration file or added at runtime over the HTTP interface. Checks
created via the HTTP interface persist with that node.

There are several different kinds of checks:

* Script + Interval - These checks depend on invoking an external application
  that performs the health check, exits with an appropriate exit code, and potentially
  generates some output. A script is paired with an invocation interval (e.g.
  every 30 seconds). This is similar to the Nagios plugin system. The output of
  a script check is limited to 4KB. Output larger than this will be truncated.
  By default, Script checks will be configured with a timeout equal to 30 seconds.
  It is possible to configure a custom Script check timeout value by specifying the
  `timeout` field in the check definition. When the timeout is reached on Windows,
  Consul will wait for any child processes spawned by the script to finish. For any
  other system, Consul will attempt to force-kill the script and any child processes
  it has spawned once the timeout has passed.
  In Consul 0.9.0 and later, script checks are not enabled by default. To use them you
  can either use :
  * [`enable_local_script_checks`](/docs/agent/options.html#_enable_local_script_checks):
    enable script checks defined in local config files. Script checks defined via the HTTP
    API will not be allowed.
  * [`enable_script_checks`](/docs/agent/options.html#_enable_script_checks): enable
    script checks regardless of how they are defined.

  ~> **Security Warning:** Enabling script checks in some configurations may
  introduce a remote execution vulnerability which is known to be targeted by
  malware. We strongly recommend `enable_local_script_checks` instead. See [this
  blog post](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations)
  for more details.

* HTTP + Interval - These checks make an HTTP `GET` request to the specified URL,
  waiting the specified `interval` amount of time between requests (eg. 30 seconds).
  The status of the service depends on the HTTP response code: any `2xx` code is 
  considered passing, a `429 Too ManyRequests` is a warning, and anything else is
  a failure. This type of check
  should be preferred over a script that uses `curl` or another external process
  to check a simple HTTP operation. By default, HTTP checks are `GET` requests
  unless the `method` field specifies a different method. Additional header
  fields can be set through the `header` field which is a map of lists of
  strings, e.g. `{"x-foo": ["bar", "baz"]}`. By default, HTTP checks will be
  configured with a request timeout equal to 10 seconds.
  It is possible to configure a custom HTTP check timeout value by
  specifying the `timeout` field in the check definition. The output of the
  check is limited to roughly 4KB. Responses larger than this will be truncated.
  HTTP checks also support TLS. By default, a valid TLS certificate is expected.
  Certificate verification can be turned off by setting the `tls_skip_verify`
  field to `true` in the check definition.

* TCP + Interval - These checks make a TCP connection attempt to the specified 
  IP/hostname and port, waiting `interval` amount of time between attempts 
  (e.g. 30 seconds). If no hostname
  is specified, it defaults to "localhost". The status of the service depends on
  whether the connection attempt is successful (ie - the port is currently
  accepting connections). If the connection is accepted, the status is
  `success`, otherwise the status is `critical`. In the case of a hostname that
  resolves to both IPv4 and IPv6 addresses, an attempt will be made to both
  addresses, and the first successful connection attempt will result in a
  successful check. This type of check should be preferred over a script that
  uses `netcat` or another external process to check a simple socket operation.
  By default, TCP checks will be configured with a request timeout of 10 seconds. 
  It is possible to configure a custom TCP check timeout value by specifying the 
  `timeout` field in the check definition.

* <a name="TTL"></a>Time to Live (TTL) - These checks retain their last known
  state for a given TTL.  The state of the check must be updated periodically
  over the HTTP interface. If an external system fails to update the status
  within a given TTL, the check is set to the failed state. This mechanism,
  conceptually similar to a dead man's switch, relies on the application to
  directly report its health. For example, a healthy app can periodically `PUT` a
  status update to the HTTP endpoint; if the app fails, the TTL will expire and
  the health check enters a critical state. The endpoints used to update health
  information for a given check are:
  [pass](/api/agent/check.html#ttl-check-pass),
  [warn](/api/agent/check.html#ttl-check-warn),
  [fail](/api/agent/check.html#ttl-check-fail), and
  [update](/api/agent/check.html#ttl-check-update).  TTL
  checks also persist their last known status to disk. This allows the Consul
  agent to restore the last known status of the check across restarts.  Persisted
  check status is valid through the end of the TTL from the time of the last
  check.

* Docker + Interval - These checks depend on invoking an external application which
  is packaged within a Docker Container. The application is triggered within the running
  container via the Docker Exec API. We expect that the Consul agent user has access
  to either the Docker HTTP API or the unix socket. Consul uses ```$DOCKER_HOST``` to
  determine the Docker API endpoint. The application is expected to run, perform a health
  check of the service running inside the container, and exit with an appropriate exit code.
  The check should be paired with an invocation interval. The shell on which the check
  has to be performed is configurable which makes it possible to run containers which
  have different shells on the same host. Check output for Docker is limited to
  4KB. Any output larger than this will be truncated. In Consul 0.9.0 and later, the agent
  must be configured with [`enable_script_checks`](/docs/agent/options.html#_enable_script_checks)
  set to `true` in order to enable Docker health checks.

* gRPC + Interval - These checks are intended for applications that support the standard
  [gRPC health checking protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md).
  The state of the check will be updated by probing the configured endpoint, waiting `interval`
  amount of time between probes (eg. 30 seconds). By default, gRPC checks will be configured 
  with a default timeout of 10 seconds.
  It is possible to configure a custom timeout value by specifying the `timeout` field in
  the check definition. gRPC checks will default to not using TLS, but TLS can be enabled by
  setting `grpc_use_tls` in the check definition. If TLS is enabled, then by default, a valid
  TLS certificate is expected. Certificate verification can be turned off by setting the
  `tls_skip_verify` field to `true` in the check definition.

* <a name="alias"></a>Alias - These checks alias the health state of another registered
  node or service. The state of the check will be updated asynchronously,
  but is nearly instant. For aliased services on the same agent, the local
  state is monitored and no additional network resources are consumed. For
  other services and nodes, the check maintains a blocking query over the
  agent's connection with a current server and allows stale requests. If there
  are any errors in watching the aliased node or service, the check state will be
  critical. For the blocking query, the check will use the ACL token set
  on the service or check definition or otherwise will fall back to the default ACL
  token set with the agent (`acl_token`).

## Check Definition

A script check:

```javascript
{
  "check": {
    "id": "mem-util",
    "name": "Memory utilization",
    "args": ["/usr/local/bin/check_mem.py", "-limit", "256MB"],
    "interval": "10s",
    "timeout": "1s"
  }
}
```

A HTTP check:

```javascript
{
  "check": {
    "id": "api",
    "name": "HTTP API on port 5000",
    "http": "https://localhost:5000/health",
    "tls_skip_verify": false,
    "method": "POST",
    "header": {"x-foo":["bar", "baz"]},
    "interval": "10s",
    "timeout": "1s"
  }
}
```

A TCP check:

```javascript
{
  "check": {
    "id": "ssh",
    "name": "SSH TCP on port 22",
    "tcp": "localhost:22",
    "interval": "10s",
    "timeout": "1s"
  }
}
```

A TTL check:

```javascript
{
  "check": {
    "id": "web-app",
    "name": "Web App Status",
    "notes": "Web app does a curl internally every 10 seconds",
    "ttl": "30s"
  }
}
```

A Docker check:

```javascript
{
  "check": {
    "id": "mem-util",
    "name": "Memory utilization",
    "docker_container_id": "f972c95ebf0e",
    "shell": "/bin/bash",
    "args": ["/usr/local/bin/check_mem.py"],
    "interval": "10s"
  }
}
```

A gRPC check:

```javascript
{
  "check": {
    "id": "mem-util",
    "name": "Service health status",
    "grpc": "127.0.0.1:12345",
    "grpc_use_tls": true,
    "interval": "10s"
  }
}
```

An alias check for a local service:

```javascript
{
  "check": {
    "id": "web-alias",
    "alias_service": "web"
  }
}
```

Each type of definition must include a `name` and may optionally provide an
`id` and `notes` field. The `id` must be unique per _agent_ otherwise only the
last defined check with that `id` will be registered. If the `id` is not set
and the check is embedded within a service definition a unique check id is
generated. Otherwise, `id` will be set to `name`. If names might conflict,
unique IDs should be provided.

The `notes` field is opaque to Consul but can be used to provide a human-readable
description of the current state of the check. Similarly, an external process
updating a TTL check via the HTTP interface can set the `notes` value.

Checks may also contain a `token` field to provide an ACL token. This token is
used for any interaction with the catalog for the check, including
[anti-entropy syncs](/docs/internals/anti-entropy.html) and deregistration.
For Alias checks, this token is used if a remote blocking query is necessary
to watch the state of the aliased node or service.

Script, TCP, HTTP, Docker, and gRPC checks must include an `interval` field. This
field is parsed by Go's `time` package, and has the following
[formatting specification](https://golang.org/pkg/time/#ParseDuration):
> A duration string is a possibly signed sequence of decimal numbers, each with
> optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m".
> Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

In Consul 0.7 and later, checks that are associated with a service may also contain
an optional `deregister_critical_service_after` field, which is a timeout in the
same Go time format as `interval` and `ttl`. If a check is in the critical state
for more than this configured value, then its associated service (and all of its
associated checks) will automatically be deregistered. The minimum timeout is 1
minute, and the process that reaps critical services runs every 30 seconds, so it
may take slightly longer than the configured timeout to trigger the deregistration.
This should generally be configured with a timeout that's much, much longer than
any expected recoverable outage for the given service.

To configure a check, either provide it as a `-config-file` option to the
agent or place it inside the `-config-dir` of the agent. The file must
end in a ".json" or ".hcl" extension to be loaded by Consul. Check definitions
can also be updated by sending a `SIGHUP` to the agent. Alternatively, the
check can be registered dynamically using the [HTTP API](/api/index.html).

## Check Scripts

A check script is generally free to do anything to determine the status
of the check. The only limitations placed are that the exit codes must obey
this convention:

 * Exit code 0 - Check is passing
 * Exit code 1 - Check is warning
 * Any other code - Check is failing

This is the only convention that Consul depends on. Any output of the script
will be captured and stored in the `output` field.

In Consul 0.9.0 and later, the agent must be configured with
[`enable_script_checks`](/docs/agent/options.html#_enable_script_checks) set to `true`
in order to enable script checks.

## Initial Health Check Status

By default, when checks are registered against a Consul agent, the state is set
immediately to "critical". This is useful to prevent services from being
registered as "passing" and entering the service pool before they are confirmed
to be healthy. In certain cases, it may be desirable to specify the initial
state of a health check. This can be done by specifying the `status` field in a
health check definition, like so:

```javascript
{
  "check": {
    "id": "mem",
    "args": ["/bin/check_mem", "-limit", "256MB"],
    "interval": "10s",
    "status": "passing"
  }
}
```

The above service definition would cause the new "mem" check to be
registered with its initial state set to "passing".

## Service-bound checks

Health checks may optionally be bound to a specific service. This ensures
that the status of the health check will only affect the health status of the
given service instead of the entire node. Service-bound health checks may be
provided by adding a `service_id` field to a check configuration:

```javascript
{
  "check": {
    "id": "web-app",
    "name": "Web App Status",
    "service_id": "web-app",
    "ttl": "30s"
  }
}
```

In the above configuration, if the web-app health check begins failing, it will
only affect the availability of the web-app service. All other services
provided by the node will remain unchanged.

## Agent Certificates for TLS Checks

The [enable_agent_tls_for_checks](/docs/agent/options.html#enable_agent_tls_for_checks)
agent configuration option can be utilized to have HTTP or gRPC health checks
to use the agent's credentials when configured for TLS.

## Multiple Check Definitions

Multiple check definitions can be defined using the `checks` (plural)
key in your configuration file.

```javascript
{
  "checks": [
    {
      "id": "chk1",
      "name": "mem",
      "args": ["/bin/check_mem", "-limit", "256MB"],
      "interval": "5s"
    },
    {
      "id": "chk2",
      "name": "/health",
      "http": "http://localhost:5000/health",
      "interval": "15s"
    },
    {
      "id": "chk3",
      "name": "cpu",
      "args": ["/bin/check_cpu"],
      "interval": "10s"
    },
    ...
  ]
}
```

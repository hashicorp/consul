---
layout: "docs"
page_title: "Connect - Proxies"
sidebar_current: "docs-connect-proxies"
description: |-
  A Connect-aware proxy enables unmodified applications to use Connect. A per-service proxy sidecar transparently handles inbound and outbound service connections, automatically wrapping and verifying TLS connections.
---

# Connect Proxies

A Connect-aware proxy enables unmodified applications to use Connect.
A per-service proxy sidecar transparently handles inbound and outbound
service connections, automatically wrapping and verifying TLS connections.

When a proxy is used, the actual service being proxied should only accept
connections on a loopback address. This requires all external connections
to be established via the Connect protocol to provide authentication and
authorization.

Consul supports both _managed_ and _unmanaged_ proxies. A managed proxy
is started, configured, and stopped by Consul. An unmanaged proxy is the
responsibility of the user, like any other Consul service.

## Managed Proxies

Managed proxies are started, configured, and stopped by Consul. They are
enabled via basic configurations within the
[service definition](/docs/agent/services.html).
This is the easiest way to start a proxy and allows Consul users to begin
using Connect with only a small configuration change.

Managed proxies also offer the best security. Managed proxies are given
a unique proxy-specific ACL token that allows read-only access to Connect
information for the specific service the proxy is representing. This ACL
token is more restrictive than can be currently expressed manually in
an ACL policy.

The default managed proxy is a basic proxy built-in to Consul and written
in Go. Having a basic built-in proxy allows Consul to have a sane default
with performance that is good enough for most workloads. In some basic
benchmarks, the service-to-service communication over the built-in proxy
could sustain 5 Gbps with sub-millisecond latency. Therefore,
the performance impact of even the basic built-in proxy is minimal.

Consul will be integrating with advanced proxies in the near future to support
more complex configurations and higher performance. The configuration below is
all for the built-in proxy.

-> **Security note:** 1.) Managed proxies can only be configured
via agent configuration files. They _cannot_ be registered via the HTTP API.
And 2.) Managed proxies are not started at all if Consul is running as root.
Both of these default configurations help prevent arbitrary process
execution or privilege escalation. This behavior can be configured
[per-agent](/docs/agent/options.html#connect_proxy).

### Lifecycle

The Consul agent starts managed proxies on demand and supervises them,
restarting them if they crash. The lifecycle of the proxy process is decoupled
from the agent so if the agent crashes or is restarted for an upgrade, the
managed proxy instances will _not_ be stopped.

Note that this behaviour while desirable in production might leave proxy
processes running indefinitely if you manually stop the agent and clear it's
data dir during testing.

To terminate a managed proxy cleanly you need to deregister the service that
requested it. If the agent is already stopped and will not be restarted again,
you may choose to locate the proxy processes and kill them manually.

While in `-dev` mode, unless a `-data-dir` is explicitly set, managed proxies
switch to being killed when the agent exits since it can't store state in order
to re-adopt them on restart.

### Minimal Configuration

Managed proxies are configured within a
[service definition](/docs/agent/services.html). The simplest possible
managed proxy configuration is an empty configuration. This enables the
default managed proxy and starts a listener for that service:

```json
{
  "service": {
    "name": "redis",
    "port": 6379,
    "connect": { "proxy": {} }
  }
}
```

The listener is started on random port within the configured Connect
port range. It can be discovered using the
[DNS interface](/docs/agent/dns.html#connect-capable-service-lookups)
or
[Catalog API](#).
In most cases, service-to-service communication is established by
a proxy configured with upstreams (described below), which handle the
discovery transparently.

### Upstream Configuration

To transparently discover and establish Connect-based connections to
dependencies, they must be configured with a static port on the managed
proxy configuration:

```json
{
  "service": {
    "name": "web",
    "port": 8080,
    "connect": {
      "proxy": {
        "config": {
          "upstreams": [{
            "destination_name": "redis",
            "local_bind_port": 1234
          }]
        }
      }
    }
  }
}
```

In the example above,
"redis" is configured as an upstream with static port 1234 for service "web".
When a TCP connection is established on port 1234, the proxy
will find Connect-compatible "redis" services via Consul service discovery
and establish a TLS connection identifying as "web".

~> **Security:** Any application that can communicate to the configured
static port will be able to masquerade as the source service ("web" in the
example above). You must either trust any loopback access on that port or
use namespacing techniques provided by your operating system.

For full details of the configurable options available see the [built-in proxy
configuration reference](/docs/connect/configuration.html#built-in-proxy-options).

### Prepared Query Upstreams

The upstream destination may also be a
[prepared query](/api/query.html).
This allows complex service discovery behavior such as connecting to
the nearest neighbor or filtering by tags.

For example, given a prepared query named "nearest-redis" that is
configured to route to the nearest Redis instance, an upstream can be
configured to route to this query. In the example below, any TCP connection
to port 1234 will attempt a Connect-based connection to the nearest Redis
service.

```json
{
  "service": {
    "name": "web",
    "port": 8080,
    "connect": {
      "proxy": {
        "config": {
          "upstreams": [{
            "destination_name": "redis",
            "destination_type": "prepared_query",
            "local_bind_port": 1234
          }]
        }
      }
    }
  }
}
```

-> **Note:** Connect does not currently support cross-datacenter
service communication. Therefore, prepared queries with Connect should
only be used to discover services within a single datacenter. See
[Multi-Datacenter Connect](/docs/connect/index.html#multi-datacenter) for
more information.

For full details of the configurable options available see the [built-in proxy
configuration reference](/docs/connect/configuration.html#built-in-proxy-options).

### Dynamic Upstreams

If an application requires dynamic dependencies that are only available
at runtime, it must currently [natively integrate](/docs/connect/native.html)
with Connect. After natively integrating, the HTTP API or
[DNS interface](/docs/agent/dns.html#connect-capable-service-lookups)
can be used.

### Custom Managed Proxy

[Custom proxies](/docs/connect/proxies/integrate.html) can also be
configured to run as a managed proxy. To configure custom proxies, specify
an alternate command to execute for the proxy:

```json
{
  "service": {
    "name": "web",
    "port": 8080,
    "connect": {
      "proxy": {
        "exec_mode": "daemon",
        "command":   ["/usr/bin/my-proxy", "-flag-example"]
      }
    }
  }
}
```

The `exec_mode` value specifies how the proxy is executed. The only
supported value at this time is "daemon". The command is the binary and
any arguments to execute.
The "daemon" mode expects a proxy to run as a long-running, blocking
process. It should not double-fork into the background. The custom
proxy should retrieve its configuration (such as the port to run on)
via the [custom proxy integration APIs](/docs/connect/proxies/integrate.html).

The default proxy command can be changed at an agent-global level
in the agent configuration. An example in HCL format is shown below.

```
connect {
  proxy_defaults {
    command = ["/usr/bin/my-proxy"]
  }
}
```

With this configuration, all services registered without an explicit
proxy command will use `my-proxy` instead of the default built-in proxy.

### Managed Proxy Logs

Managed proxies have both stdout and stderr captured in log files in the agent's
`data_dir`. They can be found in
`<data_dir>/proxy/logs/<proxy_service_id>-std{err,out}.log`.

The built-in proxy will inherit it's log level from the agent so if the agent is
configured with `log_level = DEBUG`, a proxy it starts will also output `DEBUG`
level logs showing service discovery, certificate and authorization information.

~> **Note:** In `-dev` mode there is no `data_dir` unless one is explicitly
configured so logging is disabled. You can access logs by providing the
[`-data-dir`](/docs/agent/options.html#_data_dir) CLI option. If a data dir is
configured, this will also cause proxy processes to stay running when the agent
terminates as described in [Lifecycle](#lifecycle).

## Unmanaged Proxies

Unmanaged proxies are regular Consul services that are registered as a
proxy type and declare the service they represent. The proxy process must
be started, configured, and stopped manually by some external process such
as an operator or scheduler.

To declare a service as a proxy, the service definition must contain
at least two additional fields:

  * `Kind` (string) must be set to `connect-proxy`. This declares that the
    service is a proxy type.

  * `ProxyDestination` (string) must be set to the service that this proxy
    is representing.

  * `Port` must be set so that other Connect services can discover the exact
    address for connections. `Address` is optional if the service is being
    registered against an agent, since it'll inherit the node address.

Example:

```json
{
  "Name": "redis-proxy",
  "Kind": "connect-proxy",
  "ProxyDestination": "redis",
  "Port": 8181
}
```

With this service registered, any Connect proxies searching for a
Connect-capable endpoint for "redis" will find this proxy.

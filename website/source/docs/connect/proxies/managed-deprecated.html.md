---
layout: "docs"
page_title: "Connect - Deprecated Managed Proxies"
sidebar_current: "docs-connect-proxies"
description: |-
  Consul 1.2 launched it's Connect Beta period with a feature named Managed
  Proxies which are now deprecated. This page describes how they worked and why
  they are no longer supported.
---

# Managed Proxy Deprecation

Consul Connect was first released as a beta feature in Consul 1.2.0. The initial
release included a feature called "Managed Proxies". Managed proxies were
Connect proxies where the proxy process was started, configured, and stopped by
Consul. They were enabled via basic configurations within the service
definition.

!> **Consul 1.6.0 removes Managed Proxies completely.** 
This documentation is provided for prior versions only. You may consider using
[sidecar service
registrations](/docs/connect/proxies/sidecar-service.html) instead.

Managed proxies have been deprecated since Consul 1.3 and have been fully removed
in Consul 1.6. Anyone using Managed Proxies should aim to change their workflow
as soon as possible to avoid issues with a later upgrade.

After transitioning away from all managed proxy usage, the `proxy` subdirectory inside [`data_dir`](https://www.consul.io/docs/agent/options.html#_data_dir) (specified in Consul config) can be deleted to remove extraneous configuration files and free up disk space.

**new and known issues will not be fixed**.

## Deprecation Rationale

Originally managed proxies traded higher implementation complexity for an easier
"getting started" user experience. After seeing how Connect was investigated and
adopted during beta it because obvious that they were not the best trade off.

Managed proxies only really helped in local testing or VM-per-service based
models whereas a lot of users jumped straight to containers where they are not
helpful. They also add only targeted fairly basic supervisor features which
meant most people would want to use something else in production for consistency
with other workloads. So the high implementation cost of building robust process
supervision didn't actually benefit most real use-cases.

Instead of this Connect 1.3.0 introduces the concept of [sidecar service
registrations](/docs/connect/proxies/sidecar-service.html) which
have almost all of the benefits of simpler configuration but without any of the
additional process management complexity. As a result they can be used to
simplify configuration in both container-based and realistic production
supervisor settings.

## Managed Proxy Documentation

As the managed proxy features continue to be supported for now, the rest of this
page will document how they work in the interim.

-> **Deprecation Note:** It's _strongly_ recommended you do not build anything
using Managed proxies and consider using [sidecar service
registrations](/docs/connect/proxies/sidecar-service.html) instead.

Managed proxies are given
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

-> **Security Note:** 1.) Managed proxies can only be configured
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
        "upstreams": [{
          "destination_name": "redis",
          "local_bind_port": 1234
        }]
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

-> **Deprecation Note:** versions 1.2.0 to 1.3.0 required specifying `upstreams`
as part of the opaque `config` that is passed to the proxy. However, since
1.3.0, the `upstreams` configuration is now specified directily under the
`proxy` key. Old service definitions using the nested config will continue to
work and have the values copied into the new location. This allows the upstreams
to be registered centrally rather than being part of the local-only config
passed to the proxy instance.

For full details of the additional configurable options available when using the
built-in proxy see the [built-in proxy configuration
reference](/docs/connect/configuration.html#built-in-proxy-options).

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
        "upstreams": [{
          "destination_name": "redis",
          "destination_type": "prepared_query",
          "local_bind_port": 1234
        }]
      }
    }
  }
}
```

For full details of the additional configurable options available when using the
built-in proxy see the [built-in proxy configuration
reference](/docs/connect/configuration.html#built-in-proxy-options).

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
        "command":   ["/usr/bin/my-proxy", "-flag-example"],
        "config": {
          "foo": "bar"
        }
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

The `config` key is an optional opaque JSON object which will be passed through
to the proxy via the proxy configuration endpoint to allow any configuration
options the proxy needs to be specified. See the [built-in proxy
configuration reference](/docs/connect/configuration.html#built-in-proxy-options)
for details of config options that can be passed when using the built-in proxy.

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

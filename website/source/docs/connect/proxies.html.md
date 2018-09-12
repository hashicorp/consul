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

-> **Note:** Connect does not currently support cross-datacenter
service communication. Therefore, prepared queries with Connect should
only be used to discover services within a single datacenter. See
[Multi-Datacenter Connect](/docs/connect/index.html#multi-datacenter) for
more information.

For full details of the additional configurable options available when using the
built-in proxy see the [built-in proxy configuration
reference](/docs/connect/configuration.html#built-in-proxy-options).

### Dynamic Upstreams

If an application requires dynamic dependencies that are only available
at runtime, it must currently [natively integrate](/docs/connect/native.html)
with Connect. After natively integrating, the HTTP API or
[DNS interface](/docs/agent/dns.html#connect-capable-service-lookups)
can be used.

### Upstream Configuration Reference

The following example shows all possible upstream configuration parameters.

Note that in versions 1.2.0 to 1.3.0, managed proxy upstreams were specified
inside the opaque `connect.proxy.config` map. The format is almost unchanged
however managed proxy upstreams are now defined a level up in the
`connect.proxy.upstreams`. The old location is deprecated and will be
automatically converted into the new for an interim period before support is
dropped in a future major release. The only difference in format between the
upstream defintions is that the field `destination_datacenter` has been renamed
to `datacenter` to reflect that it's the discovery target and not necessarily
the same as the instance that will be returned in the case of a prepared query
that fails over to another datacenter.

Note that `snake_case` is used here as it works in both [config file and API
registrations](/docs/agent/services.html#service-definition-parameter-case).

```json
{
  "destination_type": "service",
  "destination_name": "redis",
  "datacenter": "dc1",
  "local_bind_address": "127.0.0.1",
  "local_bind_port": 1234,
  "config": {}
},
```

* `destination_name` `string: <required>` - Specifies the name of the service or
  prepared query to route connect to.
* `local_bind_port` `int: <required>` - Specifies the port to bind a local
  listener to for the application to make outbound connections to this upstream.
* `local_bind_address` `string: <optional>` - Specifies the address to bind a
  local listener to for the application to make outbound connections to this
  upstream. Defaults to `127.0.0.1`.
* `destination_type` `string: <optional>` - Speficied the type of discovery
  query to use to find an instance to connect to. Valid values are `service` or
  `prepared_query`. Defaults to `service`.
* `datacenter` `string: <optional>` - Specifies the datacenter to issue the
  discovery query too. Defaults to the local datacenter.
* `config` `object: <optional>` - Specifies opaque configuration options that
  will be provided to the proxy instance for this specific upstream. Can contain
  any valid JSON object. This might be used to configure proxy-specific features
  like timeouts or retries for the given upstream. See the 
  [built-in proxy configuration reference](/docs/connect/configuration.html#built-in-proxy-options)
  for options available when using the built-in proxy.

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

## Unmanaged Proxies

Unmanaged proxies are regular Consul services that are registered as a
proxy type and declare the service they represent. The proxy process must
be started, configured, and stopped manually by some external process such
as an operator or scheduler.

To declare a service as a proxy, the service definition must contain
the following fields:

  * `kind` (string) must be set to `connect-proxy`. This declares that the
    service is a proxy type.

  * `proxy.destination_service_name` (string) must be set to the service that this
    proxy is representing. Note that this replaces `proxy_destination` in
    versions 1.2.0 to 1.3.0.

  * `port` must be set so that other Connect services can discover the exact
    address for connections. `address` is optional if the service is being
    registered against an agent, since it'll inherit the node address.

Minimal Example:

```json
{
  "name": "redis-proxy",
  "kind": "connect-proxy",
  "proxy": {
    "destination_service_name": "redis"
  },
  "port": 8181
}
```

With this service registered, any Connect proxies searching for a
Connect-capable endpoint for "redis" will find this proxy.

### Complete Configuration Example

The following is a complete example showing all the options available when
registering an unmanaged proxy instance.

```json
{
  "name": "redis-proxy",
  "kind": "connect-proxy",
  "proxy": {
    "destination_service_name": "redis",
    "destination_service_id": "redis1",
    "local_service_address": "127.0.0.1",
    "local_service_port": 9090,
    "config": {},
    "upstreams": []
  },
  "port": 8181
}
```

#### Proxy Parameters

 - `destination_service_name` `string: <required>` - Specifies the _name_ of the
   service this instance is proxying. Both side-car and centralized
   load-balancing proxies must specify this. It is used during service
   discovery to find the correct proxy instances to route to for a given service
   name.
  
 - `destination_service_id` `string: <optional>` - Specifies the _ID_ of a single
   specific service instance that this proxy is representing. This is only valid
   for side-car style proxies that run on the same node. It is assumed that the
   service instance is registered via the same Consul agent so the ID is unique
   and has no node qualifier. This is useful to show in tooling which proxy
   instance is a side-car for which application instance and will enable
   fine-grained analysis of the metrics coming from the proxy.

 - `local_service_address` `string: <optional>` - Specifies the address a side-car
   proxy should attempt to connect to the local application instance on.
   Defaults to 127.0.0.1.

 - `local_service_port` `int: <optional>` - Specifies the port a side-car
   proxy should attempt to connect to the local application instance on.
   Defaults to the port advertised by the service instance identified by 
   `destination_service_id` if it exists otherwise it may be empty in responses.

 - `config` `object: <optional>` - Specifies opaque config JSON that will be
   stored and returned along with the service instance from future API calls.

 - `upstreams` `array<Upstream>: <optional>` - Specifies the upstream services
   this proxy should create listeners for. The format is defined in 
   [Upstream Configuration Reference](#upstream-configuration-reference).
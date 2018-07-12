---
layout: "docs"
page_title: "Connect - Configuration"
sidebar_current: "docs-connect-config"
description: |-
  A Connect-aware proxy enables unmodified applications to use Connect. A per-service proxy sidecar transparently handles inbound and outbound service connections, automatically wrapping and verifying TLS connections.
---

# Connect Configuration

There are many configuration options exposed for Connect. The only option
that must be set is the "enabled" option on Consul Servers to enable Connect.
All other configurations are optional and have reasonable defaults.

## Enable Connect on the Cluster

The first step to use Connect is to enable Connect for your Consul
cluster. By default, Connect is disabled. Enabling Connect requires changing
the configuration of only your Consul _servers_ (not client agents). To enable
Connect, add the following to a new or existing
[server configuration file](/docs/agent/options.html). In HCL:

```hcl
connect {
  enabled = true
}
```

This will enable Connect and configure your Consul cluster to use the
built-in certificate authority for creating and managing certificates.
You may also configure Consul to use an external
[certificate management system](/docs/connect/ca.html), such as
[Vault](https://vaultproject.io).

No agent-wide configuration is necessary for non-server agents. Services
and proxies may always register with Connect settings, but they will fail to
retrieve or verify any TLS certificates. This causes all Connect-based
connection attempts to fail until Connect is enabled on the server agents.

-> **Note:** Connect is enabled by default when running Consul in
dev mode with `consul agent -dev`.

~> **Security note:** Enabling Connect is enough to try the feature but doesn't
automatically ensure complete security. Please read the [Connect production
guide](/docs/guides/connect-production.html) to understand the additional steps
needed for a secure deployment.

## Built-In Proxy Options

This is a complete example of all the configuration options available for the
built-in proxy. Note that only the `service.connect.proxy.config` map is being
described here, the rest of the service definition is shown for context and is
[described elsewhere](/docs/connect/proxies.html#managed-proxies).

```javascript
{
  "service": {
    "name": "web",
    "port": 8080,
    "connect": {
      "proxy": {
        "config": {
          "bind_address": "0.0.0.0",
          "bind_port": 20000,
          "tcp_check_address": "192.168.0.1",
          "disable_tcp_check": false,
          "local_service_address": "127.0.0.1:1234",
          "local_connect_timeout_ms": 1000,
          "handshake_timeout_ms": 10000,
          "upstreams": [
            {
              "destination_type": "service",
              "destination_name": "redis",
              "destination_datacenter": "dc1",
              "local_bind_address": "127.0.0.1",
              "local_bind_port": 1234,
              "connect_timeout_ms": 10000
            },
          ]
        }
      }
    }
  }
}
```

#### Configuration Key Reference

All fields are optional with a sane default.

* <a name="bind_address"></a><a href="#bind_address">`bind_address`</a> -
  The address the proxy will bind it's _public_ mTLS listener to. It
  defaults to the same address the agent binds to.

* <a name="bind_port"></a><a href="#bind_port">`bind_port`</a> - The
  port the proxy will bind it's _public_ mTLS listener to. If not provided, the
  agent will attempt to assign one from its [configured proxy port
  range](/docs/agent/options.html#proxy_min_port) if available. By default the
  range is [20000, 20255] and the port is selected at random from that range.

* <a name="tcp_check_address"></a><a
  href="#tcp_check_address">`tcp_check_address`</a> - The address the agent will
  run a [TCP health check](/docs/agent/checks.html) against. By default this is
  the same as the proxy's [bind address](#bind_address) except if the
  bind_address is `0.0.0.0` or `[::]` in which case this defaults to `127.0.0.1`
  and assumes the agent can dial the proxy over loopback. For more complex
  configurations where agent and proxy communicate over a bridge for example,
  this configuration can be used to specify a different _address_ (but not port)
  for the agent to use for health checks if it can't talk to the proxy over
  localhost or it's publicly advertised port. The check always uses the same
  port that the proxy is bound to.

* <a name="disable_tcp_check"></a><a
  href="#disable_tcp_check">`disable_tcp_check`</a> - If true, this disables a
  TCP check being setup for the proxy. Default is false.

* <a name="local_service_address"></a><a href="#local_service_address">`local_service_address`</a> - The
  `[address]:port` that the proxy should use to connect to the local application
  instance. By default it assumes `127.0.0.1` as the address and takes the port
  from the service definition's `port` field. Note that allowing the application
  to listen on any non-loopback address may expose it externally and bypass
  Connect's access enforcement. It may be useful though to allow non-standard
  loopback addresses or where an alternative known-private IP is available for
  example when using internal networking between containers.

* <a name="local_connect_timeout_ms"></a><a href="#local_connect_timeout_ms">`local_connect_timeout_ms`</a> - The number
  of milliseconds the proxy will wait to establish a connection to the _local
  application_ before giving up. Defaults to `1000` or 1 second.

* <a name="handshake_timeout_ms"></a><a href="#handshake_timeout_ms">`handshake_timeout_ms`</a> - The
  number of milliseconds the proxy will wait for _incoming_ mTLS connections to 
  complete the TLS handshake. Defaults to `10000` or 10 seconds.

* <a name="upstreams"></a><a href="#upstreams">`upstreams`</a> - An array of
  upstream definitions for remote services that the proxied
  application needs to make outgoing connections to. Each definition has the
  following fields:
  * <a name="destination_name"></a><a href="#destination_name">`destination_name`</a> - 
    [required] The name of the service or prepared query to route connect to.
  * <a name="local_bind_port"></a><a href="#local_bind_port">`local_bind_port`</a> - 
    [required] The port to bind a local listener to for the application to
    make outbound connections to this upstream.
  * <a name="local_bind_address"></a><a href="#local_bind_address">`local_bind_address`</a> - 
    The address to bind a local listener to for the application to make
    outbound connections to this upstream.
  * <a name="destination_type"></a><a href="#destination_type">`destination_type`</a> - 
    Either `service` or `upstream`. The type of discovery query to use to find 
    an instance to connect to. Defaults to `service`.
  * <a name="destination_datacenter"></a><a href="#destination_datacenter">`destination_datacenter`</a> - 
    The datacenter to issue the discovery query too. Defaults to the local datacenter.
  * <a name="connect_timeout_ms"></a><a href="#connect_timeout_ms">`connect_timeout_ms`</a> - 
    The number of milliseconds the proxy will wait to establish a connection to 
    and complete TLS handshake with the _remote_ application or proxy. Defaults 
    to `10000` or 10 seconds.


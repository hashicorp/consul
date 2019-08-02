---
layout: "docs"
page_title: "Connect - Built-in Proxy"
sidebar_current: "docs-connect-proxies-built-in"
description: |-
  Consul Connect comes with a built-in proxy for testing and development.
---

# Built-In Proxy Options

Consul comes with a built-in L4 proxy for testing and development with Consul
Connect.

Below is a complete example of all the configuration options available
for the built-in proxy.

~> **Note:** Although you can configure the built-in proxy using configuration
entries, it doesn't have the L7 capability necessary for the observability
features released with Consul 1.5.

```javascript
{
  "service": {
    ...
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
          "upstreams": [...]
        },
        "upstreams": [
          {
            ...
            "config": {
              "connect_timeout_ms": 1000
            }
          }
        ]
      }
    }
  }
}
```

## Proxy Config Key Reference

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

* <a name="upstreams"></a><a href="#upstreams">`upstreams`</a> - **Deprecated**
  Upstreams are now specified in the `connect.proxy` definition. Upstreams
  specified in the opaque config map here will continue to work for
  compatibility but it's strongly recommended that you move to using the higher
  level [upstream
  configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference).

## Proxy Upstream Config Key Reference

All fields are optional with a sane default.

* <a name="connect_timeout_ms"></a><a
  href="#connect_timeout_ms">`connect_timeout_ms`</a> - The number of
  milliseconds the proxy will wait to establish a TLS connection to the
  discovered upstream instance before giving up. Defaults to `10000` or 10
  seconds.

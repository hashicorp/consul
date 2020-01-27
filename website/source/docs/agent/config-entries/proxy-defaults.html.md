---
layout: "docs"
page_title: "Configuration Entry Kind: Proxy Defaults"
sidebar_current: "docs-agent-cfg_entries-proxy_defaults"
description: |-
  The proxy-defaults config entry kind allows for configuring global config defaults across all services for Connect proxy configuration. Currently, only one global entry is supported.
---

# Proxy Defaults

The `proxy-defaults` config entry kind allows for configuring global config
defaults across all services for Connect proxy configuration. Currently, only
one global entry is supported.

## Sample Config Entries

Set the default protocol for all sidecar proxies:

```hcl
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
```

Set proxy-specific defaults:

```hcl
kind = "proxy-defaults"
name = "global"
config {
  local_connect_timeout_ms = 1000
  handshake_timeout_ms = 10000
}
```

## Available Fields

- `Kind` - Must be set to `proxy-defaults`

- `Name` - Must be set to `global`

- `Config` `(map[string]arbitrary)` - An arbitrary map of configuration values used by Connect proxies.
  The available configurations depend on the Connect proxy you use. Any values
  that your proxy allows can be configured globally here. To
  explore these options please see the documentation for your chosen proxy.

  * [Envoy](/docs/connect/proxies/envoy.html#bootstrap-configuration)
  * [Consul's built-in proxy](/docs/connect/proxies/built-in.html)

- `MeshGateway` `(MeshGatewayConfig: <optional>)` - Controls the default
  [mesh gateway configuration](/docs/connect/mesh_gateway.html#connect-proxy-configuration)
  for all proxies. Added in v1.6.0.

  - `Mode` `(string: "")` - One of `none`, `local`, or `remote`.
  
- `Expose` `(ExposeConfig: <optional>)` - Controls the default
  [expose path configuration](/docs/connect/registration/service-registration.html#expose-paths-configuration-reference)
  for Envoy. Added in v1.6.2.

  Exposing paths through Envoy enables a service to protect itself by only listening on localhost, while still allowing 
  non-Connect-enabled applications to contact an HTTP endpoint. 
  Some examples include: exposing a `/metrics` path for Prometheus or `/healthz` for kubelet liveness checks.

  - `Checks` `(bool: false)` - If enabled, all HTTP and gRPC checks registered with the agent are exposed through Envoy.
 Envoy will expose listeners for these checks and will only accept connections originating from localhost or Consul's 
 [advertise address](/docs/agent/options.html#advertise). The port for these listeners are dynamically allocated from 
 [expose_min_port](/docs/agent/options.html#expose_min_port) to [expose_max_port](/docs/agent/options.html#expose_max_port). 
 This flag is useful when a Consul client cannot reach registered services over localhost. One example is when running 
 Consul on Kubernetes, and Consul agents run in their own pods.
  - `Paths` `array<Path>: []` - A list of paths to expose through Envoy.
    - `Path` `(string: "")` - The HTTP path to expose. The path must be prefixed by a slash. ie: `/metrics`.
    - `LocalPathPort` `(int: 0)` - The port where the local service is listening for connections to the path.
    - `ListenerPort` `(int: 0)` - The port where the proxy will listen for connections. This port must be  available 
    for the listener to be set up. If the port is not free then Envoy will not expose a listener for the path, 
    but the proxy registration will not fail. 
    - `Protocol` `(string: "http")` - Sets the protocol of the listener. One of `http` or `http2`. For gRPC use `http2`.

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `proxy-defaults` config entry requires no specific privileges.

Creating, updating, or deleting a `proxy-defaults` config entry requires
`operator:write`.

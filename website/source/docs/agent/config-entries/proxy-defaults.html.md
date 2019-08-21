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

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `proxy-defaults` config entry requires no specific privileges.

Creating, updating, or deleting a `proxy-defaults` config entry requires
`operator:write`.

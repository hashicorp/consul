---
layout: "docs"
page_title: "Configuration Entry Kind: Service Defaults"
sidebar_current: "docs-agent-cfg_entries-service_defaults"
description: |-
  The service-defaults config entry kind controls default global values for a service, such as its protocol.
---

# Service Defaults

The `service-defaults` config entry kind controls default global values for a
service, such as its protocol.

## Sample Config Entries

Set the default protocol for a service to HTTP:

```hcl
Kind = "service-defaults"
Name = "web"
Protocol = "http"
```

## Available Fields

- `Kind` - Must be set to `service-defaults`

- `Name` `(string: <required>)` - Set to the name of the service being configured.

- `Protocol` `(string: "tcp")` - Sets the protocol of the service. This is used
  by Connect proxies for things like observability features and to unlock usage
  of the [`service-splitter`](/docs/agent/config-entries/service-splitter.html) and
  [`service-router`](/docs/agent/config-entries/service-router.html) config
  entries for a service.

- `MeshGateway` `(MeshGatewayConfig: <optional>)` - Controls the default
  [mesh gateway configuration](/docs/connect/mesh_gateway.html#connect-proxy-configuration)
  for this service. Added in v1.6.0.

  - `Mode` `(string: "")` - One of `none`, `local`, or `remote`.

- `ExternalSNI` `(string: "")` - This is an optional setting that allows for
  the TLS [SNI](https://en.wikipedia.org/wiki/Server_Name_Indication) value to
  be changed to a non-connect value when federating with an external system.
  Added in v1.6.0.

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `service-defaults` config entry requires `service:read` on itself.

Creating, updating, or deleting a `service-defaults` config entry requires
`service:write` on itself.

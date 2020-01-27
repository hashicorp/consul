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
    - `ListenerPort` `(int: 0)` - The port where the proxy will listen for connections. This port must be  available for 
    the listener to be set up. If the port is not free then Envoy will not expose a listener for the path, 
    but the proxy registration will not fail. 
    - `Protocol` `(string: "http")` - Sets the protocol of the listener. One of `http` or `http2`. For gRPC use `http2`.

## ACLs

Configuration entries may be protected by
[ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls).

Reading a `service-defaults` config entry requires `service:read` on itself.

Creating, updating, or deleting a `service-defaults` config entry requires
`service:write` on itself.

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

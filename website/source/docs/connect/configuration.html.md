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

-> **Tip:** Connect is enabled by default when running Consul in
dev mode with `consul agent -dev`.

## Agent Configuration

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

Services and proxies may always register with Connect settings, but they will
fail to retrieve or verify any TLS certificates. This causes all Connect-based
connection attempts to fail until Connect is enabled on the server agents.

Other optional Connect configurations that you can set in the server
configuration file include:

- [certificate authority settings](/docs/agent/options.html#connect)
- [token replication](/docs/agent/options.html#acl_tokens_replication)
- [dev mode](/docs/agent/options.html#_dev)
- [server host name verification](/docs/agent/options.html#verify_server_hostname)

If you would like to use Envoy as your Connect proxy you will need to [enable
gRPC](/docs/agent/options.html#grpc_port).

Additionally if you plan on using the observability features of Connect, it can
be convenient to configure your proxies and services using [configuration
entries](/docs/agent/config_entries.html) which you can interact with using the
CLI or API, or by creating configuration entry files. You will want to enable
[centralized service
configuration](/docs/agent/options.html#enable_central_service_config) on
clients, which allows each service's proxy configuration to be managed centrally
via API.

!> **Security note:** Enabling Connect is enough to try the feature but doesn't
automatically ensure complete security. Please read the [Connect production
guide](https://learn.hashicorp.com/consul/developer-segmentation/connect-production) to understand the additional steps
needed for a secure deployment.

## Centralized Proxy and Service Configuration

To account for common Connect use cases where you have many instances of the
same service, and many colocated sidecar proxies, Consul allows you to customize
the settings for all of your proxies or all the instances of a given service at
once using [Configuration Entries](/docs/agent/config_entries.html).

You can override centralized configurations for individual proxy instances in
their
[sidecar service definitions](/docs/connect/registration/sidecar-service.html),
and the default protocols for service instances in their [service
registrations](/docs/agent/services.html).

## Schedulers

Consul Connect is especially useful if you are using an orchestrator like Nomad
or Kubernetes, because these orchestrators can deploy thousands of service instances
which frequently move hosts. Sidecars for each service can be configured through
these schedulers, and in some cases they can automate Consul configuration,
sidecar deployment, and service registration.

### Nomad

Connect can be used with Nomad to provide secure service-to-service
communication between Nomad jobs and task groups. The ability to use the dynamic
port feature of Nomad makes Connect particularly easy to use. Learn about how to
configure Connect on Nomad by reading the
[integration documentation](/docs/connect/platform/nomad.html)

### Kubernetes

The Consul Helm chart can automate much of Consul Connect's configuration, and
makes it easy to automatically inject Envoy sidecars into new pods when they are
deployed. Learn about the [Helm chart](/docs/platform/k8s/helm.html) in general,
or if you are already familiar with it, check out it's
[connect specific configurations](/docs/platform/k8s/connect.html).

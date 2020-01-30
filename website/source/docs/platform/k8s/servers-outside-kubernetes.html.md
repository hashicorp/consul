---
layout: "docs"
page_title: "Consul Servers Outside of Kubernetes - Kubernetes"
sidebar_current: "docs-platform-k8s-run-servers-outside"
description: |-
  Running Consul servers outside of Kubernetes
---

# Consul Servers Outside of Kubernetes

If you have a Consul cluster already running, you can configure your
Consul clients inside Kubernetes to join this existing cluster.

The below `config.yaml` file shows how to configure the Helm chart to install
Consul clients that will join an existing cluster.

The `global.enabled` value first disables all chart components by default
so that each component is opt-in. This allows us to _only_ setup the client
agents. We then opt-in to the client agents by setting `client.enabled` to
`true`.

Next, `client.exposeGossipPorts` can be set to `true` or `false` depending on if
you want the clients to be exposed on the Kubernetes internal node IPs (`true`) or
their pod IPs (`false`).

Finally, `client.join` is set to an array of valid
[`-retry-join` values](/docs/agent/options.html#retry-join). In the
example above, a fake [cloud auto-join](/docs/agent/cloud-auto-join.html)
value is specified. This should be set to resolve to the proper addresses of
your existing Consul cluster.

```yaml
# config.yaml
global:
  enabled: false

client:
  enabled: true
  # Set this to true to expose the Consul clients using the Kubernetes node
  # IPs. If false, the pod IPs must be routable from the external servers.
  exposeGossipPorts: true
  join:
    - "provider=my-cloud config=val ..."
```


-> **Networking:** Note that for the Kubernetes nodes to join an existing
cluster, the nodes (and specifically the agent pods) must be able to connect
to all other server and client agents inside and _outside_ of Kubernetes over [LAN](https://www.consul.io/docs/glossary.html#lan-gossip).
If this isn't possible, consider running a separate Consul cluster inside Kubernetes
and federating it with your cluster outside Kubernetes.
You may also consider adopting Consul Enterprise for
[network segments](/docs/enterprise/network-segments/index.html).


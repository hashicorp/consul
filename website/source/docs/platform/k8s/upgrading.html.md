---
layout: "docs"
page_title: "Upgrading"
sidebar_current: "docs-platform-k8s-ops-upgrading"
description: |-
  Upgrading Consul on Kubernetes
---

# Upgrading Consul on Kubernetes

To upgrade Consul on Kubernetes, we follow the same pattern as
[generally upgrading Consul](/docs/upgrading.html), except we can use
the Helm chart to step through a rolling deploy. It is important to understand
how to [generally upgrade Consul](/docs/upgrading.html) before reading this
section.

Upgrading Consul on Kubernetes will follow the same pattern: each server
will be updated one-by-one. After that is successful, the clients will
be updated in batches.

## Upgrading Consul Servers

To initiate the upgrade, change the `server.image` value to the
desired Consul version. For illustrative purposes, the example below will
use `consul:123.456`. Also, set the `server.updatePartition` value
_equal to the number of server replicas_:

```yaml
server:
  image: "consul:123.456"
  replicas: 3
  updatePartition: 3
```

The `updatePartition` value controls how many instances of the server
cluster are updated. Only instances with an index _greater than_ the
`updatePartition` value are updated (zero-indexed). Therefore, by setting
it equal to replicas, none should update yet.

Next, run the upgrade. You should run this with `--dry-run` first to verify
the changes that will be sent to the Kubernetes cluster.

```
$ helm upgrade consul ./
...
```

This should cause no changes (although the resource will be updated). If
everything is stable, begin by decreasing the `updatePartition` value by one,
and running `helm upgrade` again. This should cause the first Consul server
to be stopped and restarted with the new image.

Wait until the Consul server cluster is healthy again (30s to a few minutes)
then decrease `updatePartition` and upgrade again. Continue until
`updatePartition` is `0`. At this point, you may remove the
`updatePartition` configuration. Your server upgrade is complete.

## Upgrading Consul Clients

With the servers upgraded, it is time to upgrade the clients. To upgrade
the clients, set the `client.image` value to the desired Consul version.
Then, run `helm upgrade`. This will upgrade the clients in batches, waiting
until the clients come up healthy before continuing.

## Configuring TLS on an Existing Cluster

As of version `0.16.0`, the Consul Helm chart supports TLS for communication
within the cluster. If you already have a Consul cluster deployed on Kubernetes,
you may want to configure TLS in a way that minimizes downtime to your applications.
Consul already supports rolling out TLS on an existing cluster without downtime.
However, depending on your Kubernetes use case, your upgrade procedure may be different.

### Gradual TLS Rollout without Consul Connect

If you're not using Consul Connect, follow this process.

1. Run a Helm upgrade with the following config:
   ```yaml
   global:
     tls:
       enabled: true
       # This configuration sets `verify_outgoing`, `verfiy_server_hostname`,
       # and `verify_incoming` to `false` on servers and clients,
       # which allows TLS-disabled nodes to join the cluster.
       verify: false
   server:
     updatePartition: <number_of_server_replicas>
   ```
   This upgrade will trigger a rolling update of the clients, as well as any
   other `consul-k8s` components, such as sync catalog or client snapshot deployments.
1. Perform a rolling upgrade of the servers, as described in
   [Upgrade Consul Servers](#upgrading-consul-servers).
1. Repeat steps 1 and 2, turning on TLS verification by removing the
   `global.tls.verify` property.

### Gradual TLS Rollout with Consul Connect

Because the Envoy proxy needs to talk to the Consul client agent regularly
for service discovery, we can't enable TLS on the clients without also re-injecting
TLS-enabled proxy into the application pods. To perform TLS rollout with minimal
downtime, we recommend instead to add a new Kubernetes node pool and migrate your
applications to it.

1. Add a new identical node pool.
1. Cordon all nodes in the old pool by running `kubectl cordon`
   to ensure Kubernetes doesn't schedule any new workloads on those nodes.
1. Create the following Helm config file for the upgrade:
    ```yaml
    global:
      tls:
        enabled: true
        # This configuration sets `verify_outgoing`, `verfiy_server_hostname`,
        # and `verify_incoming` to `false` on servers and clients,
        # which allows TLS-disabled nodes to join the cluster.
        verify: false
    server:
      updatePartition: <number_of_server_replicas>
    client:
      updateStrategy: |
         type: OnDelete
    ```
   In this configuration, we're setting `server.updatePartition` to the number of
   server replicas as described in [Upgrade Consul Servers](#upgrading-consul-servers)
   and `client.updateStrategy` to `OnDelete` to manually trigger an upgrade of the clients.
1. Run `helm upgrade` with the above config file. The upgrade will trigger an update of any
   component except clients and servers, such as Consul Connect webhook deployment
   or sync catalog deployment. Note that the sync catalog and the client
   snapshot deployments will not be in the `ready` state until the clients on their
   nodes are upgraded. It is OK to proceed to the next step without them being ready
   because Kubernetes will keep the old deployment pod around, and so there will be no
   downtime.
1. Gradually perform an upgrade of the clients by deleting client pods on the new node
   pool.
1. Redeploy all your Connect-enabled applications. Now that the Connect injector is TLS-aware,
   it will add TLS configuration to the sidecar proxy. Also, Kubernetes should schedule
   these applications on the new node pool.
1. Perform a rolling upgrade of the servers described in
   [Upgrade Consul Servers](#upgrading-consul-servers).
1. If everything is healthy, delete the old node pool.
1. Finally, remove `global.tls.verify` from your Helm config file, as well as
   `client.updateStrategy`, and perform a rolling upgrade of the servers.
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

If you already have a Consul cluster deployed on Kubernetes and
would like to turn on TLS for internal Consul communication,
please see
[Configuring TLS on an Existing Cluster](/docs/platform/k8s/tls-on-existing-cluster.html).

---
layout: "docs"
page_title: "Consul Clients Outside of Kubernetes - Kubernetes"
sidebar_current: "docs-platform-k8s-run-clients-outside"
description: |-
  Consul clients running on non-Kubernetes nodes can join a Consul cluster running within Kubernetes.
---

# Consul Clients Outside Kubernetes

Consul clients running on non-Kubernetes nodes can join a Consul cluster running within Kubernetes.

## Auto-join

The recommended way to join a cluster running within Kubernetes is to
use the ["k8s" cloud auto-join provider](/docs/agent/cloud-auto-join.html#kubernetes-k8s-).

The auto-join provider dynamically discovers IP addresses to join using
the Kubernetes API. It authenticates with Kubernetes using a standard
`kubeconfig` file. This works with all major hosted Kubernetes offerings
as well as self-hosted installations. The token in the `kubeconfig` file
needs to have permissions to list pods in the namespace where Consul servers
are deployed.

The auto-join string below will join a Consul server cluster that is
started using the [official Helm chart](/docs/platform/k8s/helm.html):

```sh
$ consul agent -retry-join 'provider=k8s label_selector="app=consul,component=server"'
```

By default, Consul will join the default gossip port. Pods may set an
annotation `consul.hashicorp.com/auto-join-port` to an integer value or
a named port to specify the port for the auto-join to return. This enables
different pods to have different exposed ports.

## Networking

Consul typically requires a fully connected network.
Because the Consul Helm chart currently doesn't allow exposing servers' gossip ports via a `hostPort`,
nodes outside of Kubernetes joining a cluster running within Kubernetes must be able to communicate
to pod IPs via the network. Note that the auto-join provider discussed above will use pod IPs by default.

-> **Consul Enterprise customers** may use
[network segments](/docs/enterprise/network-segments/index.html) to
enable non-fully-connected topologies. However, out-of-cluster nodes must still
be able to communicate with the server pod or host IP addresses.

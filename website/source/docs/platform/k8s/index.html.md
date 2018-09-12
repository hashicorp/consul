---
layout: "docs"
page_title: "Kubernetes"
sidebar_current: "docs-platform-k8s-index"
description: |-
  Consul has many integrations with Kubernetes. You can deploy Consul to Kubernetes using the Helm Chart, sync services between Consul and Kubernetes, automatically secure Pod communication with Connect, and more. This section documents the official integrations between Consul and Kubernetes.
---

# Kubernetes

Consul has many integrations with Kubernetes. You can deploy Consul
to Kubernetes using the Helm Chart, sync services between Consul and
Kubernetes, automatically secure Pod communication with Connect, and more.
This section documents the official integrations between Consul and Kubernetes.

## Use Cases

**Running a Consul server cluster:** The Consul server cluster can run directly
on Kubernetes. This can be used by both nodes within Kubernetes as well as
nodes external to Kubernetes, as long as they can communicate to the server
nodes via the network.

**Running Consul clients:** Consul clients can run as pods on every node
and expose the Consul API to running pods. This enables many Consul tools
such as envconsul, consul-template, and more to work on Kubernetes since a
local agent is available. This will also register each Kubernetes node with
the Consul catalog for full visibility into your infrastructure

**Service sync to enable Kubernetes and non-Kubernetes services to communicate:**
Consul can sync Kubernetes services with its own service registry. This allows
Kubernetes services to use native Kubernetes service discovery to discover
and connect to external services, and for external services to use Consul
service discovery to discover and connect to Kubernetes services.

**Automatic encryption and authorization for pod network connections:**
Consul can automatically inject the [Connect](/docs/connect/index.html)
sidecar into pods so that they can accept and establish encrypted
and authorized network connections via mutual TLS. And because Connect
can run anywhere, pods can also communicate with external services (and
vice versa) over a fully encrypted connection.

**And more!** Consul can run directly on Kubernetes, so in addition to the
native integrations provided by Consul itself, any other tool built for
Kubernetes can choose to leverage Consul.

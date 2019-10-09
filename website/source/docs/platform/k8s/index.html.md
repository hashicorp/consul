---
layout: "docs"
page_title: "Kubernetes"
sidebar_current: "docs-platform-k8s-index"
description: |-
  Consul has many integrations with Kubernetes. You can deploy Consul to Kubernetes using the Helm chart, sync services between Consul and Kubernetes, automatically secure Pod communication with Connect, and more. This section documents the official integrations between Consul and Kubernetes.
---

# Kubernetes

Consul has many integrations with Kubernetes. You can deploy Consul
to Kubernetes using the Helm chart, sync services between Consul and
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
the Consul catalog for full visibility into your infrastructure.

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

## Getting Started With Consul and Kubernetes

There are several ways to try Consul with Kubernetes in different environments.

 - The [Consul and minikube guide](https://learn.hashicorp.com/consul/
   getting-started-k8s/minikube?utm_source=consul.io&utm_medium=docs) is a quick walk through of how to deploy Consul with the official Helm chart on a local instance of Minikube. 

 - The [Deploying Consul with Kubernetes guide](https://learn.hashicorp.com/
   consul/getting-started-k8s/minikube?utm_source=consul.io&utm_medium=docs)
   walks you through deploying Consul on Kubernetes with the official Helm chart and can be applied to any Kubernetes installation type.

 - The [Kubernetes on Azure guide](https://learn.hashicorp.com/consul/
   getting-started-k8s/azure-k8s?utm_source=consul.io&utm_medium=docs) is a complete walk through on how to deploy Consul on AKS.

 - The [Consul and Kubernetes Reference Architecture](
   https://learn.hashicorp.com/consul/day-1-operations/kubernetes-reference?utm_source=consul.io&utm_medium=docs) guide provides recommended practices for production. 

 - The [Consul and Kubernetes Deployment](
   https://learn.hashicorp.com/consul/day-1-operations/kubernetes-deployment-guide?utm_source=consul.io&utm_medium=docs) guide covers the necssary steps to install and configure a new Consul cluster on Kubernetes in production. 

## "consul-k8s" Project

The dedicated [consul-k8s project](https://github.com/hashicorp/consul-k8s)
contains the integration functionality between Consul and Kubernetes.
You generally will not need to invoke this project directly since the
[Helm chart](/docs/platform/k8s/helm.html) automates the installation and
configuration of the project when necessary.

We may integrate this functionality directly into Consul in the future,
but the separate project allows us to iterate and version the Kubernetes
functionality separately. Additionally, a lot of the functionality works
across multiple Consul versions, so you're able to update and resolve any
Kubernetes integration issues without also upgrading Consul itself which
can be more difficult.

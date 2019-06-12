---
layout: "docs"
page_title: "Connect (Service Segmentation)"
sidebar_current: "docs-connect-index"
description: |-
  Consul Connect provides service-to-service connection authorization and
  encryption using mutual TLS.
---

# Connect

Consul Connect provides service-to-service connection authorization and
encryption using mutual Transport Layer Security (TLS). Applications can use
[sidecar proxies](/docs/connect/proxies.html) in a service mesh configuration to
automatically establish TLS connections for inbound and outbound connections
without being aware of Connect at all. Applications may also [natively integrate
with Connect](/docs/connect/native.html) for optimal performance and security.
Connect can help you secure your services and provide data about service-to-service
communications.

## Application Security

Connect enables secure deployment best-practices with automatic
service-to-service encryption, and identity-based authorization.
Connect uses the registered service identity (rather than IP addresses) to
enforce access control with [intentions](/docs/connect/intentions.html). This
makes it easier to reason about access control and enables services to be
rescheduled by orchestrators including Kubernetes and Nomad. Intention
enforcement is network agnostic, so Connect works with physical networks, cloud
networks, software-defined networks, cross-cloud, and more.

## Observability

One of the key benefits Consul Connect is the uniform and consistent view it can
provide  of all the services on your network, irrespective of their different
programming languages and frameworks. When you configure Consul Connect to use
sidecar proxies, those proxies "see" all service-to-service traffic and can
collect data about it. Consul Connect can configure Envoy proxies to collect
layer 7 metrics and export them to tools like Prometheus. Correctly instrumented
application can also send open tracing data through Envoy.

## Getting Started With Connect

There are several ways to try Connect in different environments.

<<<<<<< HEAD
 - The [Connect introduction guide](https://learn.hashicorp.com/consul/getting-started/connect)
   is a simple walk through of connecting two services on your local machine
   using only Consul Connect, and configuring your first intention.
=======
 * The [Connect introduction](https://learn.hashicorp.com/consul/getting-started/connect) in the
   Getting Started guide provides a simple walk through of getting two services
   to communicate via Connect using only Consul directly on your local machine.

 * The [Envoy guide](https://learn.hashicorp.com/consul/developer-segmentation/connect-envoy) walks through getting
   started with Envoy as a proxy, and uses Docker to run components locally
   without installing anything else.

 * The [Kubernetes documentation](/docs/platform/k8s/run.html) shows how to get
   from an empty Kubernetes cluster to having Consul installed and Envoy
   configured to proxy application traffic automatically using the official helm
   chart.

## Agent Caching and Performance

To enable microsecond-speed responses on
[agent Connect API endpoints](/api/agent/connect.html), the Consul agent
locally caches most Connect-related data and sets up background
[blocking queries](/api/features/blocking.html) against the server
to update the cache in the background. This allows most API calls such
as retrieving certificates or authorizing connections to use in-memory
data and respond very quickly.

All data cached locally by the agent is populated on demand. Therefore,
if Connect is not used at all, the cache does not store any data. On first
request, the data is loaded from the server and cached. The set of data cached
is: public CA root certificates, leaf certificates, and intentions. For
leaf certificates and intentions, only data related to the service requested
is cached, not the full set of data.

Further, the cache is partitioned by ACL token and datacenters. This is done
to minimize the complexity of the cache and prevent bugs where an ACL token
may see data it shouldn't from the cache. This results in higher memory usage
for cached data since it is duplicated per ACL token, but with the benefit
of simplicity and security.

With Connect enabled, you'll likely see increased memory usage by the
local Consul agent. The total memory is dependent on the number of intentions
related to the services registered with the agent accepting Connect-based
connections. The other data (leaf certificates and public CA certificates)
is a relatively fixed size per service. In most cases, the overhead per
service should be relatively small: single digit kilobytes at most.

The cache does not evict entries due to memory pressure. If memory capacity
is reached, the process will attempt to swap. If swap is disabled, the Consul
agent may begin failing and eventually crash. Cache entries do have TTLs
associated with them and will evict their entries if they're not used. Given
a long period of inactivity (3 days by default), the cache will empty itself.

## Multi-Datacenter

Using Connect for service-to-service communications across multiple datacenters 
requires Consul Enterprise. 
>>>>>>> master

 - The [Envoy guide](https://learn.hashicorp.com/consul/developer-segmentation/connect-envoy)
   walks through using Envoy as a proxy. It uses Docker to run components
   locally without installing anything else.

 - The [Kubernetes guide](https://learn.hashicorp.com/consul/getting-started-k8s/minikube)
   walks you though configuring Consul Connect in Kubernetes using the Helm
   chart, and using intentions. You can run the guide on Minikube or an extant
   Kubernets cluster.

 - The [observability guide](https://learn.hashicorp.com/consul/getting-started-k8s/l7-observability-k8s)
   shows how to deploy a basic metrics collection and visualization pipeline on
   a Minikube or Kubernetes cluster using the official Helm charts for Consul,
   Prometheus, and Grafana.

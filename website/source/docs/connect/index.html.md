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

One of the key benefits of Consul Connect is the uniform and consistent view it can
provide  of all the services on your network, irrespective of their different
programming languages and frameworks. When you configure Consul Connect to use
sidecar proxies, those proxies "see" all service-to-service traffic and can
collect data about it. Consul Connect can configure Envoy proxies to collect
layer 7 metrics and export them to tools like Prometheus. Correctly instrumented
application can also send open tracing data through Envoy.

## Getting Started With Connect

There are several ways to try Connect in different environments.

 - The [Connect introduction guide](https://learn.hashicorp.com/consul/getting-started/connect)
   is a simple walk through of connecting two services on your local machine
   using only Consul Connect, and configuring your first intention.

 - The [Envoy guide](https://learn.hashicorp.com/consul/developer-segmentation/connect-envoy)
   walks through using Envoy as a proxy. It uses Docker to run components
   locally without installing anything else.

 - The [Kubernetes guide](https://learn.hashicorp.com/consul/getting-started-k8s/minikube)
   walks you through configuring Consul Connect in Kubernetes using the Helm
   chart, and using intentions. You can run the guide on Minikube or an extant
   Kubernets cluster.

 - The [observability guide](https://learn.hashicorp.com/consul/getting-started-k8s/l7-observability-k8s)
   shows how to deploy a basic metrics collection and visualization pipeline on
   a Minikube or Kubernetes cluster using the official Helm charts for Consul,
   Prometheus, and Grafana.

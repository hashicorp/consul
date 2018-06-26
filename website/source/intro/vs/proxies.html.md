---
layout: "intro"
page_title: "Consul vs. Envoy and Other Proxies"
sidebar_current: "vs-other-proxies"
description: |-
  Modern service proxies provide high-level service routing, authentication, telemetry, and more for microservice and cloud environments. Envoy is a popular and feature rich proxy. This page describes how Consul relates to proxies such as Envoy.
---

# Consul vs. Envoy and Other Proxies

Modern service proxies provide high-level service routing, authentication,
telemetry, and more for microservice and cloud environments. Envoy is
a popular and feature rich proxy.

Proxies require a rich set of configuration to operate since backend
addresses, frontend listeners, routes, filters, telemetry shipping, and
more must all be configured. Further, a modern infrastructure contains
many proxies, often one proxy per service as proxies are deployed in
a "sidecar" model next to a service. Therefore, a primary challenge of
proxies is the configuration sprawl and orchestration.

Proxies form what is referred to as the "data plane": the pathway which
data travels for network connections. Above this is the "control plane"
which provides the rules and configuration for the data plane. Proxies
typically integrate with outside solutions to provide the control plane.
For example, Envoy integrates with Consul to dynamically populate
service backend addresses.

Consul is a control plane solution. The service catalog serves as a registry
for services and their addresses and can be used to route traffic for proxies.
The Connect feature of Consul provides the TLS certificates and service
access graph, but still requires a proxy to exist in the data path. As a
control plane, Consul integrates with many data plane solutions including
Envoy, HAProxy, Nginx, and more.

Consul provides a built-in proxy written in Go. This trades performance
for ease of use: by being built-in to Consul, users of Consul can get
started with solutions such as Connect without needing to install other
software. But the built-in proxy isn't meant to compete on features or
performance with dedicated proxy solutions such as Envoy. Consul enables
third party proxies to integrate with Connect and provide the data
plane with Consul operating as the control plane.

The Connect feature of Consul operates at layer 4 by authorizing a TLS
connection to succeed or fail. Proxies provide excellent solutions to
layer 7 concerns such as path-based routing, tracing and telemetry, and
more. Consul encourages using any proxy that provides the featureset required
by the user.

Further, by supporting a pluggable data plane model, the right proxy can be
deployed as needed. For non-performance critical applications, the built-in
proxy can be used. For performance critical applications, Envoy can be used.
For some applications that may require hardware, a hardware load balancer
such an F5 appliance may be deployed. Consul provides an API for all of these
solutions to be integrated.

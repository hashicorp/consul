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
a popular and feature-rich proxy that is often
used on its own. Consul [integrates with Envoy](https://www.consul.io/docs/connect/proxies/envoy.html) to simplify its configuration. 

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

The [Consul Envoy integration](https://www.consul.io/docs/connect/proxies/envoy.html)
is currently the primary way to utilize advanced layer 7 features provided
by Consul. In addition to Envoy, Consul enables
third party proxies to integrate with Connect and provide the data
plane with Consul operating as the control plane.

Proxies provide excellent solutions to layer 7 concerns such as path-based
routing, tracing and telemetry, and more. By supporting a pluggable data plane model, the right proxy can be
deployed as needed.
For performance-critical applications or those
that utilize layer 7 functionality, Envoy can be used. For non-performance critical layer 4 applications, you can use Consul's [built-in proxy](https://www.consul.io/docs/connect/proxies/built-in.html) for convenience.

For some applications that may require hardware, a hardware load balancer
such an F5 appliance may be deployed. Consul encourages this use of the right
proxy for the scenario and treats hardware load balancers as swappable components that can be run
alongside other proxies, assuming they integrate with the [necessary APIs](https://www.consul.io/docs/connect/proxies/integrate.html)
for Connect.

---
layout: "docs"
page_title: "Connect - Observability"
sidebar_current: "docs-connect-observability"
description: |-
  This page documents the configurations necessary for L7 observability using
  Consul Connect.
---

# Observability

In order to take advantage of Connect's L7 observability features you will need
to:

- Deploy sidecar proxies that are capable of emitting metrics with each of your
  services. We have first class support for Envoy.
- Define where your proxies should send metrics that they collect.
- Define the protocols for each of your services.
- Define the upstreams for each of your services.

If you are using Envoy as your sidecar proxy, you will need to [enable
gRPC](/docs/agent/options.html#grpc_port) on your client agents. To define the
metrics destination and service protocol you may want to enable [configuration
entries](/docs/agent/options.html#config_entries) and [centralized service
configuration](/docs/agent/options.html#enable_central_service_config). If you
are using Kubernetes, the Helm chart can simpify much of the necessary
configuration, which you can learn about in the [observability
guide](https://learn.hashicorp.com/consul/getting-started-k8s/l7-observability-k8s).

### Metrics Destination

For Envoy the metrics destination can be configured in the proxy configuration
entry's `config` section.

```
kind = "proxy-defaults"
name = "global"
config {
   "envoy_dogstatsd_url": "udp://127.0.0.1:9125"
}
```

Find other possible metrics syncs in the [Connect Envoy documentation](/docs/connect/proxies/envoy.html#bootstrap-configuration).

### Service Protocol

You can specify the [service protocol](/docs/agent/config-entries/service-defaults.html#protocol)
in the `service-defaults` configuration entry. You can override it in the
[service registration](/docs/agent/services.html). By default, proxies only give
you L4 metrics. This protocol allows proxies to handle requests at the right L7
protocol and emit richer L7 metrics. It also allows proxies to make per-request
load balancing and routing decisions.

### Service Upstreams

You can set the upstream for each service using the proxy's
[`upstreams`](/docs/connect/registration/service-registration.html#upstreams)
sidecar parameter, which can be defined in a service's [sidecar
registration](/docs/connect/registration/sidecar-service.html).

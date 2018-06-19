---
layout: "intro"
page_title: "Consul vs. Istio"
sidebar_current: "vs-other-istio"
description: |-
  Istio is a platform for connecting and securing microservices. This page describes the similarities and differences between Istio and Consul.
---

# Consul vs. Istio

Istio is an open platform to connect, manage, and secure microservices.

To enable the full functionality of Istio, multiple services must
be deployed. For the control plane: Pilot, Mixer, and Citadel must be
deployed and for the data plane an Envoy sidecar is deployed. Additionally,
Istio requires a 3rd party service catalog from Kubernetes, Consul, Eureka,
or others. At a minimum, three Istio-dedicated services along with at
least one separate distributed system (in addition to Istio) must be
configured for the full functionality of Istio.

Istio is architected to work on any platform. However, the documentation
and resources for installing and configuring Istio on non-Kubernetes systems
are few and the number of moving pieces Istio requires poses a challenge for
installation, configuration, and operation.

Istio provides layer 7 features for path-based routing, traffic shaping,
load balancing, and telemetry. Access control policies can be configured
targeting both layer 7 and layer 4 properties to control access, routing,
and more based on service identity.

Consul is a single binary providing both server and client capabilities, and
includes all functionality for service catalog, configuration, TLS certificates,
authorization, and more. No additional systems need to be installed to use
Consul, although Consul optionally supports external systems such as Vault
to augment behavior. This architecture enables Consul to be easily installed
on any platform, including directly onto the machine.

Consul uses an agent-based model where each node in the cluster runs a
Consul Client. This client maintains a local cache that is efficiently updated
from servers. As a result, all secure service communication APIs respond in
microseconds and do not require any external communication. Further,
service-to-service communication continues operating even if the Consul
cluster is degraded.

The data plane for Consul is pluggable. It includes a built-in proxy with
a larger performance trade off for ease of use. But you may also use third
party proxies such as Envoy. The ability to use the right proxy for the job
allows flexible heterogeneous deployments where different proxies may be
more correct for the applications they're proxying.

Consul enforces authorization and identity to layer 4 only. We believe
service identity should be tied to layer 4, whereas layer 7 should be used
for routing, telemetry, etc. We encourge users to use the pluggable data
plane layer to use a proxy that supports the layer 7 features necessary
for the cluster. Consul will be adding more layer 7 features in the future.

Consul implements automatic TLS certificate management complete with rotation
support. Both leaf and root certificates can be rotated automatically across
a large Consul cluster with zero disruption to connections. The certificate
management system is pluggable through code change in Consul and will be
exposed as an external plugin system shortly. This enables Consul to work
with any PKI solution.

Because Consul's service connection feature "Connect" is built-in, it
inherits the operational stability of Consul. Consul has been in production
for large companies since 2014 and is known to be deployed on as many as
50,000 nodes in a single cluster.

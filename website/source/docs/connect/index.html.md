---
layout: "docs"
page_title: "Connect (Service Segmentation)"
sidebar_current: "docs-connect-index"
description: |-
  Consul Connect provides service-to-service connection authorization and encryption using mutual TLS.
---

# Connect

Consul Connect provides service-to-service connection authorization
and encryption using mutual TLS. Applications can use
[sidecar proxies](/docs/connect/proxies.html)
to automatically establish TLS connections for inbound and outbound connections
without being aware of Connect at all. Applications may also
[natively integrate with Connect](/docs/connect/native.html)
for optimal performance and security.

## How it Works

TODO

## Eliminating East-West Firewalls

East-west firewalls are the typical tool for network security in a static world.
East-west is the transfer of data from server to server within a datacenter,
versus North-south traffic which describes end user to server communications.

These firewalls wrap services with ingress/egress policies. This perimeter-based
approach is difficult to scale in a dynamic world with dozens or hundreds of
services or where machines may be frequently created or destroyed. Firewalls
create a sprawl of rules for each service instance that quickly becomes
overly difficult to maintain.

Service security in a dynamic world is best solved through service-to-service
authentication and authorization. Instead of IP-based network security,
services can be deployed to low-trust networks and rely on service-identity
based security over in-transit data encryption.

Connect enables service segmentation by securing service-to-service
communications through mutual TLS and transparent proxying on zero-trust
networks. This allows direct service communication without relying on firewalls
for east-west traffic security.

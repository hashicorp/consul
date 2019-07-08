---
layout: "docs"
page_title: "Connect - Mesh Gateways"
sidebar_current: "docs-connect-meshgateways"
description: |-
  A Mesh Gateway enables better routing of a Connect service's data to upstreams in other datacenters. This section details how to use Envoy and describes how you can plug in a gateway of your choice.
---

# Mesh Gateways

Mesh gateways enable routing of Connect traffic between different Consul datacenters. Those datacenters
can reside in different clouds or runtime environments where general interconnectivity between all services
in all datacenters isn't feasible. These gateways operate by sniffing the SNI header out of the Connect session
and then route the connection to the appropriate destination based on the server name requested. The data
within the Connect session is not decrypted by the Gateway.

![Mesh Gateway Architecture](/assets/images/mesh-gateways.png)

## Prerequisites

Each mesh gateway needs three things:

1. A local Consul agent to manage its configuration.
2. General network connectivity to all services within its local Consul datacenter.
3. General network connectivity to all mesh gateways within remote Consul datacenters.

## Modes of Operation

Each upstream of a Connect proxy can be configured to be routed through a mesh gateway. Depending on
your network, this can operate in the following modes:

* `local` - In this mode the Connect proxy makes its outbound connection to a gateway running in the
  same datacenter. That gateway is then responsible for ensuring the data gets forwarded along to
  gateways in the destination datacenter. This is the mode of operation depicted in the diagram towards
  the beginning of the page.

* `remote` - In this mode the Connect proxy makes its outbound connection to a gateway running in the
  destination datacenter. That gateway will then forward the data to the final destination service.

* `none` - In this mode, no gateway is used and a Connect proxy makes its outbound connections directly
  to the destination services.

## Configuration

TODO







TODO TODO TODO - What level of information should we put here?
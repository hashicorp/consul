---
layout: "docs"
page_title: "Required Ports"
sidebar_current: "docs-install-ports"
description: |-
  Before starting Consul it is important to have the necessary bind ports accessible.
---

# Required Ports


Consul requires up to 6 different ports to work properly, some on
TCP, UDP, or both protocols. Below we document the requirements for each
port. 

## Ports Table

Before running Consul, you should ensure the following bind ports are accessible. 


|  Use                              | Default Ports    | 
| --------------------------------- | ---------------- |
| DNS: The DNS server (TCP and UDP)              | 8600             |
| HTTP: The HTTP API (TCP Only)               | 8500             |
| HTTPS: The HTTPs API              | disabled (8501)* | 
| gRPC: The gRPC API                | disabled (8502)* | 
| LAN Serf: The Serf LAN port (TCP and UDP)      | 8301             | 
| Wan Serf: The Serf WAN port (TCP and UDP)       | 8302             |
| server: Server RPC address (TCP Only)   | 8300             | 
| Sidecar Proxy Min: Inclusive min port number to use for automatically assigned sidecar service registrations.   | 21000            | 
| Sidecar Proxy Max: Inclusive max port number to use for automatically assigned sidecar service registrations. | 21255            | 

*For `HTTPS` and `gRPC` the ports specified in the table 
are recommendations.

## Port Information

**DNS Interface** Used to resolve DNS queries. 

**HTTP API** This is used by clients to talk to the HTTP
  API.

**HTTPS API** (Optional) Is off by default, but port 8501 is a convention 
  used by various tools as the default.

**gRPC API** (Optional). Currently gRPC is
   only used to expose the xDS API to Envoy proxies. It is off by default, but port 8502 is a convention used by various tools as the default. Defaults to 8502 in `-dev` mode.

**Serf LAN** This is used to handle gossip in the LAN.
  Required by all agents. 

**Serf WAN** This is used by servers to gossip over the WAN, to
  other servers. As of Consul 0.8 the WAN join flooding feature requires
  the Serf WAN port (TCP/UDP) to be listening on both WAN and LAN interfaces. See also:
   [Consul 0.8.0 CHANGELOG](https://github.com/hashicorp/consul/blob/master/CHANGELOG.md#080-april-5-2017) and [GH-3058](https://github.com/hashicorp/consul/issues/3058)

**Server RPC** This is used by servers to handle incoming
  requests from other agents. 

Note, the default ports can be changed in the [agent configuration](/docs/agent/options.html#ports). 

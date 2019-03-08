---
layout: "docs"
page_title: "Required Ports"
sidebar_current: "docs-install-ports"
description: |-
  Before starting Consul it is important to have the necessary bind ports accessible.
---

# Required Ports

Before running Consul, you should ensure the following bind ports are accessible. 


|  Use                              | Default Ports    | 
| --------------------------------- | ---------------- |
| DNS: The DNS server               | 8600             |
| HTTP: The HTTP API                | 8500             |
| HTTPS: The HTTPs API              | disabled (8501)* | 
| gRPC: The gRPC API                | disabled (8502)* | 
| LAN Serf: The Serf LAN port.      | 8301             | 
| Wan Serf: The Serf WAN port       | 8302             |
| server: Server RPC address        | 8300             | 
| Sidecar Proxy Min: Inclusive min port number to use for automatically assigned sidecar service registrations.   | 21000            | 
| Sidecar Proxy Max: Inclusive max port number to use for automatically assigned sidecar service registrations. | 21255            | 

*For `HTTPS` and `gRPC` the ports specified in the table 
are recommendations.

Note, the default ports can be changed in the [agent configuration](/docs/agent/options.html#ports). 
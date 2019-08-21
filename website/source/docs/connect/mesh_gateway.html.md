---
layout: "docs"
page_title: "Connect - Mesh Gateways"
sidebar_current: "docs-connect-meshgateways"
description: |-
  A Mesh Gateway enables better routing of a Connect service's data to upstreams in other datacenters. This section details how to use Envoy and describes how you can plug in a gateway of your choice.
---

-> **1.6.0+:**  This feature is available in Consul versions 1.6.0 and newer.

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

Mesh gateways also require that your Consul datacenters are configured correctly:

- You'll need to use Consul version 1.6.0.
- Consul [Connect](/docs/agent/options.html#connect) must be enabled in both datacenters.
- Each of your [datacenters](/docs/agent/options.html#datacenter) must have a unique name. 
- Your datacenters must be [WAN joined](https://learn.hashicorp.com/consul/security-networking/datacenters).
- The [primary datacenter](/docs/agent/options.html#primary_datacenter) must be set to the same value in both datacenters. This specifies which datacenter is the authority for Connect certificates and is required for services in all datacenters to establish mutual TLS with each other.
- [gRPC](/docs/agent/options.html#grpc_port) must be enabled. 
- If you want to [enable gateways globally](/docs/connect/mesh_gateway.html#enabling-gateways-globally) you must enable [centralized configuration](/docs/agent/options.html#enable_central_service_config). 

## Modes of Operation

Each upstream of a Connect proxy can be configured to be routed through a mesh gateway. Depending on
your network, the proxy's connection to the gateway can happen in one of the following modes:

* `local` - In this mode the Connect proxy makes its outbound connection to a gateway running in the
  same datacenter. That gateway is then responsible for ensuring the data gets forwarded along to
  gateways in the destination datacenter. This is the mode of operation depicted in the diagram at
  the beginning of the page.

* `remote` - In this mode the Connect proxy makes its outbound connection to a gateway running in the
  destination datacenter. That gateway will then forward the data to the final destination service.

* `none` - In this mode, no gateway is used and a Connect proxy makes its outbound connections directly
  to the destination services.

## Mesh Gateway Configuration

Mesh gateways are defined very similarly to other typical services. The one exception is that a mesh gateway
service definition may contain a `Proxy.Config` entry just like a Connect proxy service to define opaque
configuration parameters useful for the actual proxy software.

## Connect Proxy Configuration

Configuring a Connect Proxy to use gateways is as simple as setting its mode of operation. This can be done
in several different places allowing for global to more fine grained control. If the gateway mode is configured
in multiple locations the order of precedence is as follows

1. Upstream Definition
2. Service Instance Definition
3. Centralized `service-defaults` configuration entry
4. Centralized `proxy-defaults` configuration entry.

### Enabling Gateways Globally

The following `proxy-defaults` configuration will enable gateways for all Connect services in the `local` mode.

```hcl
Kind = "proxy-defaults"
Name = "global"
MeshGateway {
   Mode = "local"
}
```

### Enabling Gateways Per-Service

The following `service-defaults` configuration will enable gateways for all Connect services with the name "web".

```hcl
Kind = "service-defaults"
Name = "web"
MeshGateway {
   Mode = "local"
}
```

### Enabling Gateways for a Service Instance

The following service definition will enable gateways for the service instance in the `remote` mode.

```hcl
service {
   name = "web-sidecar-proxy"
   port = 8181
   proxy {
      destination_service_name = "web"
      mesh_gateway {
         mode = "remote"
      }
      upstreams = [
         {
            destination_name = "api"
            datacenter = "secondary"
            local_bind_port = 10000
         }
      ]
   }
}
```

Or alternatively as a sidecar service:

```hcl
service {
  name = "web"
  port = 8181
  connect {
    sidecar_service {
      proxy {
        mesh_gateway {
         mode = "remote"
        }
        upstreams = [
          {
            destination_name = "api"
            datacenter = "secondary"
            local_bind_port = 10000
          }
        ]
      }
    }
  }
}
```

### Enabling Gateways for a Proxy Upstream

The following service definition will enable gateways in the `local` mode for one upstream, the `remote` mode
for a second upstream and will disable gateways for a third upstream.

```hcl
service {
   name = "web-sidecar-proxy"
   port = 8181
   proxy {
      destination_service_name = "web"
      upstreams = [
         {
            destination_name = "api"
            local_bind_port = 10000
            mesh_gateway {
               mode = "remote"
            }
         },
         {
            destination_name = "db"
            local_bind_port = 10001
            mesh_gateway {
               mode = "local"
            }
         },
         {
            destination_name = "logging"
            local_bind_port = 10002
            mesh_gateway {
               mode = "none"
            }
         },
      ]
   }
}
```

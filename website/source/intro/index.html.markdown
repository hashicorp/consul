---
layout: "intro"
page_title: "Introduction"
sidebar_current: "what"
description: |-
  Welcome to the intro guide to Consul! This guide is the best place to start with Consul. We cover what Consul is, what problems it can solve, how it compares to existing software, and a quick start for using Consul. If you are already familiar with the basics of Consul, the documentation provides more of a reference for all available features.
---

# Introduction to Consul

Welcome to the intro guide to Consul! This guide is the best place to start
with Consul. We cover what Consul is, what problems it can solve, how it compares
to existing software, and a quick start for using Consul. If you are already familiar
with the basics of Consul, the [documentation](/docs/index.html) provides more
of a reference for all available features.

## What is Consul?

Consul has multiple components, but as a whole, it is a tool for discovering
and configuring services in your infrastructure. It provides several
key features:

* **Service Discovery**: Clients of Consul can _provide_ a service, such as
  `api` or `mysql`, and other clients can use Consul to _discover_ providers
  of a given service. Using either DNS or HTTP, applications can easily find
  the services they depend upon.

* **Health Checking**: Consul clients can provide any number of health checks,
  either associated with a given service ("is the webserver returning 200 OK"), or
  with the local node ("is memory utilization below 90%"). This information can be
  used by an operator to monitor cluster health, and it is used by the service
  discovery components to route traffic away from unhealthy hosts.

* **Key/Value Store**: Applications can make use of Consul's hierarchical key/value
  store for any number of purposes including: dynamic configuration, feature flagging,
  coordination, leader election, etc. The simple HTTP API makes it easy to use.

* **Multi Datacenter**: Consul supports multiple datacenters out of the box. This
  means users of Consul do not have to worry about building additional layers of
  abstraction to grow to multiple regions.

Consul is designed to be friendly to both the DevOps community and
application developers, making it perfect for modern, elastic infrastructures.

## Basic Architecture of Consul

Consul is a distributed, highly available system. There is an
[in-depth architecture overview](/docs/internals/architecture.html) available,
but this section will cover the basics so you can get an understanding
of how Consul works. This section will purposely omit details to quickly
provide an overview of the architecture.

Every node that provides services to Consul runs a _Consul agent_. Running
an agent is not required for discovering other services or getting/setting
key/value data. The agent is responsible for health checking the services
on the node as well as the node itself.

The agents talk to one or more _Consul servers_. The Consul servers are
where data is stored and replicated. The servers themselves elect a leader.
While Consul can function with one server, 3 to 5 is recommended to avoid
data loss scenarios. A cluster of Consul servers is recommended for each
datacenter.

Components of your infrastructure that need to discover other services
or nodes can query any of the Consul servers _or_ any of the Consul agents.
The agents forward queries to the servers automatically.

Each datacenter runs a cluster of Consul servers. When a cross-datacenter
service discovery or configuration request is made, the local Consul servers
forward the request to the remote datacenter and return the result.

## Next Steps

See the page on [how Consul compares to other software](/intro/vs/index.html)
to see how it fits into your existing infrastructure. Or continue onwards with
the [getting started guide](/intro/getting-started/install.html) to get
Consul up and running and see how it works.

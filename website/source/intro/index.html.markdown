---
layout: "intro"
page_title: "Introduction"
sidebar_current: "what"
---

# Introduction to Consul

Welcome to the intro guide to Consul! This guide is a the best place to start
with Consul. We cover what Consul is, what problems it can solve, how it compares
to existing software, and a quick start for using Consul. If you are already familiar
with the basics of Consul, the [documentation](/docs/index.html) provides more
of a reference for all available features.

## What is Consul?

Consul has multiple components, but as a whole, it is tool for managing
and coordinating infrastructure. It provides several key features:

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
  store for any number of purposes including dynamic configuration, feature flagging,
  coordination, leader election, etc. The simple HTTP API makes it easy to use.

* **Multi Datacenter**: Consul supports multiple datacenters out of the box. This
  means users of Consul do not have to worry about building additional layers of
  abstraction to grow to multiple regions.

See the page on [how Consul compares to other software](/intro/vs/index.html)
to see just how it fits into your existing infrastructure. Or continue onwards with
the [getting started guide](/intro/getting-started/install.html) to get
Consul up and running and see how it works.

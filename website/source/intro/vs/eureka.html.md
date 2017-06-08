---
layout: "intro"
page_title: "Consul vs. Eureka"
sidebar_current: "vs-other-eureka"
description: |-
  Eureka is a service discovery tool that provides a best effort registry and discovery service. It uses central servers and clients which are typically natively integrated with SDKs. Consul provides a super set of features, such as health checking, key/value storage, ACLs, and multi-datacenter awareness.
---

# Consul vs. Eureka

Eureka is a service discovery tool. The architecture is primarily client/server,
with a set of Eureka servers per datacenter, usually one per availability zone.
Typically clients of Eureka use an embedded SDK to register and discover services.
For clients that are not natively integrated, a sidecar such as Ribbon is used
to transparently discover services via Eureka.

Eureka provides a weakly consistent view of services, using best effort replication.
When a client registers with a server, that server will make an attempt to replicate
to the other servers but provides no guarantee. Service registrations have a short
Time-To-Live (TTL), requiring clients to heartbeat with the servers. Unhealthy services
or nodes will stop heartbeating, causing them to timeout and be removed from the registry.
Discovery requests can route to any service, which can serve stale or missing data due to
the best effort replication. This simplified model allows for easy cluster administration
and high scalability.

Consul provides a super set of features, including richer health checking, key/value store,
and multi-datacenter awareness. Consul requires a set of servers in each datacenter, along
with an agent on each client, similar to using a sidecar like Ribbon. The Consul agent allows
most applications to be Consul unaware, performing the service registration via configuration
files and discovery via DNS or load balancer sidecars.

Consul provides a strong consistency guarantee, since servers replicate state using the
[Raft protocol](/docs/internals/consensus.html). Consul supports a rich set of health checks
including TCP, HTTP, Nagios/Sensu compatible scripts, or TTL based like Eureka. Client nodes
participate in a [gossip based health check](/docs/internals/gossip.html), which distributes
the work of health checking, unlike centralized heartbeating which becomes a scalability challenge.
Discovery requests are routed to the elected Consul leader which allows them to be strongly consistent
by default. Clients that allow for stale reads enable any server to process their request allowing
for linear scalability like Eureka.

The strongly consistent nature of Consul means it can be used as a locking service for leader
elections and cluster coordination. Eureka does not provide similar guarantees, and typically
requires running ZooKeeper for services that need to perform coordination or have stronger
consistency needs.

Consul provides a toolkit of features needed to support a service oriented architecture.
This includes service discovery, but also rich health checking, locking, Key/Value, multi-datacenter
federation, an event system, and ACLs. Both Consul and the ecosystem of tools like consul-template
and envconsul try to minimize application changes required to integration, to avoid needing
native integration via SDKs. Eureka is part of a larger Netflix OSS suite, which expects applications
to be relatively homogeneous and tightly integrated. As a result, Eureka only solves a limited
subset of problems, expecting other tools such as ZooKeeper to be used alongside.


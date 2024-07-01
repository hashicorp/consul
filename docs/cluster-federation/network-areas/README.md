# Network Areas (ENTERPRISE ONLY)

## Overview

### Description and Background
Network areas define pairwise gossip pools over the WAN between Consul datacenters. Gossip traffic between Consul servers in an area is done over the server RPC port, and uses TCP connections. This is unlike Consul's primary WAN and LAN pools, which primarily rely on UDP-based probes.

TCP was used because it allows configuration with TLS certificates. The TLS encryption will be used if [configured](https://developer.hashicorp.com/consul/docs/security/encryption#rpc-encryption-with-tls) and [enabled](https://developer.hashicorp.com/consul/api-docs/operator/area#usetls). Because gossip connects every node in that pool, there is also a connection between servers from the same datacenter. Connections between servers from the same datacenter default to using TLS if it is configured. The overhead of the TCP protocol is limited since there is a small number of servers in an area. Note that the symmetric keys used by Consul's WAN and LAN pools are not used to encrypt traffic from network areas.

In versions of Consul prior to v1.8, network areas would establish a new TCP connection for every network area message. This was then substituted by connection pooling, where each server will maintain a TCP connection to every server in the network area. However, note that when a server in a version > `v1.8.0` dials a server on an older version, it will fall-back to the old connection-per-message behavior.


### Key Components
* Consul Enterprise:
  * Network areas are created via Consul Enterprise's APIs and are persisted on server agents. 
  * Consul servers regularly reconcile the network area gossip pools they are in against the expected network areas registered in the state store.
* Memberlist
  * Memberlist manages the list of nodes in the gossip pool by implementing the failure detection mechanism from the [Lifeguard paper](https://arxiv.org/pdf/1707.00788.pdf).
* Serf
  * Serf is the interface that drives node health updates in Consul.
  * When memberlist updates the status of a member then Serf will send corresponding events to Consul. Based on these events Consul then updates the catalog. 


### Telemetry

`consul.area.connections.outgoing` - Tracks outbound network area connections. When both the dialing and dialed servers support pooling then this metric tracks open connections in the pool. When connection pooling is not supported this metric tracks connections opened over time.


## Implementation Overview

### Area creation
Network areas are created with requests to the `/operator/area` HTTP endpoint. The area defined in the request is then commited through RAFT and persisted on Consul servers.

Every Consul Enterprise server maintains a reconciliation routine where every 30s it will query the list of areas in the state store, and then join or leave gossip pools to reflect that state.

Joining a network area pool involves:
1. Setting memberlist and Serf configuration. 
   * Prior to Consul `v1.8.11` and `v1.9.5`, network areas were configured with memberlist's [DefaultWANConfig](https://github.com/hashicorp/memberlist/blob/838073fef1a4e1f6cb702a57a8075304098b1c31/config.go#L315). This was then updated to instead use the server's [gossip_wan](https://developer.hashicorp.com/consul/docs/agent/config/config-files#gossip_wan) configuration, which falls back to the DefaultWANConfig if it was not specified.
   * As of Consul `v1.8.11`/`v1.9.5` it is not possible to tune gossip communication on a per-area basis.

2. Update the server's gossip network, which keeps track of network areas that the server is a part of. This gossip network is also used to dispatch incoming **gossip** connections to handlers for the appropriate area.

3. Update the server's router, which enables forwarding RPC requests to remote servers in the area. These are cross-datacenters requests made to Consul's public APIs, such as to list service health.


### Area gossip

When a network area is added to a server's gossip network, Consul configures memberlist with a custom transport that implements memberlist's [Transport interface](https://github.com/hashicorp/memberlist/blob/619135cdd9e5dda8c12f8ceef39bdade4f5899b6/transport.go#L28). 

The primary difference from memberlists's default [NetTransport](https://github.com/hashicorp/memberlist/blob/619135cdd9e5dda8c12f8ceef39bdade4f5899b6/net_transport.go#L42), is that the area transport **exclusively** ingests from and writes to TCP connections multiplexed over the RPC port. For example, when writing to an address the area transport will acquire a connection from the pool and then write to that connection. This is unlike `NetTransport`'s `WriteTo` implementation, which sends packets to a UDP listener. 

Since TCP connections are used, establishing a connection is subject to a dial timeout. The dial timeout was initially set to 10s until `v1.8.0`, where the move to connection pooling lowered it to 100ms. This 100ms timeout was insufficient for WAN connections over long distances and later reverted to 10s in versions `v1.8.11`/`v1.9.5`.  As of Consul `v1.8.11`, this dial timeout is not configurable and applies to all network areas.

When a connection is established, inbound gossip requests are handled by the area transport. This handler then passes the request off to memberlist via the [packet](https://github.com/hashicorp/memberlist/blob/838073fef1a4e1f6cb702a57a8075304098b1c31/transport.go#L49) or [stream](https://github.com/hashicorp/memberlist/blob/838073fef1a4e1f6cb702a57a8075304098b1c31/transport.go#L61) channels.

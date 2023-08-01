# FAQ

This section addresses some frequently asked questions about Consul's architecture.

### How does eventually-consistent gossip relate to the Raft consensus protocol?

When you query Consul for information about a service, such as via the [DNS interface](https://developer.hashicorp.com/consul/docs/services/discovery/dns-overview), the agent will always make an internal RPC request to a Consul server that will query the consistent state store. Even though an agent might learn that another agent is down via gossip, that won't be reflected in service discovery until the current Raft leader server perceives that through gossip and updates the catalog using Raft. You can see an example of where these layers are plumbed together here - https://github.com/hashicorp/consul/blob/v1.0.5/agent/consul/leader.go#L559-L602.

## Why does a blocking query sometimes return with identical results?

Consul's [blocking queries](https://developer.hashicorp.com/consul/api-docs#blocking-queries) make a best-effort attempt to wait for new information, but they may return the same results as the initial query under some circumstances. First, queries are limited to 10 minutes max, so if they time out they will return. Second, due to Consul's prefix-based internal immutable radix tree indexing, there may be modifications to higher-level nodes in the radix tree that cause spurious wakeups. In particular, waiting on things that do not exist is not very efficient, but not very expensive for Consul to serve, so we opted to keep the code complexity low and not try to optimize for that case. You can see the common handler that implements the blocking query logic here - https://github.com/hashicorp/consul/blob/v1.0.5/agent/consul/rpc.go#L361-L439. For more on the immutable radix tree implementation, see https://github.com/hashicorp/go-immutable-radix/ and https://github.com/hashicorp/go-memdb, and the general support for "watches".

### Do the client agents store any key/value entries?

No. These are always fetched via an internal RPC request to a Consul server. The agent doesn't do any caching, and if you want to be able to fetch these values even if there's no cluster leader, then you can use a more relaxed [consistency mode](https://developer.hashicorp.com/consul/api-docs/features/consistency). You can see an example where the `/v1/kv/<key>` HTTP endpoint on the agent makes an internal RPC call here - https://github.com/hashicorp/consul/blob/v1.0.5/agent/kvs_endpoint.go#L56-L90.

### I don't want to run a Consul agent on every node, can I just run servers with a load balancer in front?

We strongly recommend running the Consul agent on each node in a cluster. Even the key/value store benefits from having agents on each node. For example, when you lock a key it's done through a [session](https://developer.hashicorp.com/consul/docs/dynamic-app-config/sessions), which has a lifetime that's by default tied to the health of the agent as determined by Consul's gossip-based distributed failure detector. If the agent dies, the session will be released automatically, allowing some other process to quickly see that and obtain the lock without having to wait for an open-ended TTL to expire. If you are using Consul's service discovery features, the local agent runs the health checks for each service registered on that node and only needs to send edge-triggered updates to the Consul servers (because gossip will determine if the agent itself dies). Most attempts to avoid running an agent on each node will face solving issues that are already solved by Consul's design if the agent is deployed as intended.

For cases where you really cannot run an agent alongside a service, such as for monitoring an [external service](https://developer.hashicorp.com/consul/tutorials/developer-discovery/service-registration-external-services), there's a companion project called the [Consul External Service Monitor](https://github.com/hashicorp/consul-esm) that may help.

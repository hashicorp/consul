# xDS Server

The xDS Server is a gRPC service that implements [xDS] and handles requests from
an [envoy proxy].

[xDS]: https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
[envoy proxy]: https://www.consul.io/docs/connect/proxies/envoy


## Authorization

Requests to the xDS server are authorized based on an assumption of how
`proxycfg.ConfigSnapshot` are constructed. Most interfaces (HTTP, DNS, RPC)
authorize requests by authorizing the data in the response, or by filtering
out data that the requester is not authorized to view. The xDS server authorizes
requests by looking at the proxy ID in the request and ensuring the ACL token has
`service:write` access to either the destination service (for kind=ConnectProxy), or
the gateway service (for other kinds).

This authorization strategy requires that [agent/proxycfg] only fetches data using a
token with the same permissions, and that it only stores data by proxy ID. We assume
that any data in the snapshot was already filtered, which allows this authorization to
only perform a shallow check against the proxy ID.

[agent/proxycfg]: https://github.com/hashicorp/consul/blob/main/agent/proxycfg

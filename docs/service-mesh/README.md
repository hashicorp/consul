# Service Mesh (Connect)

- call out: envoy/proxy is the data plane, Consul is the control plane
- [xDS Server] - a gRPC service that implements [xDS] and handles requests from an [envoy proxy].
- [agent/proxycfg]
- [Certificate Authority](./ca) for issuing TLS certs for services and client agents
- command/connect/envoy - bootstrapping and running envoy
- command/connect/proxy - built-in proxy that is dev-only and not supported 
  for production.
- `connect/` - "Native" service mesh

[xDS Server]: ./xds.md
[xDS]: https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
[envoy proxy]: https://www.consul.io/docs/connect/proxies/envoy
[agent/proxycfg]: https://github.com/hashicorp/consul/blob/main/agent/proxycfg

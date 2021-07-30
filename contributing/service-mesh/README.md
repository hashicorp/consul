# Service Mesh (Connect)

- call out: envoy/proxy is the data plane, Consul is the control plane
- agent/xds - gRPC service that implements
  [xDS](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [agent/proxycfg](https://github.com/hashicorp/consul/blob/main/agent/proxycfg/proxycfg.go)
- CA Manager - certificate authority
- command/connect/envoy - bootstrapping and running envoy
- command/connect/proxy - built-in proxy that is dev-only and not supported 
  for production.
- `connect/` - "Native" service mesh


# Service Mesh (Connect)

This section is a work in progress.


- [Certificate Authority](./ca) for issuing TLS certs for services and client agents
- call out: envoy/proxy is the data plane, Consul is the control plane
- agent/xds - gRPC service that implements
  [xDS](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol)
- [agent/proxycfg](https://github.com/hashicorp/consul/blob/master/agent/proxycfg/proxycfg.go)
- command/connect/envoy - bootstrapping and running envoy
- command/connect/proxy - built-in proxy that is dev-only and not supported 
  for production.
- `connect/` - "Native" service mesh


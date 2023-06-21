# Configuration Entries
## Merging Service Config
Merging data from defaults in config entries is roughly a two-step process:
1. Flatten central configuration for proxies at a Consul server. Resolving config is done on behalf of a specific service, and this merge considers multiple config entries, such as proxy-defaults and service-defaults.
2. Merge flattened central defaults into sidecar proxy registrations. This occurs at a client agent in the agentful case, or on a server in the agentless case.

Any time the centralized defaults are updated, internal watches will fire and their data will be merged into the registration for the given sidecar proxy. You can ensure that a config entry has applied by querying the service:
* Agentful: `/v1/agent/service/:service_id`
* Agentless: `/v1/health/connect/:service_name?merge-central-config`

### Agentless
The agentless flow relies on [configentry.MergeNodeServiceWithCentralConfig](https://github.com/hashicorp/consul/blob/0402fd23a349513d3e8d137ddbffcdefcc89838b/agent/configentry/merge_service_config.go) to merge default configuration. This helper is called both when bootstrapping a new dataplane proxy and when an Envoy proxy first dials the `xds` server on a Consul server.

This merge does not directly update the proxy registration in the catalog. The merged proxy registration is passed to the consuming `proxycfg` package.

### Agentful
The [agent.ServiceManager](https://github.com/hashicorp/consul/blob/0402fd23a349513d3e8d137ddbffcdefcc89838b/agent/service_manager.go#LL18) is responsible for ensuring that central configuration is merged down into proxy registrations on **client agents**.  Any time a proxy is registered or updated, the service manager will set up a watch through the agent cache to fetch the central configuration.

When the central configuration is fetched, the configuration is merged and persisted to the agent state in [serviceConfigWatch.handleUpdate](https://github.com/hashicorp/consul/blob/0402fd23a349513d3e8d137ddbffcdefcc89838b/agent/service_manager.go#L256).

## Consuming Central Config
Once centralized defaults have been merged into service registrations, the data becomes available to consume by the `proxycfg` package. This package assembles the necessary Consul data to generate Envoy configuration for a given proxy.

The `proxycfg` package does not need explicit watches for the service or proxy-defaults that apply to a sidecar proxy because of how these defaults are merged into proxy registrations. When defaults change, a fresh `proxycfg` snapshot is built based on the latest registration.


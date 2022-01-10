# Health Checks

This section is still a work in progress.

[agent/checks](https://github.com/hashicorp/consul/tree/main/agent/checks) contains the logic for
performing active [health checking](https://www.consul.io/docs/agent/checks.html).


## Check Registration flows

There are many paths to register a check. Many of these use different struct
types, so to properly validate and convert a check, all of these paths must
be reviewed and tested.

1. API [/v1/catalog/register](https://www.consul.io/api-docs/catalog#register-entity) - the `Checks`
   field on `structs.RegisterRequest`. The entrypoint is `CatalogRegister` in
   [agent/catalog_endpoint.go].
2. API [/v1/agent/check/register](https://www.consul.io/api-docs/agent/check#register-check) - the entrypoint
   is `AgentRegisterCheck` in [agent/agent_endpoint.go]
3. API [/v1/agent/service/register](https://www.consul.io/api-docs/agent/service#register-service) -
   the `Check` or `Checks` fields on `ServiceDefinition`. The entrypoint is `AgentRegisterService`
   in [agent/agent_endpoint.go].
4. Config [Checks](https://www.consul.io/docs/discovery/checks) - the `Checks` and `Check` fields
   on `config.Config` in [agent/config/config.go].
5. Config [Service.Checks](https://www.consul.io/docs/discovery/services) - the
   `Checks` and `Check` fields on `ServiceDefinition` in [agent/config/config.go].
6. CLI [consul services register](https://www.consul.io/commands/services/register) - the
   `Checks` and `Check` fields on `api.AgentServiceRegistration`. The entrypoint is
   `ServicesFromFiles` in [command/services/config.go].


[agent/catalog_endpoint.go]: https://github.com/hashicorp/consul/blob/main/agent/catalog_endpoint.go
[agent/agent_endpoint.go]: https://github.com/hashicorp/consul/blob/main/agent/agent_endpoint.go
[agent/config/config.go]: https://github.com/hashicorp/consul/blob/main/agent/config/config.go
[command/services/config.go]: https://github.com/hashicorp/consul/blob/main/command/services/config.go

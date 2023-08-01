# Health Checks

This section is still a work in progress.

[agent/checks](https://github.com/hashicorp/consul/tree/main/agent/checks) contains the logic for
performing active [health checking](https://developer.hashicorp.com/consul/docs/services/usage/checks).


## Check Registration flows

There are many paths to register a check. Many of these use different struct
types, so to properly validate and convert a check, all of these paths must
be reviewed and tested.

1. API [/v1/catalog/register](https://developer.hashicorp.com/consul/api-docs/catalog#register-entity) - the `Checks`
   field on `structs.RegisterRequest`. The entrypoint is `CatalogRegister` in
   [agent/catalog_endpoint.go].
2. API [/v1/agent/check/register](https://developer.hashicorp.com/consul/api-docs/agent/check#register-check) - the entrypoint
   is `AgentRegisterCheck` in [agent/agent_endpoint.go]
3. API [/v1/agent/service/register](https://developer.hashicorp.com/consul/api-docs/agent/service#register-service) -
   the `Check` or `Checks` fields on `ServiceDefinition`. The entrypoint is `AgentRegisterService`
   in [agent/agent_endpoint.go].
4. Config [Checks](https://developer.hashicorp.com/consul/docs/services/usage/checks) - the `Checks` and `Check` fields
   on `config.Config` in [agent/config/config.go].
5. Config [Service.Checks](https://developer.hashicorp.com/consul/docs/services/usage/register-services-checks) - the
   `Checks` and `Check` fields on `ServiceDefinition` in [agent/config/config.go].
6. The returned fields of `ServiceDefinition` in [agent/config/builder.go].
7. CLI [consul services register](https://developer.hashicorp.com/consul/commands/services/register) - the
   `Checks` and `Check` fields on `api.AgentServiceRegistration`. The entrypoint is
   `ServicesFromFiles` in [command/services/config.go].
8. API [/v1/txn](https://developer.hashicorp.com/consul/api-docs/txn) - the `Transaction` API allows for registering a check.


[agent/catalog_endpoint.go]: https://github.com/hashicorp/consul/blob/main/agent/catalog_endpoint.go
[agent/agent_endpoint.go]: https://github.com/hashicorp/consul/blob/main/agent/agent_endpoint.go
[agent/config/config.go]: https://github.com/hashicorp/consul/blob/main/agent/config/config.go
[command/services/config.go]: https://github.com/hashicorp/consul/blob/main/command/services/config.go

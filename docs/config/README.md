# Agent Configuration

The [Agent Configuration] is the primary mechanism for configuring Consul. Agent
Configuration also allows for specifying [Config Entries], [Services], and [Checks] that
will be loaded when the agent starts.

Most configuration comes from [hcl] or `json` files, but some configuration can also be
specified using command line flags, and some can be loaded with [Auto-Config].

See also the [checklist for adding a new field] to the configuration.

[hcl]: https://github.com/hashicorp/hcl/tree/hcl1
[Agent Configuration]: https://developer.hashicorp.com/consul/docs/agent/config
[checklist for adding a new field]: ./checklist-adding-config-fields.md
[Auto-Config]: #auto-config
[Config Entries]: https://developer.hashicorp.com/consul/docs/agent/config/config-files#config_entries
[Services]: https://developer.hashicorp.com/consul/docs/services/services
[Checks]: https://developer.hashicorp.com/consul/docs/services/usage/register-services-checks


## Code

The Agent Configuration is implemented in [agent/config], and the primary entrypoint is
[Load]. Config loading is performed in phases:

1. Command line flags are used to create a `config.LoadOpts` and passed to `Load`.
2. `Load` reads all the config files and builds an ordered list of `config.Source`.
3. Each `config.Source` is read to produce a `config.Config`.
4. Each `config.Config` is merged ontop the previous.
5. A `config.RuntimeConfig` is produced from the merged `config.Config`
6. The `config.RuntimeConfig` is validated.
7. Finally a result is returned with the `RuntimeConfig` and any warnings, or an error.

[agent/config]: https://github.com/hashicorp/consul/tree/main/agent/config
[Load]: https://pkg.go.dev/github.com/hashicorp/consul/agent/config#Load

If [Auto-Config] is enabled, when it receives the config from the server, the
entire process is repeated a second time with the addition config provided as another
`config.Source`.

Default values can be specified in one of the [default sources] or set when
converting from `Config` to `RuntimeConfig` in [builder.build]. Hopefully in the future we
should remove one of those ways of setting default values.

[default sources]: https://github.com/hashicorp/consul/blob/main/agent/config/default.go
[builder.build]: https://github.com/hashicorp/consul/blob/main/agent/config/builder.go

## Auto-Config

Auto-Config is enabled by the [auto_config] field in an Agent Configuration file. It is
implemented in a couple packages.

* the server RPC endpoint is in [agent/consul/auto_config_endpoint.go]
* the client that receives and applies the config is implemented in [agent/auto-config]

[auto_config]: https://developer.hashicorp.com/consul/docs/agent/config/config-files#auto_config
[agent/consul/auto_config_endpoint.go]: https://github.com/hashicorp/consul/blob/main/agent/consul/auto_config_endpoint.go
[agent/auto-config]: https://github.com/hashicorp/consul/tree/main/agent/auto-config

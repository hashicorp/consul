---
layout: "docs"
page_title: "Configuration Entry Definitions"
sidebar_current: "docs-agent-cfg_entries"
description: |-
  Consul allows storing configuration entries centrally to be used as defaults for configuring other aspects of Consul.
---

# Configuration Entries

Configuration entries can be created to provide cluster-wide defaults for various aspects of Consul. Every configuration
entry has at least two fields: `Kind` and `Name`. Those two fields are used to uniquely identify a configuration entry.
When put into configuration files, configuration entries can be specified as HCL or JSON objects.

Example:

```hcl
Kind = "<supported kind>"
Name = "<name of entry>"
```

The two supported `Kind` configuration entries are detailed below.

## Configuration Entry Kinds

### Proxy Defaults - `proxy-defaults`

Proxy defaults allow for configuring global config defaults across all services for Connect proxy configuration. Currently,
only one global entry is supported.

```hcl
Kind = "proxy-defaults"
Name = "global"
Config {
   proxy_specific_value = "foo"
}
```

* `Kind` - Must be set to `proxy-defaults`

* `Name` - Must be set to `global`

* `Config` - An arbitrary map of configuration values used by Connect proxies. See

#### Proxy Configuration References

* [Consul's Builtin Proxy](/docs/connect/configuration.html#built-in-proxy-options)
* [Envoy](/docs/connect/proxies/envoy.html#bootstrap-configuration)

### Service Defaults - `service-defaults`

Service defaults control default global values for a service, such as its protocol.

```hcl
Kind = "service-defaults"
Name = "web"
Protocol = "http"
```

* `Kind` - Must be set to `service-defaults`

* `Name` - Set to the name of the service being configured.

* `Protocol` - Sets the protocol of the service. This is used by Connect proxies for things like observability features.

## Applying Configuration Entries

There are two ways to introduce new configuration entries to Consul. The first way is to use either the [API](/api/config.html) or [CLI](/docs/commands/config.html) to manage
them in a running cluster. The second way is by placing inlined configuration entry definitions into the Consul server's
[configuration file](/docs/agent/options.html#config_entries_bootstrap).

### Managing Configuration Entries with the Consul CLI

TODO INCOMING

### Bootstrapping From A Configuration File

Configuration entries can be bootstrapped by putting them into all of the Consul server's [configuration files](/docs/agent/options.html#config_entries_bootstrap).
Each entry is embedded inline within the configuration and when that server gains leadership it will attempt to initialize that
configuration entry with the desired values if it does not exist. If a configuration entry with the same kind and name already exists
nothing will be done for that entry.


## Using Configuration Entries For Service Defaults

When the agent is [configured](/docs/agent/options.html#enable_central_service_config) to enable central service configurations,
it will look for service configuration defaults that match a registering service instance. If it finds any, the agent will merge
those defaults with the service instance configuration. This allows for things like service protocol or proxy configuration to
be defined globally and inherited by any affected service registrations.

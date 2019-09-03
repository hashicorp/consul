---
layout: "docs"
page_title: "Configuration Entry Definitions"
sidebar_current: "docs-agent-cfg_entries"
description: |-
  Consul allows storing configuration entries centrally to be used as defaults for configuring other aspects of Consul.
---

# Configuration Entries

Configuration entries can be created to provide cluster-wide defaults for
various aspects of Consul. Every configuration entry has at least two fields:
`Kind` and `Name`. Those two fields are used to uniquely identify a
configuration entry. When put into configuration files, configuration entries
can be specified as HCL or JSON objects using either `snake_case` or `CamelCase`
for key names.

Example:

```hcl
Kind = "<supported kind>"
Name = "<name of entry>"
```

The supported `Kind` names for configuration entries are:

* [`service-router`](/docs/agent/config-entries/service-router.html) - defines
where to send layer 7 traffic based on the HTTP route

* [`service-splitter`](/docs/agent/config-entries/service-splitter.html) - defines
how to divide requests for a single HTTP route based on percentages

* [`service-resolver`](/docs/agent/config-entries/service-resolver.html) - matches
service instances with a specific Connect upstream discovery requests

* [`service-defaults`](/docs/agent/config-entries/service-defaults.html) - configures
defaults for all the instances of a given service

* [`proxy-defaults`](/docs/agent/config-entries/proxy-defaults.html) - controls
proxy configuration

## Managing Configuration Entries

Configuration entries should be managed with the Consul
[CLI](/docs/commands/config.html) or [API](/api/config.html). Additionally, as a
convenience for initial cluster bootstrapping, configuration entries can be
specified in all of the Consul servers's
[configuration files](/docs/agent/options.html#config_entries_bootstrap)

### Managing Configuration Entries with the CLI

#### Creating or Updating a Configuration Entry

The [`consul config write`](/docs/commands/config/write.html) command is used to
create and update configuration entries. This command will load either a JSON or
HCL file holding the configuration entry definition and then will push this
configuration to Consul.

Example HCL Configuration File - `proxy-defaults.hcl`:

```hcl
Kind = "proxy-defaults"
Name = "global"
Config {
   local_connect_timeout_ms = 1000
   handshake_timeout_ms = 10000
}
```

Then to apply this configuration, run:

```bash
$ consul config write proxy-defaults.hcl
```

If you need to make changes to a configuration entry, simple edit that file and
then rerun the command. This command will not output anything unless there is an
error in applying the configuration entry. The `write` command also supports a
`-cas` option to enable performing a compare-and-swap operation to prevent
overwriting other unknown modifications.

#### Reading a Configuration Entry

The [`consul config read`](/docs/commands/config/read.html) command is used to
read the current value of a configuration entry. The configuration entry will be
displayed in JSON form which is how its transmitted between the CLI client and
Consul's HTTP API.

Example:

```bash
$ consul config read -kind service-defaults -name web
{
   "Kind": "service-defaults",
   "Name": "web",
   "Protocol": "http"
}
```

#### Listing Configuration Entries

The [`consul config list`](/docs/commands/config/list.html) command is used to
list out all the configuration entries for a given kind.

Example:

```bash
$ consul config list -kind service-defaults
web
api
db
```


#### Deleting Configuration Entries

The [`consul config delete`](/docs/commands/config/delete.html) command is used
to delete an entry by specifying both its `kind` and `name`.

Example:

```bash
$ consul config delete -kind service-defaults -name web
```

This command will not output anything when the deletion is successful.

### Bootstrapping From A Configuration File


Configuration entries can be bootstrapped by adding them inline to each Consul
server’s configuration file. When a server gains leadership, it will attempt to
initialize the configuration entries. If a configuration entry does not already
exist outside of the servers configuration, then it will create it. If a
configuration entry does exist, that matches both `kind` and `name`, then the
server will do nothing.


## Using Configuration Entries For Service Defaults

When the agent is
[configured](/docs/agent/options.html#enable_central_service_config) to enable
central service configurations, it will look for service configuration defaults
that match a registering service instance. If it finds any, the agent will merge
those defaults with the service instance configuration. This allows for things
like service protocol or proxy configuration to be defined globally and
inherited by any affected service registrations.

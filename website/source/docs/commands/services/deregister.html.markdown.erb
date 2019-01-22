---
layout: "docs"
page_title: "Commands: Services Deregister"
sidebar_current: "docs-commands-services-deregister"
---

# Consul Agent Service Deregistration

Command: `consul services deregister`

The `services deregister` command deregisters a service with the local agent.
Note that this command can only deregister services that were registered
with the agent specified (defaults to the local agent) and is meant to
be paired with `services register`.

This is just one method for service deregistration. If the service was
registered with a configuration file, then deleting that file and
[reloading](/docs/commands/reload.html) Consul is the correct method to
deregister. See [Service Definition](/docs/agent/services.html) for more
information about registering services generally.

## Usage

Usage: `consul services deregister [options] [FILE...]`

This command can deregister either a single service using the `-id` flag
documented below, or one or more services using service definition files
in HCL or JSON format.
This flexibility makes it easy to pair the command with the
`services register` command since the argument syntax is the same.

#### API Options

<%= partial "docs/commands/http_api_options_client" %>

#### Service Deregistration Flags

The flags below should only be set if _no arguments_ are given. If no
arguments are given, the flags below can be used to deregister a single
service.

* `-id` - The ID of the service.

## Examples

To deregister by ID:

```text
$ consul services deregister -id=web
```

To deregister from a configuration file:

```text
$ cat web.json
{
  "Service": {
    "Name": "web"
  }
}

$ consul services deregister web.json
```

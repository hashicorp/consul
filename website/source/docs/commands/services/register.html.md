---
layout: "docs"
page_title: "Commands: Services Register"
sidebar_current: "docs-commands-services-register"
---

# Consul Agent Service Registration

Command: `consul services register`

The `services register` command registers a service with the local agent.
This command returns after registration and must be paired with explicit
service deregistration. This command simplifies service registration from
scripts, in dev mode, etc.

This is just one method of service registration. Services can also be
registered by placing a [service definition](/docs/agent/services.html)
in the Consul agent configuration directory and issuing a
[reload](/docs/commands/reload.html). This approach is easiest for
configuration management systems that other systems that have access to
the configuration directory. Clients may also use the
[HTTP API](/api/agent/service.html) directly.

## Usage

Usage: `consul services register [options] [FILE...]`

This command can register either a single service using flags documented
below, or one or more services using service definition files in HCL
or JSON format. The service is registered against the specified Consul
agent (defaults to the local agent). This agent will execute all registered
health checks.

This command returns after registration succeeds. It must be paired with
a deregistration command or API call to remove the service. To ensure that
services are properly deregistered, it is **highly recommended** that
a check is created with the
[`DeregisterCriticalServiceAfter`](/api/agent/check.html#deregistercriticalserviceafter)
configuration set. This will ensure that even if deregistration failed for
any reason, the agent will automatically deregister the service instance after
it is unhealthy for the specified period of time.

Registered services are persisted in the agent state directory. If the
state directory remains unmodified, registered services will persist across
restarts.

~> **Warning for Consul operators:** The Consul agent persists registered
services in the local state directory. If this state directory is deleted
or lost, services registered with this command will need to be reregistered.

#### API Options

<%= partial "docs/commands/http_api_options_client" %>

#### Service Registration Flags

The flags below should only be set if _no arguments_ are given. If no
arguments are given, the flags below can be used to register a single
service.

Note that the behavior of each of the fields below is exactly the same
as when constructing a standard [service definition](/docs/agent/services.html).
Please refer to that documentation for full details.

* `-id` - The ID of the service. This will default to `-name` if not set.

* `-name` - The name of the service to register.

* `-address` - The address of the service. If this isn't specified,
  it will default to the address registered with the local agent.

* `-port` - The port of the service.

* `-meta key=value` - Specify arbitrary KV metadata to associate with the
  service instance. This can be specified multiple times.

* `-tag value` - Associate a tag with the service instance. This flag can
  be specified multiples times.

## Examples

To create a simple service:

```text
$ consul services register -name=web
```

To create a service from a configuration file:

```text
$ cat web.json
{
  "Service": {
    "Name": "web"
  }
}

$ consul services register web.json
```

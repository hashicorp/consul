---
layout: "docs"
page_title: "Service Definition"
sidebar_current: "docs-agent-services"
---

# Services

One of the main goals of service discovery is to provide a catalog of available
services. To that end, the agent provides a simple service definition format
to declare the availability of a service, and to potentially associate it with
a health check. A health check is considered to be application level if it
associated with a service. A service is defined in a configuration file,
or added at runtime over the HTTP interface.

## Service Definition

A service definition that is a script looks like:

    {
        "service": {
            "name": "redis",
            "id": "redis8080",
            "tags": ["master"],
            "port": 8000,
            "check": {
                "script": "/usr/local/bin/check_redis.py",
                "interval": "10s"
            }
        }
    }

A service definition must include a `name`, and may optionally provide
an `id`, `tags`, `port`, and `check`.  The `id` is set to the `name` if not
provided. It is required that all services have a unique ID per node, so if names
might conflict then unique ID's should be provided.

The `tags` is a list of opaque value to Consul, but can be used to distinguish
between "master" or "slave" nodes, different versions, or any other service level labels.
The `port` can be used as well to make a service oriented architecture
simpler to configure. This way the address and port of a service can
be discovered.

Lastly, a service can have an associated health check. This is a powerful
feature as it allows a web balancer to gracefully remove failing nodes, or
a database to replace a failed slave, etc. The health check is strongly integrated
in the DNS interface as well. If a service is failing its health check or
a node has any failing system-level check, the DNS interface will omit that
node from any service query.

There is more information about [checks here](/docs/agent/checks.html). The
check must be of the script or TTL type. If it is a script type, `script` and
`interval` must be provided. If it is a TTL type, then only `ttl` must be
provided. The check name is automatically generated as "service:<service-id>".

To configure a service, either provide it as a `-config-file` option to the
agent, or place it inside the `-config-dir` of the agent. The file must
end in the ".json" extension to be loaded by Consul. Check definitions can
also be updated by sending a `SIGHUP` to the agent. Alternatively, the
service can be registered dynamically using the [HTTP API](/docs/agent/http.html).


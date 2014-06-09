---
layout: "docs"
page_title: "Check Definition"
sidebar_current: "docs-agent-checks"
---

# Checks

One of the primary roles of the agent is the management of system and
application level health checks. A health check is considered to be application
level if it associated with a service. A check is defined in a configuration file,
or added at runtime over the HTTP interface.

There are two different kinds of checks:

 * Script + Interval - These checks depend on invoking an external application
 which does the health check and exits with an appropriate exit code, potentially
 generating some output. A script is paired with an invocation interval (e.g.
 every 30 seconds). This is similar to the Nagios plugin system.

 * TTL - These checks retain their last known state for a given TTL. The state
 of the check must be updated periodically over the HTTP interface. If an
 external system fails to update the status within a given TTL, the check is
 set to the failed state. This mechanism is used to allow an application to
 directly report its health. For example, a web app can periodically curl the
 endpoint, and if the app fails, then the TTL will expire and the health check
 enters a critical state. This is conceptually similar to a dead man's switch.

## Check Definition

A check definition that is a script looks like:

    {
        "check": {
            "id": "mem-util",
            "name": "Memory utilization",
            "script": "/usr/local/bin/check_mem.py",
            "interval": "10s"
        }
    }

A TTL based check is very similar:

    {
        "check": {
            "id": "web-app",
            "name": "Web App Status",
            "notes": "Web app does a curl internally every 10 seconds",
            "ttl": "30s"
        }
    }

Both types of definitions must include a `name`, and may optionally
provide an `id` and `notes` field. The `id` is set to the `name` if not
provided. It is required that all checks have a unique ID per node, so if names
might conflict then unique ID's should be provided.

The `notes` field is opaque to Consul, but may be used for human
readable descriptions. The field is set to any output that a script
generates, and similarly the TTL update hooks can update the `notes`
as well.

To configure a check, either provide it as a `-config-file` option to the
agent, or place it inside the `-config-dir` of the agent. The file must
end in the ".json" extension to be loaded by Consul. Check definitions can
also be updated by sending a `SIGHUP` to the agent. Alternatively, the
check can be registered dynamically using the [HTTP API](/docs/agent/http.html).

## Check Scripts

A check script is generally free to do anything to determine the status
of the check. The only limitations placed are the exit codes must convey
a specific meaning. Specifically:

 * Exit code 0 - Check is passing
 * Exit code 1 - Check is warning
 * Any other code - Check is failing

This is the only convention that Consul depends on. Any output of the script
will be captured and stored in the `notes` field so that it can be viewed
by human operators.

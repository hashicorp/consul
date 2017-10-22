---
layout: "docs"
page_title: "Watches"
sidebar_current: "docs-agent-watches"
description: |-
  Watches are a way of specifying a view of data (e.g. list of nodes, KV pairs, health checks) which is monitored for updates. When an update is detected, an external handler is invoked. A handler can be any executable. As an example, you could watch the status of health checks and notify an external system when a check is critical.
---

# Watches

Watches are a way of specifying a view of data (e.g. list of nodes, KV pairs, health
checks) which is monitored for updates. When an update is detected, an external handler
is invoked. A handler can be any executable or HTTP endpoint. As an example, you could watch the status
of health checks and notify an external system when a check is critical.

Watches are implemented using blocking queries in the [HTTP API](/api/index.html).
Agents automatically make the proper API calls to watch for changes
and inform a handler when the data view has updated.

Watches can be configured as part of the [agent's configuration](/docs/agent/options.html#watches),
causing them to run once the agent is initialized. Reloading the agent configuration
allows for adding or removing watches dynamically.

Alternatively, the [watch command](/docs/commands/watch.html) enables a watch to be
started outside of the agent. This can be used by an operator to inspect data in Consul
or to easily pipe data into processes without being tied to the agent lifecycle.

In either case, the `type` of the watch must be specified. Each type of watch
supports different parameters, some required and some optional. These options are specified
in a JSON body when using agent configuration or as CLI flags for the watch command.

## Handlers

The watch configuration specifies the view of data to be monitored.
Once that view is updated, the specified handler is invoked. Handlers can be either an
executable or an HTTP endpoint. A handler receives JSON formatted data
with invocation info, following a format that depends on the type of the watch.
Each watch type documents the format type. Because they map directly to an HTTP
API, handlers should expect the input to match the format of the API. A Consul
index is also given, corresponding to the responses from the
[HTTP API](/api/index.html). 

### Executable

An executable handler reads the JSON invocation info from stdin. Additionally,
the `CONSUL_INDEX` environment variable will be set to the Consul index
Anything written to stdout is logged.

Here is an example configuration, where `handler_type` is optionally set to
`script`:

```javascript
{
  "type": "key",
  "key": "foo/bar/baz",
  "handler_type": "script",
  "args": ["/usr/bin/my-service-handler.sh", "-redis"]
}
```

Prior to Consul 1.0, watches used a single `handler` field to define the command to run, and
would always run in a shell. In Consul 1.0, the `args` array was added so that handlers can be
run without a shell. The `handler` field is deprecated, and you should include the shell in
the `args` to run under a shell, eg. `"args": ["sh", "-c", "..."]`.

### HTTP endpoint

An HTTP handler sends an HTTP request when a watch is invoked. The JSON invocation info is sent
as a payload along the request. The response also contains the Consul index as a header named
`X-Consul-Index`.

The HTTP handler can be configured by setting `handler_type` to `http`. Additional handler options
are set using `http_handler_config`. The only required parameter is the `path` field which specifies
the URL to the HTTP endpoint. Consul uses `POST` as the default HTTP method, but this is also configurable.
Other optional fields are `header`, `timeout` and`tls_skip_verify`. The watch invocation data is
always sent as a JSON payload.

Here is an example configuration:

```javascript
{
  "type": "key",
  "key": "foo/bar/baz",
  "handler_type": "http",
  "http_handler_config": {
    "path":"https://localhost:8000/watch",
    "method": "POST",
    "header": {"x-foo":["bar", "baz"]},
    "timeout": "10s",
    "tls_skip_verify": false
  }
}
```

## Global Parameters

In addition to the parameters supported by each option type, there
are a few global parameters that all watches support:

* `datacenter` - Can be provided to override the agent's default datacenter.
* `token` - Can be provided to override the agent's default ACL token.
* `args` - The handler subprocess and arguments to invoke when the data view updates.
* `handler` - The handler shell command to invoke when the data view updates.

## Watch Types

The following types are supported. Detailed documentation on each is below:

* [`key`](#key) - Watch a specific KV pair
* [`keyprefix`](#keyprefix) - Watch a prefix in the KV store
* [`services`](#services) - Watch the list of available services
* [`nodes`](#nodes) - Watch the list of nodes
* [`service`](#service)-  Watch the instances of a service
* [`checks`](#checks) - Watch the value of health checks
* [`event`](#event) - Watch for custom user events


### <a name="key"></a>Type: key

The "key" watch type is used to watch a specific key in the KV store.
It requires that the "key" parameter be specified.

This maps to the `/v1/kv/` API internally.

Here is an example configuration:

```javascript
{
  "type": "key",
  "key": "foo/bar/baz",
  "args": ["/usr/bin/my-service-handler.sh", "-redis"]
}
```

Or, using the watch command:

    $ consul watch -type=key -key=foo/bar/baz /usr/bin/my-key-handler.sh

An example of the output of this command:

```javascript
{
  "Key": "foo/bar/baz",
  "CreateIndex": 1793,
  "ModifyIndex": 1793,
  "LockIndex": 0,
  "Flags": 0,
  "Value": "aGV5",
  "Session": ""
}
```

### <a name="keyprefix"></a>Type: keyprefix

The "keyprefix" watch type is used to watch a prefix of keys in the KV store.
It requires that the "prefix" parameter be specified. This watch
returns *all* keys matching the prefix whenever *any* key matching the prefix
changes.

This maps to the `/v1/kv/` API internally.

Here is an example configuration:

```javascript
{
  "type": "keyprefix",
  "prefix": "foo/",
  "args": ["/usr/bin/my-service-handler.sh", "-redis"]
}
```

Or, using the watch command:

    $ consul watch -type=keyprefix -prefix=foo/ /usr/bin/my-prefix-handler.sh

An example of the output of this command:

```javascript
[
  {
    "Key": "foo/bar",
    "CreateIndex": 1796,
    "ModifyIndex": 1796,
    "LockIndex": 0,
    "Flags": 0,
    "Value": "TU9BUg==",
    "Session": ""
  },
  {
    "Key": "foo/baz",
    "CreateIndex": 1795,
    "ModifyIndex": 1795,
    "LockIndex": 0,
    "Flags": 0,
    "Value": "YXNkZg==",
    "Session": ""
  },
  {
    "Key": "foo/test",
    "CreateIndex": 1793,
    "ModifyIndex": 1793,
    "LockIndex": 0,
    "Flags": 0,
    "Value": "aGV5",
    "Session": ""
  }
]
```

### <a name="services"></a>Type: services

The "services" watch type is used to watch the list of available
services. It has no parameters.

This maps to the `/v1/catalog/services` API internally.

An example of the output of this command:

```javascript
{
  "consul": [],
  "redis": [],
  "web": []
}
```

### <a name="nodes"></a>Type: nodes

The "nodes" watch type is used to watch the list of available
nodes. It has no parameters.

This maps to the `/v1/catalog/nodes` API internally.

An example of the output of this command:

```javascript
[
  {
    "Node": "nyc1-consul-1",
    "Address": "192.241.159.115"
  },
  {
    "Node": "nyc1-consul-2",
    "Address": "192.241.158.205"
  },
  {
    "Node": "nyc1-consul-3",
    "Address": "198.199.77.133"
  },
  {
    "Node": "nyc1-worker-1",
    "Address": "162.243.162.228"
  },
  {
    "Node": "nyc1-worker-2",
    "Address": "162.243.162.226"
  },
  {
    "Node": "nyc1-worker-3",
    "Address": "162.243.162.229"
  }
]
```

### <a name="service"></a>Type: service

The "service" watch type is used to monitor the providers
of a single service. It requires the "service" parameter
and optionally takes the parameters "tag" and "passingonly".
The "tag" parameter will filter by tag, and "passingonly" is
a boolean that will filter to only the instances passing all
health checks.

This maps to the `/v1/health/service` API internally.

Here is an example configuration:

```javascript
{
  "type": "service",
  "service": "redis",
  "args": ["/usr/bin/my-service-handler.sh", "-redis"]
}
```

Or, using the watch command:

    $ consul watch -type=service -service=redis /usr/bin/my-service-handler.sh

An example of the output of this command:

```javascript
[
  {
    "Node": {
      "Node": "foobar",
      "Address": "10.1.10.12"
    },
    "Service": {
      "ID": "redis",
      "Service": "redis",
      "Tags": null,
      "Port": 8000
    },
    "Checks": [
      {
        "Node": "foobar",
        "CheckID": "service:redis",
        "Name": "Service 'redis' check",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "redis",
        "ServiceName": "redis"
      },
      {
        "Node": "foobar",
        "CheckID": "serfHealth",
        "Name": "Serf Health Status",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": ""
      }
    ]
  }
]
```

### <a name="checks"></a>Type: checks

The "checks" watch type is used to monitor the checks of a given
service or those in a specific state. It optionally takes the "service"
parameter to filter to a specific service or the "state" parameter to
filter to a specific state. By default, it will watch all checks.

This maps to the `/v1/health/state/` API if monitoring by state
or `/v1/health/checks/` if monitoring by service.

An example of the output of this command:

```javascript
[
  {
    "Node": "foobar",
    "CheckID": "service:redis",
    "Name": "Service 'redis' check",
    "Status": "passing",
    "Notes": "",
    "Output": "",
    "ServiceID": "redis",
    "ServiceName": "redis"
  }
]
```

### <a name="event"></a>Type: event

The "event" watch type is used to monitor for custom user
events. These are fired using the [consul event](/docs/commands/event.html) command.
It takes only a single optional "name" parameter which restricts
the watch to only events with the given name.

This maps to the `v1/event/list` API internally.

Here is an example configuration:

```javascript
{
  "type": "event",
  "name": "web-deploy",
  "args": ["/usr/bin/my-service-handler.sh", "-web-deploy"]
}
```

Or, using the watch command:

    $ consul watch -type=event -name=web-deploy /usr/bin/my-deploy-handler.sh -web-deploy

An example of the output of this command:

```javascript
[
  {
    "ID": "f07f3fcc-4b7d-3a7c-6d1e-cf414039fcee",
    "Name": "web-deploy",
    "Payload": "MTYwOTAzMA==",
    "NodeFilter": "",
    "ServiceFilter": "",
    "TagFilter": "",
    "Version": 1,
    "LTime": 18
  },
  ...
]
```

To fire a new `web-deploy` event the following could be used:

    $ consul event -name=web-deploy 1609030

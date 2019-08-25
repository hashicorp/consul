---
layout: api
page_title: Discovery Chain - HTTP API
sidebar_current: api-discovery-chain
description: |-
  The /discovery-chain endpoints are for interacting with the discovery chain.
---

-> **1.6.0+:**  The discovery chain API is available in Consul versions 1.6.0 and newer.

# Discovery Chain HTTP Endpoint

~> This is a low-level API primarily targeted at developers building external
[Connect proxy integrations](/docs/connect/proxies/integrate.html). Future
high-level proxy integration APIs may obviate the need for this API over time.

The `/discovery-chain` endpoint returns the compiled [discovery
chain](/docs/internals/discovery-chain.html) for a service.

This will fetch all related [configuration
entries](/docs/agent/config_entries.html) and render them into a form suitable
for use by a [connect proxy] (/docs/connect/proxies.html) implementation.  This
is a key component of [L7 Traffic
Management](/docs/connect/l7-traffic-management.html).

## Read Compiled Discovery Chain

If overrides are needed they are passed as the JSON-encoded request body and 
the `POST` method must be used, otherwise `GET` is sufficient.

| Method             | Path                         | Produces                   |
| ------------------ | ---------------------------- | -------------------------- |
| `GET`              | `/discovery-chain/:service`  | `application/json`         |
| `POST`<sup>1</sup> | `/discovery-chain/:service`  | `application/json`         |

<sup>1</sup> Both GET and POST are for **read** operations. GET requests do not
permit request bodies so a POST is required if override parameters are needed.

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching        | ACL Required   |
| ---------------- | ----------------- | -------------------- | -------------- |
| `YES`            | `all`             | `background refresh` | `service:read` |

### URL Parameters

- `service` `(string: <required>)` - Specifies the service to query when
  compiling the discovery chain.  This is provided as part of the URL.

- `compile-dc` `(string: "")` - Specifies the datacenter to use as the basis of
  compilation. This will default to the datacenter of the agent being queried.
  This is specified as part of the URL as a query parameter.

    This value comes from an [upstream
    configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
    [`datacenter`](/docs/connect/registration/service-registration.html#datacenter)
    parameter.

### POST Body Parameters

- `OverrideConnectTimeout` `(duration: 0s)` - Overrides the final [connect
  timeout](/docs/agent/config-entries/service-resolver.html#connecttimeout) for
  any service resolved in the compiled chain.

    This value comes from the `connect_timeout_ms` key in an [upstream
    configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
    opaque
    [`config`](/docs/connect/registration/service-registration.html#config-1)
    parameter.

- `OverrideProtocol` `(string: "")` - Overrides the final
  [protocol](/docs/agent/config-entries/service-defaults.html#protocol) used in
  the compiled discovery chain.

    If the chain ordinarily would be TCP and an L7 protocol is passed here the
    chain will still not include Routers or Splitters. If the chain ordinarily
    would be L7 and TCP is passed here the chain will not include Routers or
    Splitters.

    This value comes from the `protocol` key in an [upstream
    configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
    opaque
    [`config`](/docs/connect/registration/service-registration.html#config-1)
    parameter.

- `OverrideMeshGateway` `(MeshGatewayConfig: <optional>)` - Overrides the final
  [mesh gateway configuration](/docs/connect/mesh_gateway.html#connect-proxy-configuration)
  for this any service resolved in the compiled chain.

    This value comes from either the [proxy
    configuration](/docs/connect/registration/service-registration.html#complete-configuration-example)
    [`mesh_gateway`](/docs/connect/registration/service-registration.html#mesh_gateway)
    parameter or an [upstream
    configuration](/docs/connect/registration/service-registration.html#upstream-configuration-reference)
    [`mesh_gateway`](/docs/connect/registration/service-registration.html#mesh_gateway-1)
    parameter. If both are present the value defined on the upstream is used.

  - `Mode` `(string: "")` - One of `none`, `local`, or `remote`.

### Sample Compilations

Full documentation for the output fields is found on the [discovery chain
internals](http://localhost:4567/docs/internals/discovery-chain.html#compileddiscoverychain)
page.

#### Multi-Datacenter Failover

Config entries defined:

```hcl
kind            = "service-resolver"
name            = "web"
connect_timeout = "15s"
failover = {
  "*" = {
    datacenters = ["dc3", "dc4"]
  }
}
```

Request:

```text
$ curl http://127.0.0.1:8500/v1/discovery-chain/web
```

Response:

```json
{
    "Chain": {
        "ServiceName": "web",
        "Namespace": "default",
        "Datacenter": "dc1",
        "Protocol": "tcp",
        "StartNode": "resolver:web.default.dc1",
        "Nodes": {
            "resolver:web.default.dc1": {
                "Type": "resolver",
                "Name": "web.default.dc1",
                "Resolver": {
                    "ConnectTimeout": "15s",
                    "Target": "web.default.dc1",
                    "Failover": {
                        "Targets": [
                            "web.default.dc3",
                            "web.default.dc4"
                        ]
                    }
                }
            }
        },
        "Targets": {
            "web.default.dc1": {
                "ID": "web.default.dc1",
                "Service": "web",
                "Namespace": "default",
                "Datacenter": "dc1",
                "MeshGateway": {},
                "Subset": {},
                "SNI": "web.default.dc1.internal.47e25151-6212-ba25-8b7e-81adbbbab461.consul",
                "Name": "web.default.dc1.internal.47e25151-6212-ba25-8b7e-81adbbbab461.consul"
            },
            "web.default.dc3": {
                "ID": "web.default.dc3",
                "Service": "web",
                "Namespace": "default",
                "Datacenter": "dc3",
                "MeshGateway": {},
                "Subset": {},
                "SNI": "web.default.dc3.internal.47e25151-6212-ba25-8b7e-81adbbbab461.consul",
                "Name": "web.default.dc3.internal.47e25151-6212-ba25-8b7e-81adbbbab461.consul"
            },
            "web.default.dc4": {
                "ID": "web.default.dc4",
                "Service": "web",
                "Namespace": "default",
                "Datacenter": "dc4",
                "MeshGateway": {},
                "Subset": {},
                "SNI": "web.default.dc4.internal.47e25151-6212-ba25-8b7e-81adbbbab461.consul",
                "Name": "web.default.dc4.internal.47e25151-6212-ba25-8b7e-81adbbbab461.consul"
            }
        }
    }
}
```

#### Datacenter Redirect with Overrides

Config entries defined:

```hcl
kind            = "service-resolver"
name            = "web"
redirect {
  datacenter = "dc2"
}
```

Request:

```text
$ curl -X POST \
    -d'
{
    "OverrideConnectTimeout": "7s",
    "OverrideProtocol": "grpc",
    "OverrideMeshGateway": {
        "Mode": "remote"
    }
}
' http://127.0.0.1:8500/v1/discovery-chain/web
```

Response:

```json
{
    "Chain": {
        "ServiceName": "web",
        "Namespace": "default",
        "Datacenter": "dc1",
        "CustomizationHash": "b94f529a",
        "Protocol": "grpc",
        "StartNode": "resolver:web.default.dc2",
        "Nodes": {
            "resolver:web.default.dc2": {
                "Type": "resolver",
                "Name": "web.default.dc2",
                "Resolver": {
                    "ConnectTimeout": "7s",
                    "Target": "web.default.dc2"
                }
            }
        },
        "Targets": {
            "web.default.dc2": {
                "ID": "web.default.dc2",
                "Service": "web",
                "Namespace": "default",
                "Datacenter": "dc2",
                "MeshGateway": {
                    "Mode": "remote"
                },
                "Subset": {},
                "SNI": "web.default.dc2.internal.59c17fd4-8dfa-f54a-ae71-855b26faf637.consul",
                "Name": "web.default.dc2.internal.59c17fd4-8dfa-f54a-ae71-855b26faf637.consul"
            }
        }
    }
}
```

#### Version Split For Alternate Datacenter

Config entries defined:

```hcl
kind           = "service-resolver"
name           = "web"
default_subset = "v1"
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
# ---------------------------
kind = "service-defaults"
name = "web"
protocol = "http"
# ---------------------------
kind = "service-splitter"
name = "web"
splits = [
  {
    weight         = 90
    service_subset = "v1"
  },
  {
    weight         = 10
    service_subset = "v2"
  },
]
```

Request:

```text
$ curl http://127.0.0.1:8500/v1/discovery-chain/web?compile-dc=dc2
```

Response:

```json
{
    "Chain": {
        "ServiceName": "web",
        "Namespace": "default",
        "Datacenter": "dc2",
        "Protocol": "http",
        "StartNode": "splitter:web",
        "Nodes": {
            "resolver:v1.web.default.dc2": {
                "Type": "resolver",
                "Name": "v1.web.default.dc2",
                "Resolver": {
                    "ConnectTimeout": "5s",
                    "Target": "v1.web.default.dc2"
                }
            },
            "resolver:v2.web.default.dc2": {
                "Type": "resolver",
                "Name": "v2.web.default.dc2",
                "Resolver": {
                    "ConnectTimeout": "5s",
                    "Target": "v2.web.default.dc2"
                }
            },
            "splitter:web": {
                "Type": "splitter",
                "Name": "web",
                "Splits": [
                    {
                        "Weight": 90,
                        "NextNode": "resolver:v1.web.default.dc2"
                    },
                    {
                        "Weight": 10,
                        "NextNode": "resolver:v2.web.default.dc2"
                    }
                ]
            }
        },
        "Targets": {
            "v1.web.default.dc2": {
                "ID": "v1.web.default.dc2",
                "Service": "web",
                "ServiceSubset": "v1",
                "Namespace": "default",
                "Datacenter": "dc2",
                "MeshGateway": {},
                "Subset": {
                    "Filter": "Service.Meta.version == v1"
                },
                "SNI": "v1.web.default.dc2.internal.6c9594ec-d798-28b9-d084-aa03e81cf078.consul",
                "Name": "v1.web.default.dc2.internal.6c9594ec-d798-28b9-d084-aa03e81cf078.consul"
            },
            "v2.web.default.dc2": {
                "ID": "v2.web.default.dc2",
                "Service": "web",
                "ServiceSubset": "v2",
                "Namespace": "default",
                "Datacenter": "dc2",
                "MeshGateway": {},
                "Subset": {
                    "Filter": "Service.Meta.version == v2"
                },
                "SNI": "v2.web.default.dc2.internal.6c9594ec-d798-28b9-d084-aa03e81cf078.consul",
                "Name": "v2.web.default.dc2.internal.6c9594ec-d798-28b9-d084-aa03e81cf078.consul"
            }
        }
    }
}
```

#### HTTP Path Routing

Config entries defined:

```hcl
kind           = "service-resolver"
name           = "web"
subsets = {
  "canary" = {
    filter = "Service.Meta.flavor == canary"
  }
}
# ---------------------------
kind = "proxy-defaults"
name = "web"
config {
  protocol = "http"
}
# ---------------------------
kind = "service-router"
name = "web"
routes = [
  {
    match {
      http {
        path_prefix = "/admin"
      }
    }
    destination {
      service         = "admin"
      prefix_rewrite  = "/"
      request_timeout = "15s"
    }
  },
  {
    match {
      http {
        header = [
          {
            name  = "x-debug"
            exact = "1"
          },
        ]
      }
    }
    destination {
      service                  = "web"
      service_subset           = "canary"
      num_retries              = 5
      retry_on_connect_failure = true
      retry_on_status_codes    = [401, 409]
    }
  },
]
```

Request:

```text
$ curl http://127.0.0.1:8500/v1/discovery-chain/web
```

Response:

```json
{
    "Chain": {
        "ServiceName": "web",
        "Namespace": "default",
        "Datacenter": "dc1",
        "Protocol": "http",
        "StartNode": "router:web",
        "Nodes": {
            "resolver:admin.default.dc1": {
                "Type": "resolver",
                "Name": "admin.default.dc1",
                "Resolver": {
                    "ConnectTimeout": "5s",
                    "Default": true,
                    "Target": "admin.default.dc1"
                }
            },
            "resolver:canary.web.default.dc1": {
                "Type": "resolver",
                "Name": "canary.web.default.dc1",
                "Resolver": {
                    "ConnectTimeout": "5s",
                    "Target": "canary.web.default.dc1"
                }
            },
            "resolver:web.default.dc1": {
                "Type": "resolver",
                "Name": "web.default.dc1",
                "Resolver": {
                    "ConnectTimeout": "5s",
                    "Target": "web.default.dc1"
                }
            },
            "router:web": {
                "Type": "router",
                "Name": "web",
                "Routes": [
                    {
                        "Definition": {
                            "Match": {
                                "HTTP": {
                                    "PathPrefix": "/admin"
                                }
                            },
                            "Destination": {
                                "RequestTimeout": "15s",
                                "Service": "admin",
                                "PrefixRewrite": "/"
                            }
                        },
                        "NextNode": "resolver:admin.default.dc1"
                    },
                    {
                        "Definition": {
                            "Match": {
                                "HTTP": {
                                    "Header": [
                                        {
                                            "Name": "x-debug",
                                            "Exact": "1"
                                        }
                                    ]
                                }
                            },
                            "Destination": {
                                "Service": "web",
                                "ServiceSubset": "canary",
                                "NumRetries": 5,
                                "RetryOnConnectFailure": true,
                                "RetryOnStatusCodes": [
                                    401,
                                    409
                                ]
                            }
                        },
                        "NextNode": "resolver:canary.web.default.dc1"
                    },
                    {
                        "Definition": {
                            "Match": {
                                "HTTP": {
                                    "PathPrefix": "/"
                                }
                            },
                            "Destination": {
                                "Service": "web"
                            }
                        },
                        "NextNode": "resolver:web.default.dc1"
                    }
                ]
            }
        },
        "Targets": {
            "admin.default.dc1": {
                "ID": "admin.default.dc1",
                "Service": "admin",
                "Namespace": "default",
                "Datacenter": "dc1",
                "MeshGateway": {},
                "Subset": {},
                "SNI": "admin.default.dc1.internal.fce8a058-0981-2c04-d23c-b7375af64ce8.consul",
                "Name": "admin.default.dc1.internal.fce8a058-0981-2c04-d23c-b7375af64ce8.consul"
            },
            "canary.web.default.dc1": {
                "ID": "canary.web.default.dc1",
                "Service": "web",
                "ServiceSubset": "canary",
                "Namespace": "default",
                "Datacenter": "dc1",
                "MeshGateway": {},
                "Subset": {
                    "Filter": "Service.Meta.flavor == canary"
                },
                "SNI": "canary.web.default.dc1.internal.fce8a058-0981-2c04-d23c-b7375af64ce8.consul",
                "Name": "canary.web.default.dc1.internal.fce8a058-0981-2c04-d23c-b7375af64ce8.consul"
            },
            "web.default.dc1": {
                "ID": "web.default.dc1",
                "Service": "web",
                "Namespace": "default",
                "Datacenter": "dc1",
                "MeshGateway": {},
                "Subset": {},
                "SNI": "web.default.dc1.internal.fce8a058-0981-2c04-d23c-b7375af64ce8.consul",
                "Name": "web.default.dc1.internal.fce8a058-0981-2c04-d23c-b7375af64ce8.consul"
            }
        }
    }
}
```

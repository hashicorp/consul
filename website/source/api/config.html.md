---
layout: api
page_title: Config - HTTP API
sidebar_current: api-config
description: |-
  The /config endpoints are for managing centralized configuration
  in Consul.
---

# Config HTTP Endpoint

The `/config` endpoints create, update, delete and query central configuration
entries registered with Consul. See the
[agent configuration](/docs/agent/options.html#enable_central_service_config)
for more information on how to enable this functionality for centrally
configuring services and [configuration entries docs](/docs/agent/config_entries.html) for a description
of the configuration entries content.

## Apply Configuration

This endpoint creates or updates the given config entry.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/config`                    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required  |
| ---------------- | ----------------- | ------------- | ------------- |
| `NO`             | `none`            | `none`        | `service:write`<br>`operator:write`<sup>1</sup> |

<sup>1</sup> The ACL required depends on the config entry kind being updated:

| Config Entry Kind | Required ACL     |
| ----------------- | ---------------- |
| service-defaults  | `service:write`  |
| proxy-defaults    | `operator:write` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `cas` `(int: 0)` - Specifies to use a Check-And-Set operation. If the index is
  0, Consul will only store the entry if it does not already exist. If the index is
  non-zero, the entry is only set if the current index matches the `ModifyIndex`
  of that entry.

### Sample Payload


```javascript
{
    "Kind": "service-defaults",
    "Name": "web",
    "Protocol": "http",
}
```

### Sample Request

```sh
$ curl \
    --request PUT \
    --data @payload \
    http://127.0.0.1:8500/v1/config
```

## Get Configuration

This endpoint returns a specific config entry.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/config/:kind/:name`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required  |
| ---------------- | ----------------- | ------------- | ------------- |
| `YES`            | `all`             | `none`        | `service:read`<sup>1</sup> |

<sup>1</sup> The ACL required depends on the config entry kind being read:

| Config Entry Kind | Required ACL     |
| ----------------- | ---------------- |
| service-defaults  | `service:read`  |
| proxy-defaults    | `<none>` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `kind` `(string: <required>)` - Specifies the kind of the entry to read. This
  is specified as part of the URL.

- `name` `(string: <required>)` - Specifies the name of the entry to read. This
  is specified as part of the URL.

### Sample Request

```sh
$ curl \
    --request GET \
    http://127.0.0.1:8500/v1/config/service-defaults/web
```

### Sample Response

```json
{
    "Kind": "service-defaults",
    "Name": "web",
    "Protocol": "http",
    "CreateIndex": 15,
    "ModifyIndex": 35
}
```

## List Configurations

This endpoint returns all config entries of the given kind.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/config/:kind`              | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required  |
| ---------------- | ----------------- | ------------- | ------------- |
| `YES`            | `all`             | `none`        | `service:read`<sup>1</sup> |

<sup>1</sup> The ACL required depends on the config entry kind being read:

| Config Entry Kind | Required ACL     |
| ----------------- | ---------------- |
| service-defaults  | `service:read`  |
| proxy-defaults    | `<none>` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `kind` `(string: <required>)` - Specifies the kind of the entry to list. This
  is specified as part of the URL.

### Sample Request

```sh
$ curl \
    --request GET \
    http://127.0.0.1:8500/v1/config/service-defaults
```

### Sample Response

```json
[
    {
        "Kind": "service-defaults",
        "Name": "db",
        "Protocol": "tcp",
        "CreateIndex": 16,
        "ModifyIndex": 16
    },
    {
        "Kind": "service-defaults",
        "Name": "web",
        "Protocol": "http",
        "CreateIndex": 13,
        "ModifyIndex": 13
    }
]
```

## Delete Configuration

This endpoint creates or updates the given config entry.

| Method   | Path                         | Produces                   |
| -------- | ---------------------------- | -------------------------- |
| `DELETE` | `/config/:kind/:name`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required  |
| ---------------- | ----------------- | ------------- | ------------- |
| `NO`             | `none`            | `none`        | `service:write`<br>`operator:write`<sup>1</sup> |

<sup>1</sup> The ACL required depends on the config entry kind being deleted:

| Config Entry Kind | Required ACL     |
| ----------------- | ---------------- |
| service-defaults  | `service:write`  |
| proxy-defaults    | `operator:write` |

### Parameters

- `dc` `(string: "")` - Specifies the datacenter to query. This will default to
  the datacenter of the agent being queried. This is specified as part of the
  URL as a query parameter.

- `kind` `(string: <required>)` - Specifies the kind of the entry to delete. This
  is specified as part of the URL.

- `name` `(string: <required>)` - Specifies the name of the entry to delete. This
  is specified as part of the URL.

### Sample Request

```sh
$ curl \
    --request DELETE \
    http://127.0.0.1:8500/v1/config/service-defaults/web
```

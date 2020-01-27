---
layout: api
page_title: Keyring - Operator - HTTP API
sidebar_current: api-operator-keyring
description: |-
  The /operator/keyring endpoints allow for management of the gossip encryption
  keyring.
---

# Keyring Operator HTTP API

The `/operator/keyring` endpoints allow for management of the gossip encryption
keyring. Please see the [Gossip Protocol Guide](/docs/internals/gossip.html) for
more details on the gossip protocol and its use.

## List Gossip Encryption Keys

This endpoint lists the gossip encryption keys installed on both the WAN and LAN
rings of every known datacenter, unless otherwise specified with the `local-only` 
query parameter (see below).

If ACLs are enabled, the client will need to supply an ACL Token with `keyring`
read privileges.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/operator/keyring`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required   |
| ---------------- | ----------------- | ------------- | -------------- |
| `NO`             | `none`            | `none`        | `keyring:read` |

### Parameters

- `relay-factor` `(int: 0)` - Specifies the relay factor. Setting this to a
  non-zero value will cause nodes to relay their responses through this many
  randomly-chosen other nodes in the cluster. The maximum allowed value is `5`.
  This is specified as part of the URL as a query parameter.
- `local-only` `(bool: false)` - Setting `local-only` to true will force keyring 
  list queries to only hit local servers (no WAN traffic). This flag can only be set
  for list queries. It is specified as part of the URL as a query parameter.

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/operator/keyring
```

### Sample Response

```json
[
  {
    "WAN": true,
    "Datacenter": "dc1",
    "Segment": "",
    "Keys": {
      "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=": 1,
      "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=": 1,
      "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=": 1
    },
    "NumNodes": 1
  },
  {
    "WAN": false,
    "Datacenter": "dc1",
    "Segment": "",
    "Keys": {
      "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=": 1,
      "ZWTL+bgjHyQPhJRKcFe3ccirc2SFHmc/Nw67l8NQfdk=": 1,
      "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4=": 1
    },
    "NumNodes": 1
  }
]
```

- `WAN` is true if the block refers to the WAN ring of that datacenter (rather
   than LAN).

- `Datacenter` is the datacenter the block refers to.

- `Segment` is the network segment the block refers to.

- `Keys` is a map of each gossip key to the number of nodes it's currently
  installed on.

- `NumNodes` is the total number of nodes in the datacenter.

## Add New Gossip Encryption Key

This endpoint installs a new gossip encryption key into the cluster.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/operator/keyring`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `keyring:write` |

### Parameters

- `relay-factor` `(int: 0)` - Specifies the relay factor. Setting this to a
  non-zero value will cause nodes to relay their responses through this many
  randomly-chosen other nodes in the cluster. The maximum allowed value is `5`.
  This is specified as part of the URL as a query parameter.

- `Key` `(string: <required>)` - Specifies the encryption key to install into
  the cluster.

### Sample Payload

```json
{
  "Key": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s="
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    http://127.0.0.1:8500/v1/operator/keyring
```

## Change Primary Gossip Encryption Key

This endpoint changes the primary gossip encryption key. The key must already be
installed before this operation can succeed.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/operator/keyring`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `keyring:write` |

### Parameters

- `relay-factor` `(int: 0)` - Specifies the relay factor. Setting this to a
  non-zero value will cause nodes to relay their responses through this many
  randomly-chosen other nodes in the cluster. The maximum allowed value is `5`.
  This is specified as part of the URL as a query parameter.

- `Key` `(string: <required>)` - Specifies the encryption key to begin using as
  primary into the cluster.

### Sample Payload

```json
{
 "Key": "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4="
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/operator/keyring
```

## Delete Gossip Encryption Key

This endpoint removes a gossip encryption key from the cluster. This operation
may only be performed on keys which are not currently the primary key.

| Method  | Path                         | Produces                   |
| ------- | ---------------------------- | -------------------------- |
| `DELETE`| `/operator/keyring`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `keyring:write` |

### Parameters

- `relay-factor` `(int: 0)` - Specifies the relay factor. Setting this to a
  non-zero value will cause nodes to relay their responses through this many
  randomly-chosen other nodes in the cluster. The maximum allowed value is `5`.
  This is specified as part of the URL as a query parameter.

- `Key` `(string: <required>)` - Specifies the encryption key to delete.

### Sample Payload

```json
{
 "Key": "WbL6oaTPom+7RG7Q/INbJWKy09OLar/Hf2SuOAdoQE4="
}
```

### Sample Request

```text
$ curl \
    --request DELETE \
    --data @payload.json \
    http://127.0.0.1:8500/v1/operator/keyring
```

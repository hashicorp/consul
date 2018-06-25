---
layout: api
page_title: Intentions - Connect - HTTP API
sidebar_current: api-connect-intentions
description: |-
  The /connect/intentions endpoint provide tools for managing intentions via
  Consul's HTTP API.
---

# Intentions - Connect HTTP API

The `/connect/intentions` endpoint provide tools for managing
[intentions](/docs/connect/intentions.html).

## Create Intention

This endpoint creates a new intention and returns its ID if it was created
successfully.

The name and destination pair must be unique. If another intention matches
the name and destination, the creation will fail. You must either update the
existing intention or delete it prior to creating a new one.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST` | `/connect/intentions`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required     |
| ---------------- | ----------------- | ---------------- |
| `NO`             | `none`            | `service:write` |

### Parameters

- `SourceName` `(string: <required>)` - The source of the intention.
  For a `SourceType` of `consul` this is the name of a Consul service. The
  service doesn't need to be registered.

- `DestinationName` `(string: <required>)` - The destination of the intention.
  The intention destination is always a Consul service, unlike the source.
  The service doesn't need to be registered.

- `SourceType` `(string: <required>)` - The type for the `SourceName` value.
  This can be only "consul" today to represent a Consul service.

- `Action` `(string: <required>)` - This is one of "allow" or "deny" for
  the action that should be taken if this intention matches a request.

- `Description` `(string: nil)` - Description for the intention. This is not
  used for anything by Consul, but is presented in API responses to assist
  tooling.

- `Meta` `(map<string|string>: nil)` - Specifies arbitrary KV metadata pairs.

### Sample Payload

```json
{
  "SourceName": "web",
  "DestinationName": "db",
  "SourceType": "consul",
  "Action": "allow"
}
```

### Sample Request

```text
$ curl \
    --request POST \
    --data @payload.json \
    https://consul.rocks/v1/connect/intentions
```

### Sample Response

```json
{
  "ID": "8f246b77-f3e1-ff88-5b48-8ec93abf3e05"
}
```

## Read Specific Intention

This endpoint reads a specific intention.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/connect/intentions/:uuid`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `YES`            | `all`             | `service:read` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the intention to read. This
  is specified as part of the URL.

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/connect/intentions/e9ebc19f-d481-42b1-4871-4d298d3acd5c
```

### Sample Response

```json
{
  "ID": "e9ebc19f-d481-42b1-4871-4d298d3acd5c",
  "Description": "",
  "SourceNS": "default",
  "SourceName": "web",
  "DestinationNS": "default",
  "DestinationName": "db",
  "SourceType": "consul",
  "Action": "allow",
  "DefaultAddr": "",
  "DefaultPort": 0,
  "Meta": {},
  "Precedence": 9,
  "CreatedAt": "2018-05-21T16:41:27.977155457Z",
  "UpdatedAt": "2018-05-21T16:41:27.977157724Z",
  "CreateIndex": 11,
  "ModifyIndex": 11
}
```

## List Intentions

This endpoint lists all intentions.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/connect/intentions`        | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `YES`            | `all`             | `service:read` |

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/connect/intentions
```

### Sample Response

```json
[
  {
    "ID": "e9ebc19f-d481-42b1-4871-4d298d3acd5c",
    "Description": "",
    "SourceNS": "default",
    "SourceName": "web",
    "DestinationNS": "default",
    "DestinationName": "db",
    "SourceType": "consul",
    "Action": "allow",
    "DefaultAddr": "",
    "DefaultPort": 0,
    "Meta": {},
    "Precedence": 9,
    "CreatedAt": "2018-05-21T16:41:27.977155457Z",
    "UpdatedAt": "2018-05-21T16:41:27.977157724Z",
    "CreateIndex": 11,
    "ModifyIndex": 11
  }
]
```

## Update Intention

This endpoint updates an intention with the given values.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/connect/intentions/:uuid`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required     |
| ---------------- | ----------------- | ---------------- |
| `NO`             | `none`            | `service:write` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the intention to update. This
  is specified as part of the URL.

- Other parameters are identical to creating an intention.

### Sample Payload

```json
{
  "SourceName": "web",
  "DestinationName": "other-db",
  "SourceType": "consul",
  "Action": "allow"
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    https://consul.rocks/v1/connect/intentions/e9ebc19f-d481-42b1-4871-4d298d3acd5c
```

## Delete Intention

This endpoint deletes a specific intention.

| Method   | Path                         | Produces                   |
| -------- | ---------------------------- | -------------------------- |
| `DELETE` | `/connect/intentions/:uuid`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required     |
| ---------------- | ----------------- | ---------------- |
| `NO`             | `none`            | `service:write` |

### Parameters

- `uuid` `(string: <required>)` - Specifies the UUID of the intention to delete. This
  is specified as part of the URL.

### Sample Request

```text
$ curl \
    --request DELETE \
    https://consul.rocks/v1/connect/intentions/e9ebc19f-d481-42b1-4871-4d298d3acd5c
```

## Check Intention Result

This endpoint evaluates the intentions for a specific source and destination
and returns whether the connection would be authorized or not given the
current Consul configuration and set of intentions.

This endpoint will work even if the destination service has
`intention = "deny"` specifically set, because the resulting API response
does not contain any information about the intention itself.


| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/connect/intentions/check`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required     |
| ---------------- | ----------------- | ---------------- |
| `NO`             | `none`            | `service:read` |

### Parameters

- `source` `(string: <required>)` - Specifies the source service. This
  is specified as part of the URL.

- `destination` `(string: <required>)` - Specifies the destination service. This
  is specified as part of the URL.

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/connect/intentions/check?source=web&destination=db
```

### Sample Response

```json
{
  "Allowed": true
}
```

- `Allowed` is true if the connection would be allowed, false otherwise.

## List Matching Intentions

This endpoint lists the intentions that match a given source or destination.
The intentions in the response are in evaluation order.

| Method | Path                           | Produces                   |
| ------ | ------------------------------ | -------------------------- |
| `GET`  | `/connect/intentions/match`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `NO`             | `none`            | `service:read` |

### Parameters

- `by` `(string: <required>)` - Specifies whether to match the "name" value
  by "source" or "destination".

- `name` `(string: <required>)` - Specifies a name to match. This parameter
  can be repeated for batching multiple matches.

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/connect/intentions/match?by=source&name=web
```

### Sample Response

```json
{
  "web": [
    {
      "ID": "ed16f6a6-d863-1bec-af45-96bbdcbe02be",
      "Description": "",
      "SourceNS": "default",
      "SourceName": "web",
      "DestinationNS": "default",
      "DestinationName": "db",
      "SourceType": "consul",
      "Action": "deny",
      "DefaultAddr": "",
      "DefaultPort": 0,
      "Meta": {},
      "CreatedAt": "2018-05-21T16:41:33.296693825Z",
      "UpdatedAt": "2018-05-21T16:41:33.296694288Z",
      "CreateIndex": 12,
      "ModifyIndex": 12
    },
    {
      "ID": "e9ebc19f-d481-42b1-4871-4d298d3acd5c",
      "Description": "",
      "SourceNS": "default",
      "SourceName": "web",
      "DestinationNS": "default",
      "DestinationName": "*",
      "SourceType": "consul",
      "Action": "allow",
      "DefaultAddr": "",
      "DefaultPort": 0,
      "Meta": {},
      "CreatedAt": "2018-05-21T16:41:27.977155457Z",
      "UpdatedAt": "2018-05-21T16:41:27.977157724Z",
      "CreateIndex": 11,
      "ModifyIndex": 11
    }
  ]
}
```

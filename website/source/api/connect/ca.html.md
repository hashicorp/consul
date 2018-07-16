---
layout: api
page_title: Certificate Authority - Connect - HTTP API
sidebar_current: api-connect-ca
description: |-
  The /connect/ca endpoints provide tools for interacting with Connect's
  Certificate Authority mechanism via Consul's HTTP API.
---

# Certificate Authority (CA) - Connect HTTP API

The `/connect/ca` endpoints provide tools for interacting with Connect's
Certificate Authority mechanism.

## List CA Root Certificates

This endpoint returns the current list of trusted CA root certificates in
the cluster.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/connect/ca/roots`          | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required     |
| ---------------- | ----------------- | ---------------- |
| `YES`            | `all`             | `operator:read`  |

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/connect/ca/roots
```

### Sample Response

```json
{
    "ActiveRootID": "c7:bd:55:4b:64:80:14:51:10:a4:b9:b9:d7:e0:75:3f:86:ba:bb:24",
    "TrustDomain": "7f42f496-fbc7-8692-05ed-334aa5340c1e.consul",
    "Roots": [
        {
            "ID": "c7:bd:55:4b:64:80:14:51:10:a4:b9:b9:d7:e0:75:3f:86:ba:bb:24",
            "Name": "Consul CA Root Cert",
            "SerialNumber": 7,
            "SigningKeyID": "32:64:3a:30:39:3a:35:64:3a:38:34:3a:62:39:3a:38:39:3a:34:62:3a:64:64:3a:65:33:3a:38:38:3a:62:62:3a:39:63:3a:65:32:3a:62:32:3a:36:39:3a:38:31:3a:31:66:3a:34:62:3a:61:36:3a:66:64:3a:34:64:3a:64:66:3a:65:65:3a:37:34:3a:36:33:3a:66:33:3a:37:34:3a:35:35:3a:63:61:3a:62:30:3a:62:35:3a:36:35",
            "NotBefore": "2018-05-25T21:39:23Z",
            "NotAfter": "2028-05-22T21:39:23Z",
            "RootCert": "-----BEGIN CERTIFICATE-----\nMIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA1MjUyMTM5MjNaFw0yODA1MjIyMTM5MjNaMBYxFDASBgNVBAMT\nC0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEq4S32Pu0/VL4\nG75gvdyQuAhqMZFsfBRwD3pgvblgZMeJc9KDosxnPR+W34NXtMD/860NNVJIILln\n9lLhIjWPQqOCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/\nMGgGA1UdDgRhBF8yZDowOTo1ZDo4NDpiOTo4OTo0YjpkZDplMzo4ODpiYjo5Yzpl\nMjpiMjo2OTo4MToxZjo0YjphNjpmZDo0ZDpkZjplZTo3NDo2MzpmMzo3NDo1NTpj\nYTpiMDpiNTo2NTBqBgNVHSMEYzBhgF8yZDowOTo1ZDo4NDpiOTo4OTo0YjpkZDpl\nMzo4ODpiYjo5YzplMjpiMjo2OTo4MToxZjo0YjphNjpmZDo0ZDpkZjplZTo3NDo2\nMzpmMzo3NDo1NTpjYTpiMDpiNTo2NTA/BgNVHREEODA2hjRzcGlmZmU6Ly83ZjQy\nZjQ5Ni1mYmM3LTg2OTItMDVlZC0zMzRhYTUzNDBjMWUuY29uc3VsMD0GA1UdHgEB\n/wQzMDGgLzAtgis3ZjQyZjQ5Ni1mYmM3LTg2OTItMDVlZC0zMzRhYTUzNDBjMWUu\nY29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIBBBDOWXWApx4S6bHJ49AW87Nw8uQ/gJ\nJ6lvm3HzEQw2AiEA4PVqWt+z8fsQht0cACM42kghL97SgDSf8rgCqfLYMng=\n-----END CERTIFICATE-----\n",
            "IntermediateCerts": null,
            "Active": true,
            "CreateIndex": 8,
            "ModifyIndex": 8
        }
    ]
}
```

## Get CA Configuration

This endpoint returns the current CA configuration.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/connect/ca/configuration`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `YES`            | `all`             | `operator:read` |

### Sample Request

```text
$ curl \
    https://consul.rocks/v1/connect/ca/configuration
```

### Sample Response

```json
{
    "Provider": "consul",
    "Config": {
        "LeafCertTTL": "72h",
        "RotationPeriod": "2160h"
    },
    "CreateIndex": 5,
    "ModifyIndex": 5
}
```

## Update CA Configuration

This endpoint updates the configuration for the CA. If this results in a
new root certificate being used, the [Root Rotation]
(/docs/connect/ca.html#root-certificate-rotation) process will be triggered.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `PUT`  | `/connect/ca/configuration`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required    |
| ---------------- | ----------------- | --------------- |
| `NO`             | `none`            | `operator:write`|

### Parameters

- `Provider` `(string: <required>)` - Specifies the CA provider type to use.

- `Config` `(map[string]string: <required>)` - The raw configuration to use
for the chosen provider. For more information on configuring the Connect CA
providers, see [Provider Config](/docs/connect/ca.html).

### Sample Payload

```json
{
    "Provider": "consul",
    "Config": {
        "LeafCertTTL": "72h",
        "PrivateKey": "-----BEGIN RSA PRIVATE KEY-----...",
        "RootCert": "-----BEGIN CERTIFICATE-----...",
        "RotationPeriod": "2160h"
    }
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    https://consul.rocks/v1/connect/ca/configuration
```
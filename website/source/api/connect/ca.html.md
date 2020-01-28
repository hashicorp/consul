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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required     |
| ---------------- | ----------------- | ------------- | ---------------- |
| `YES`            | `all`             | `none`        | `none`           |

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/connect/ca/roots
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
            "SigningKeyID": "2d:09:5d:84:b9:89:4b:dd:e3:88:bb:9c:e2:b2:69:81:1f:4b:a6:fd:4d:df:ee:74:63:f3:74:55:ca:b0:b5:65",
            "ExternalTrustDomain": "a1499528-fbf6-df7b-05e5-ae81e1873fc4",
            "NotBefore": "2018-05-25T21:39:23Z",
            "NotAfter": "2028-05-22T21:39:23Z",
            "RootCert": "-----BEGIN CERTIFICATE-----\nMIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA1MjUyMTM5MjNaFw0yODA1MjIyMTM5MjNaMBYxFDASBgNVBAMT\nC0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEq4S32Pu0/VL4\nG75gvdyQuAhqMZFsfBRwD3pgvblgZMeJc9KDosxnPR+W34NXtMD/860NNVJIILln\n9lLhIjWPQqOCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/\nMGgGA1UdDgRhBF8yZDowOTo1ZDo4NDpiOTo4OTo0YjpkZDplMzo4ODpiYjo5Yzpl\nMjpiMjo2OTo4MToxZjo0YjphNjpmZDo0ZDpkZjplZTo3NDo2MzpmMzo3NDo1NTpj\nYTpiMDpiNTo2NTBqBgNVHSMEYzBhgF8yZDowOTo1ZDo4NDpiOTo4OTo0YjpkZDpl\nMzo4ODpiYjo5YzplMjpiMjo2OTo4MToxZjo0YjphNjpmZDo0ZDpkZjplZTo3NDo2\nMzpmMzo3NDo1NTpjYTpiMDpiNTo2NTA/BgNVHREEODA2hjRzcGlmZmU6Ly83ZjQy\nZjQ5Ni1mYmM3LTg2OTItMDVlZC0zMzRhYTUzNDBjMWUuY29uc3VsMD0GA1UdHgEB\n/wQzMDGgLzAtgis3ZjQyZjQ5Ni1mYmM3LTg2OTItMDVlZC0zMzRhYTUzNDBjMWUu\nY29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIBBBDOWXWApx4S6bHJ49AW87Nw8uQ/gJ\nJ6lvm3HzEQw2AiEA4PVqWt+z8fsQht0cACM42kghL97SgDSf8rgCqfLYMng=\n-----END CERTIFICATE-----\n",
            "IntermediateCerts": null,
            "Active": true,
            "PrivateKeyType": "ec",
            "PrivateKeyBits": 256,
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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `YES`            | `all`             | `none`        | `operator:read` |

### Sample Request

```text
$ curl \
    http://127.0.0.1:8500/v1/connect/ca/configuration
```

### Sample Response

```json
{
    "Provider": "consul",
    "Config": {
        "LeafCertTTL": "72h",
        "RotationPeriod": "2160h",
        "IntermediateCertTTL": "8760h"
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
[blocking queries](/api/features/blocking.html),
[consistency modes](/api/features/consistency.html),
[agent caching](/api/features/caching.html), and
[required ACLs](/api/index.html#authentication).

| Blocking Queries | Consistency Modes | Agent Caching | ACL Required    |
| ---------------- | ----------------- | ------------- | --------------- |
| `NO`             | `none`            | `none`        | `operator:write`|

### Parameters

- `Provider` `(string: <required>)` - Specifies the CA provider type to use.

- `Config` `(map[string]string: <required>)` - The raw configuration to use
for the chosen provider. For more information on configuring the Connect CA
providers, see [Provider Config](/docs/connect/ca.html).

- `ForceWithoutCrossSigning` `(bool: <optional>)` - Indicates that the CA change
  should be force to complete even if the current CA doesn't support cross
  signing. Changing root without cross-signing may cause temporary connection
  failures until the rollout completes. See [Forced Rotation Without
  Cross-Signing](/docs/connect/ca.html#forced-rotation-without-cross-signing)
  for more detail.

### Sample Payload

```json
{
    "Provider": "consul",
    "Config": {
        "LeafCertTTL": "72h",
        "PrivateKey": "-----BEGIN RSA PRIVATE KEY-----...",
        "RootCert": "-----BEGIN CERTIFICATE-----...",
        "RotationPeriod": "2160h",
        "IntermediateCertTTL": "8760h"
    },
    "ForceWithoutCrossSigning": false
}
```

### Sample Request

```text
$ curl \
    --request PUT \
    --data @payload.json \
    http://127.0.0.1:8500/v1/connect/ca/configuration
```

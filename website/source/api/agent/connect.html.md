---
layout: api
page_title: Connect - Agent - HTTP API
sidebar_current: api-agent-connect
description: |-
  The /agent/connect endpoints interact with Connect with agent-local operations.
---

# Connect - Agent HTTP API

The `/agent/connect` endpoints interact with [Connect](/docs/connect/index.html)
with agent-local operations.

These endpoints may mirror the [non-agent Connect endpoints](/api/connect.html)
in some cases. Almost all agent-local Connect endpoints perform local caching
to optimize performance of Connect without having to make requests to the server.

## Authorize

This endpoint tests whether a connection attempt is authorized between
two services. This is the primary API that must be implemented by
[proxies](/docs/connect/proxies.html) or
[native integrations](/docs/connect/native.html)
that wish to integrate with Connect. Prior to calling this API, it is expected
that the client TLS certificate has been properly verified against the
current CA roots.

The implementation of this API uses locally cached data
and doesn't require any request forwarding to a server. Therefore, the
response typically occurs in microseconds to impose minimal overhead on the
connection attempt.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `POST`  | `/agent/connect/authorize`  | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required             |
| ---------------- | ----------------- | ------------------------ |
| `NO`             | `none`            | `service:write` |

### Parameters

- `Target` `(string: <required>)` - The name of the service that is being
  requested.

- `ClientCertURI` `(string: <required>)` - The unique identifier for the
  requesting client. This is currently the URI SAN from the TLS client
  certificate.

- `ClientCertSerial` `(string: <required>)` - The colon-hex-encoded serial
  number for the requesting client cert. This is used to check against
  revocation lists.

### Sample Payload

```json
{
  "Target": "db",
  "ClientCertURI": "spiffe://dc1-7e567ac2-551d-463f-8497-f78972856fc1.consul/ns/default/dc/dc1/svc/web",
  "ClientCertSerial": "04:00:00:00:00:01:15:4b:5a:c3:94"
}
```

### Sample Request

```text
$ curl \
   --request POST \
   --data @payload.json \
    https://consul.rocks/v1/agent/connect/authorize
```

### Sample Response

```json
{
  "Authorized": true,
  "Reason": "Matched intention: web => db (allow)"
}
```

## Certificate Authority (CA) Roots

This endpoint returns the trusted certificate authority (CA) root certificates.
This is used by [proxies](/docs/connect/proxies.html) or
[native integrations](/docs/connect/native.html) to verify served client
or server certificates are valid.

This is equivalent to the [non-Agent Connect endpoint](/api/connect.html),
but the response of this request is cached locally at the agent. This allows
for very fast response times and for fail open behavior if the server is
unavailable. This endpoint should be used by proxies and native integrations.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/connect/ca/roots`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required               |
| ---------------- | ----------------- | -------------------------- |
| `YES`             | `all`            | `none` |

### Sample Request

```text
$ curl \
   https://consul.rocks/v1/connect/ca/roots
```

### Sample Response

```json
{
  "ActiveRootID": "15:bf:3a:7d:ff:ea:c1:8c:46:67:6c:db:b8:81:18:36:ad:e5:d0:c7",
  "Roots": [
    {
      "ID": "15:bf:3a:7d:ff:ea:c1:8c:46:67:6c:db:b8:81:18:36:ad:e5:d0:c7",
      "Name": "Consul CA Root Cert",
      "SerialNumber": 7,
      "SigningKeyID": "31:66:3a:39:31:3a:63:61:3a:34:31:3a:38:66:3a:61:63:3a:36:37:3a:62:66:3a:35:39:3a:63:32:3a:66:61:3a:34:65:3a:37:35:3a:35:63:3a:64:38:3a:66:30:3a:35:35:3a:64:65:3a:62:65:3a:37:35:3a:62:38:3a:33:33:3a:33:31:3a:64:35:3a:32:34:3a:62:30:3a:30:34:3a:62:33:3a:65:38:3a:39:37:3a:35:62:3a:37:65",
      "NotBefore": "2018-05-21T16:33:28Z",
      "NotAfter": "2028-05-18T16:33:28Z",
      "RootCert": "-----BEGIN CERTIFICATE-----\nMIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA1MjExNjMzMjhaFw0yODA1MTgxNjMzMjhaMBYxFDASBgNVBAMT\nC0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAER0qlxjnRcMEr\niSGlH7G7dYU7lzBEmLUSMZkyBbClmyV8+e8WANemjn+PLnCr40If9cmpr7RnC9Qk\nGTaLnLiF16OCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/\nMGgGA1UdDgRhBF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1OTpjMjpmYTo0ZTo3\nNTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToyNDpiMDowNDpiMzpl\nODo5Nzo1Yjo3ZTBqBgNVHSMEYzBhgF8xZjo5MTpjYTo0MTo4ZjphYzo2NzpiZjo1\nOTpjMjpmYTo0ZTo3NTo1YzpkODpmMDo1NTpkZTpiZTo3NTpiODozMzozMTpkNToy\nNDpiMDowNDpiMzplODo5Nzo1Yjo3ZTA/BgNVHREEODA2hjRzcGlmZmU6Ly8xMjRk\nZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIuY29uc3VsMD0GA1UdHgEB\n/wQzMDGgLzAtgisxMjRkZjVhMC05ODIwLTc2YzMtOWFhOS02ZjYyMTY0YmExYzIu\nY29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIQDzkkI7R+0U12a+zq2EQhP/n2mHmta+\nfs2hBxWIELGwTAIgLdO7RRw+z9nnxCIA6kNl//mIQb+PGItespiHZKAz74Q=\n-----END CERTIFICATE-----\n",
      "IntermediateCerts": null,
      "Active": true,
      "CreateIndex": 8,
      "ModifyIndex": 8
    }
  ]
}
```

## Service Leaf Certificate

This endpoint returns the leaf certificate representing a single service.
This certificate is used as a server certificate for accepting inbound
connections and is also used as the client certificate for establishing
outbound connections to other services.

The agent generates a CSR locally and calls the
[CA sign API](/api/connect/ca.html) to sign it. The resulting certificate
is cached and returned by this API until it is near expiry or the root
certificates change.

This API supports blocking queries. The blocking query will block until
a new certificate is necessary because the existing certificate will expire
or the root certificate is being rotated. This blocking behavior allows
clients to efficiently wait for certificate rotations.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/connect/ca/leaf/:service`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required               |
| ---------------- | ----------------- | -------------------------- |
| `YES`             | `all`            | `service:write` |

### Parameters

- `Service` `(string: <required>)` - The name of the service for the leaf
  certificate. This is specified in the URL. The service does not need to
  exist in the catalog, but the proper ACL permissions must be available.

### Sample Request

```text
$ curl \
   https://consul.rocks/v1/connect/ca/leaf/web
```

### Sample Response

```json
{
  "SerialNumber": "08",
  "CertPEM": "-----BEGIN CERTIFICATE-----\nMIIChjCCAi2gAwIBAgIBCDAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA1MjExNjMzMjhaFw0xODA1MjQxNjMzMjhaMA4xDDAKBgNVBAMT\nA3dlYjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABJdLqRKd1SRycFOFceMHOBZK\nQW8HHO8jZ5C8dRswD+IwTd/otJPiaPrVzGOAi4MsaEUgDMemvN1jiywHt3II08mj\nggFyMIIBbjAOBgNVHQ8BAf8EBAMCA7gwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsG\nAQUFBwMBMAwGA1UdEwEB/wQCMAAwaAYDVR0OBGEEXzFmOjkxOmNhOjQxOjhmOmFj\nOjY3OmJmOjU5OmMyOmZhOjRlOjc1OjVjOmQ4OmYwOjU1OmRlOmJlOjc1OmI4OjMz\nOjMxOmQ1OjI0OmIwOjA0OmIzOmU4Ojk3OjViOjdlMGoGA1UdIwRjMGGAXzFmOjkx\nOmNhOjQxOjhmOmFjOjY3OmJmOjU5OmMyOmZhOjRlOjc1OjVjOmQ4OmYwOjU1OmRl\nOmJlOjc1OmI4OjMzOjMxOmQ1OjI0OmIwOjA0OmIzOmU4Ojk3OjViOjdlMFkGA1Ud\nEQRSMFCGTnNwaWZmZTovLzExMTExMTExLTIyMjItMzMzMy00NDQ0LTU1NTU1NTU1\nNTU1NS5jb25zdWwvbnMvZGVmYXVsdC9kYy9kYzEvc3ZjL3dlYjAKBggqhkjOPQQD\nAgNHADBEAiBS8kH3UERhBPHM/CQV/jXKLr0kReLqCdq1jZxc8Aq7hQIgFIus/ZX0\nOM/X3Yc1xb/qJiiEVzXcaz3oVFULOzrNAwk=\n-----END CERTIFICATE-----\n",
  "PrivateKeyPEM": "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIAOGglbwY8HdD3LFX6Bc94co2pzeFTto8ebWoML5E+QfoAoGCCqGSM49\nAwEHoUQDQgAEl0upEp3VJHJwU4Vx4wc4FkpBbwcc7yNnkLx1GzAP4jBN3+i0k+Jo\n+tXMY4CLgyxoRSAMx6a83WOLLAe3cgjTyQ==\n-----END EC PRIVATE KEY-----\n",
  "Service": "web",
  "ServiceURI": "spiffe://11111111-2222-3333-4444-555555555555.consul/ns/default/dc/dc1/svc/web",
  "ValidAfter": "2018-05-21T16:33:28Z",
  "ValidBefore": "2018-05-24T16:33:28Z",
  "CreateIndex": 5,
  "ModifyIndex": 5
}
```

- `SerialNumber` `string` - Monotonically increasing 64-bit serial number
  representing all certificates issued by this Consul cluster.

- `CertPEM` `(string)` - The PEM-encoded certificate.

- `PrivateKeyPEM` `(string)` - The PEM-encoded private key for this certificate.

- `Service` `(string)` - The name of the service that this certificate identifies.

- `ServiceURI` `(string)` - The URI SAN for this service.

- `ValidAfter` `(string)` - The time after which the certificate is valid.
  Used with `ValidBefore` this can determine the validity period of the certificate.

- `ValidBefore` `(string)` - The time before which the certificate is valid.
  Used with `ValidAfter` this can determine the validity period of the certificate.

## Managed Proxy Configuration

This endpoint returns the configuration for a
[managed proxy](/docs/connect/proxies.html).
Ths endpoint is only useful for _managed proxies_ and not relevant
for unmanaged proxies.

Managed proxy configuration is set in the service definition. When Consul
starts the managed proxy, it provides the service ID and ACL token. The proxy
is expected to call this endpoint to retrieve its configuration. It may use
a blocking query to detect any configuration changes.

| Method | Path                         | Produces                   |
| ------ | ---------------------------- | -------------------------- |
| `GET`  | `/agent/connect/proxy/:id`    | `application/json`         |

The table below shows this endpoint's support for
[blocking queries](/api/index.html#blocking-queries),
[consistency modes](/api/index.html#consistency-modes), and
[required ACLs](/api/index.html#acls).

| Blocking Queries | Consistency Modes | ACL Required               |
| ---------------- | ----------------- | -------------------------- |
| `YES`             | `all`            | `service:write, proxy token` |

### Parameters

- `ID` `(string: <required>)` - The ID (not the name) of the proxy service
  in the local agent catalog. For managed proxies, this is provided in the
  `CONSUL_PROXY_ID` environment variable by Consul.

### Sample Request

```text
$ curl \
   https://consul.rocks/v1/connect/proxy/web-proxy
```

### Sample Response

```json
{
  "ProxyServiceID": "web-proxy",
  "TargetServiceID": "web",
  "TargetServiceName": "web",
  "ContentHash": "cffa5f4635b134b9",
  "ExecMode": "daemon",
  "Command": [
    "/usr/local/bin/consul",
    "connect",
    "proxy"
  ],
  "Config": {
    "bind_address": "127.0.0.1",
    "bind_port": 20199,
    "local_service_address": "127.0.0.1:8181"
  }
}
```

- `ProxyServiceID` `string` - The ID of the proxy service.

- `TargetServiceID` `(string)` - The ID of the target service the proxy represents.

- `TargetServiceName` `(string)` - The name of the target service the proxy represents.

- `ContentHash` `(string)` - The content hash of the response used for hash-based
  blocking queries.

- `ExecMode` `(string)` - The execution mode of the managed proxy.

- `Command` `(array<string>)` - The command for the managed proxy.

- `Config` `(map<string|any>)` - The configuration for the managed proxy. This
  is a map of primitive values (including arrays and maps) that is set by the
  user.

---
layout: "docs"
page_title: "Connect - Certificate Management"
sidebar_current: "docs-connect-ca"
description: |-
  An overview of the Connect Certificate Authority mechanisms.
---

# Connect Certificate Management

Certificate management in Connect is done centrally through the Consul
servers using the configured CA (Certificate Authority) provider. A CA provider
manages root and intermediate certificates and performs certificate signing
operations. The Consul leader orchestrates CA provider operations as necessary,
such as when a service needs a new certificate or during CA rotation events.

The CA provider abstraction enables Consul to support multiple systems for
storing and signing certificates. Consul ships with a
[built-in CA](/docs/connect/ca/consul.html) which generates and stores the
root certificate and private key on the Consul servers. Consul also has
built-in support for
[Vault as a CA](/docs/connect/ca/vault.html). With Vault, the root certificate
and private key material remain with the Vault cluster. A future version of
Consul will support pluggable CA systems using external binaries.

## CA Bootstrapping

CA initialization happens automatically when a new Consul leader is elected
as long as
[Connect is enabled](/docs/connect/configuration.html#enable-connect-on-the-cluster)
and the CA system hasn't already been initialized. This initialization process
will generate the initial root certificates and setup the internal Consul server
state.

For the initial bootstrap, the CA provider can be configured through the
[Agent configuration](/docs/agent/options.html#connect_ca_config). After
initialization, the CA can only be updated through the
[Update CA Configuration API endpoint](/api/connect/ca.html#update-ca-configuration).
If a CA is already initialized, any changes to the CA configuration in the
agent configuration file (including removing the configuration completely)
will have no effect.

If no specific provider is configured when Connect is enabled, the built-in
Consul CA provider will be used and a private key and root certificate will
be generated automatically.

## Viewing Root Certificates

Root certificates can be queried with the
[list CA Roots endpoint](/api/connect/ca.html#list-ca-root-certificates).
With this endpoint, you can see the list of currently trusted root certificates.
When a cluster first initializes, this will only list one trusted root. Multiple
roots may appear as part of
[rotation](#).

```bash
$ curl http://localhost:8500/v1/connect/ca/roots
{
    "ActiveRootID": "31:6c:06:fb:49:94:42:d5:e4:55:cc:2e:27:b3:b2:2e:96:67:3e:7e",
    "TrustDomain": "36cb52cd-4058-f811-0432-6798a240c5d3.consul",
    "Roots": [
        {
            "ID": "31:6c:06:fb:49:94:42:d5:e4:55:cc:2e:27:b3:b2:2e:96:67:3e:7e",
            "Name": "Consul CA Root Cert",
            "SerialNumber": 7,
            "SigningKeyID": "31:39:3a:34:35:3a:38:62:3a:33:30:3a:61:31:3a:34:35:3a:38:34:3a:61:65:3a:32:33:3a:35:32:3a:64:62:3a:38:64:3a:31:62:3a:66:66:3a:61:39:3a:30:39:3a:64:62:3a:66:63:3a:32:61:3a:37:32:3a:33:39:3a:61:65:3a:64:61:3a:31:31:3a:35:33:3a:66:34:3a:33:37:3a:35:63:3a:64:65:3a:64:31:3a:36:38:3a:64:38",
            "NotBefore": "2018-06-06T17:35:25Z",
            "NotAfter": "2028-06-03T17:35:25Z",
            "RootCert": "-----BEGIN CERTIFICATE-----\nMIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA2MDYxNzM1MjVaFw0yODA2MDMxNzM1MjVaMBYxFDASBgNVBAMT\nC0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEgo09lpx63bHw\ncSXeeoSpHpHgyzX1Q8ewJ3RUg6Ie8Howbs/QBz1y/kGxsF35HXij3YrqhgQyPPx4\nbQ8FH2YR4aOCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/\nMGgGA1UdDgRhBF8xOTo0NTo4YjozMDphMTo0NTo4NDphZToyMzo1MjpkYjo4ZDox\nYjpmZjphOTowOTpkYjpmYzoyYTo3MjozOTphZTpkYToxMTo1MzpmNDozNzo1Yzpk\nZTpkMTo2ODpkODBqBgNVHSMEYzBhgF8xOTo0NTo4YjozMDphMTo0NTo4NDphZToy\nMzo1MjpkYjo4ZDoxYjpmZjphOTowOTpkYjpmYzoyYTo3MjozOTphZTpkYToxMTo1\nMzpmNDozNzo1YzpkZTpkMTo2ODpkODA/BgNVHREEODA2hjRzcGlmZmU6Ly8zNmNi\nNTJjZC00MDU4LWY4MTEtMDQzMi02Nzk4YTI0MGM1ZDMuY29uc3VsMD0GA1UdHgEB\n/wQzMDGgLzAtgiszNmNiNTJjZC00MDU4LWY4MTEtMDQzMi02Nzk4YTI0MGM1ZDMu\nY29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIHl6UDdouw8Fzn/oDHputAxt3UFbVg/U\nvC6jWPuqqMwmAiEAkvMadtwjtNU7m/AQRJrj1LeG3eXw7dWO8SlI2fEs0yY=\n-----END CERTIFICATE-----\n",
            "IntermediateCerts": null,
            "Active": true,
            "CreateIndex": 8,
            "ModifyIndex": 8
        }
    ]
}
```

## CA Configuration

After initialization, the CA provider configuration can be viewed with the
[Get CA Configuration API endpoint](/api/connect/ca.html#get-ca-configuration).
Consul will filter sensitive values from this endpoint depending on the
provider in use, so the configuration may not be complete.

```bash
$ curl http://localhost:8500/v1/connect/ca/configuration
{
    "Provider": "consul",
    "Config": {
        "RotationPeriod": "2160h"
    },
    "CreateIndex": 5,
    "ModifyIndex": 5
}
```

The CA provider can be reconfigured using the
[Update CA Configuration API endpoint](/api/connect/ca.html#update-ca-configuration).
Specific options for reconfiguration can be found in the specific
CA provider documentation in the sidebar to the left.

## Root Certificate Rotation

Whenever the CA's configuration is updated in a way that causes the root key to
change, a special rotation process will be triggered in order to smoothly transition to
the new certificate. This rotation is automatically orchestrated by Consul.

This also automatically occurs when a completely different CA provider is
configured (since this changes the root key). Therefore, this automatic rotation
process can also be used to cleanly transition between CA providers. For example,
updating Connect to use Vault instead of the built-in CA.

During rotation, an intermediate CA certificate is requested from the new root, which is then
cross-signed by the old root. This cross-signed certificate is then distributed
alongside any newly-generated leaf certificates used by the proxies once the new root
becomes active, and provides a chain of trust back to the old root certificate in the
event that a certificate signed by the new root is presented to a proxy that has not yet
updated its bundle of trusted root CA certificates to include the new root.

After the cross-signed certificate has been successfully generated and the new root
certificate or CA provider has been set up, the new root becomes the active one
and is immediately used for signing any new incoming certificate requests.

If we check the [list CA roots endpoint](/api/connect/ca.html#list-ca-root-certificates)
after updating the configuration with a new root certificate, we can see both the old and new root
certificates are present, and the currently active root has an intermediate certificate
which has been generated and cross-signed automatically by the old root during the
rotation process:

```bash
$ curl localhost:8500/v1/connect/ca/roots
{
    "ActiveRootID": "d2:2c:41:94:1e:50:04:ea:86:fc:08:d6:b0:45:a4:af:8a:eb:76:a0",
    "TrustDomain": "36cb52cd-4058-f811-0432-6798a240c5d3.consul",
    "Roots": [
        {
            "ID": "31:6c:06:fb:49:94:42:d5:e4:55:cc:2e:27:b3:b2:2e:96:67:3e:7e",
            "Name": "Consul CA Root Cert",
            "SerialNumber": 7,
            "SigningKeyID": "31:39:3a:34:35:3a:38:62:3a:33:30:3a:61:31:3a:34:35:3a:38:34:3a:61:65:3a:32:33:3a:35:32:3a:64:62:3a:38:64:3a:31:62:3a:66:66:3a:61:39:3a:30:39:3a:64:62:3a:66:63:3a:32:61:3a:37:32:3a:33:39:3a:61:65:3a:64:61:3a:31:31:3a:35:33:3a:66:34:3a:33:37:3a:35:63:3a:64:65:3a:64:31:3a:36:38:3a:64:38",
            "NotBefore": "2018-06-06T17:35:25Z",
            "NotAfter": "2028-06-03T17:35:25Z",
            "RootCert": "-----BEGIN CERTIFICATE-----\nMIICmDCCAj6gAwIBAgIBBzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA2MDYxNzM1MjVaFw0yODA2MDMxNzM1MjVaMBYxFDASBgNVBAMT\nC0NvbnN1bCBDQSA3MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEgo09lpx63bHw\ncSXeeoSpHpHgyzX1Q8ewJ3RUg6Ie8Howbs/QBz1y/kGxsF35HXij3YrqhgQyPPx4\nbQ8FH2YR4aOCAXswggF3MA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/\nMGgGA1UdDgRhBF8xOTo0NTo4YjozMDphMTo0NTo4NDphZToyMzo1MjpkYjo4ZDox\nYjpmZjphOTowOTpkYjpmYzoyYTo3MjozOTphZTpkYToxMTo1MzpmNDozNzo1Yzpk\nZTpkMTo2ODpkODBqBgNVHSMEYzBhgF8xOTo0NTo4YjozMDphMTo0NTo4NDphZToy\nMzo1MjpkYjo4ZDoxYjpmZjphOTowOTpkYjpmYzoyYTo3MjozOTphZTpkYToxMTo1\nMzpmNDozNzo1YzpkZTpkMTo2ODpkODA/BgNVHREEODA2hjRzcGlmZmU6Ly8zNmNi\nNTJjZC00MDU4LWY4MTEtMDQzMi02Nzk4YTI0MGM1ZDMuY29uc3VsMD0GA1UdHgEB\n/wQzMDGgLzAtgiszNmNiNTJjZC00MDU4LWY4MTEtMDQzMi02Nzk4YTI0MGM1ZDMu\nY29uc3VsMAoGCCqGSM49BAMCA0gAMEUCIHl6UDdouw8Fzn/oDHputAxt3UFbVg/U\nvC6jWPuqqMwmAiEAkvMadtwjtNU7m/AQRJrj1LeG3eXw7dWO8SlI2fEs0yY=\n-----END CERTIFICATE-----\n",
            "IntermediateCerts": null,
            "Active": false,
            "CreateIndex": 8,
            "ModifyIndex": 24
        },
        {
            "ID": "d2:2c:41:94:1e:50:04:ea:86:fc:08:d6:b0:45:a4:af:8a:eb:76:a0",
            "Name": "Consul CA Root Cert",
            "SerialNumber": 16238269036752183483,
            "SigningKeyID": "",
            "NotBefore": "2018-06-06T17:37:03Z",
            "NotAfter": "2028-06-03T17:37:03Z",
            "RootCert": "-----BEGIN CERTIFICATE-----\nMIIDijCCAnKgAwIBAgIJAOFZ66em1qC7MA0GCSqGSIb3DQEBCwUAMGIxCzAJBgNV\nBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNp\nc2NvMRIwEAYDVQQKDAlIYXNoaUNvcnAxEjAQBgNVBAMMCWxvY2FsaG9zdDAeFw0x\nODA2MDYxNzM3MDNaFw0yODA2MDMxNzM3MDNaMGIxCzAJBgNVBAYTAlVTMRMwEQYD\nVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNpc2NvMRIwEAYDVQQK\nDAlIYXNoaUNvcnAxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEB\nBQADggEPADCCAQoCggEBAK6ostXN6W093EpI3RDNQDwC1Gq3lPNoodL5XRaVVIBU\n3X5iC+Ttk02p67cHUguh4ZrWr3o3Dzxm+gKK0lfZLW0nNYNPAIGZWQD9zVSx1Lqt\n8X0pd+fhMV5coQrh3YIG/vy17IBTSBuRUX0mXOKjOeJJlrw1HQZ8pfm7WX6LFul2\nXszvgn5K1XR+9nhPy6K2bv99qsY0sm7AqCS2BjYBW8QmNngJOdLPdhyFh7invyXe\nPqgujc/KoA3P6e3/G7bJZ9+qoQMK8uwD7PxtA2hdQ9t0JGPsyWgzhwfBxWdBWRzV\nRvVi6Yu2tvw3QrjdeKQ5Ouw9FUb46VnTU7jTO974HjkCAwEAAaNDMEEwPwYDVR0R\nBDgwNoY0c3BpZmZlOi8vMzZjYjUyY2QtNDA1OC1mODExLTA0MzItNjc5OGEyNDBj\nNWQzLmNvbnN1bDANBgkqhkiG9w0BAQsFAAOCAQEATHgCro9VXj7JbH/tlB6f/KWf\n7r98+rlUE684ZRW9XcA9uUA6y265VPnemsC/EykPsririoh8My1jVPuEfgMksR39\n9eMDJKfutvSpLD1uQqZE8hu/hcYyrmQTFKjW71CfGIl/FKiAg7wXEw2ljLN9bxNv\nGG118wrJyMZrRvFjC2QKY025QQSJ6joNLFMpftsZrJlELtRV+nx3gMabpiDRXhIw\nJM6ti26P1PyVgGRPCOG10v+OuUtwe0IZoOqWpPJN8jzSuqZWf99uolkG0xuqLNz6\nd8qvTp1YF9tTmysgvdeGALez/02HTF035RVTsQfH9tM/+4yG1UnmjLpz3p4Fow==\n-----END CERTIFICATE-----",
            "IntermediateCerts": [
                "-----BEGIN CERTIFICATE-----\nMIIDTzCCAvWgAwIBAgIBFzAKBggqhkjOPQQDAjAWMRQwEgYDVQQDEwtDb25zdWwg\nQ0EgNzAeFw0xODA2MDYxNzM3MDNaFw0yODA2MDMxNzM3MDNaMGIxCzAJBgNVBAYT\nAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNpc2Nv\nMRIwEAYDVQQKDAlIYXNoaUNvcnAxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJ\nKoZIhvcNAQEBBQADggEPADCCAQoCggEBAK6ostXN6W093EpI3RDNQDwC1Gq3lPNo\nodL5XRaVVIBU3X5iC+Ttk02p67cHUguh4ZrWr3o3Dzxm+gKK0lfZLW0nNYNPAIGZ\nWQD9zVSx1Lqt8X0pd+fhMV5coQrh3YIG/vy17IBTSBuRUX0mXOKjOeJJlrw1HQZ8\npfm7WX6LFul2Xszvgn5K1XR+9nhPy6K2bv99qsY0sm7AqCS2BjYBW8QmNngJOdLP\ndhyFh7invyXePqgujc/KoA3P6e3/G7bJZ9+qoQMK8uwD7PxtA2hdQ9t0JGPsyWgz\nhwfBxWdBWRzVRvVi6Yu2tvw3QrjdeKQ5Ouw9FUb46VnTU7jTO974HjkCAwEAAaOC\nARswggEXMGgGA1UdDgRhBF8xOTo0NTo4YjozMDphMTo0NTo4NDphZToyMzo1Mjpk\nYjo4ZDoxYjpmZjphOTowOTpkYjpmYzoyYTo3MjozOTphZTpkYToxMTo1MzpmNDoz\nNzo1YzpkZTpkMTo2ODpkODBqBgNVHSMEYzBhgF8xOTo0NTo4YjozMDphMTo0NTo4\nNDphZToyMzo1MjpkYjo4ZDoxYjpmZjphOTowOTpkYjpmYzoyYTo3MjozOTphZTpk\nYToxMTo1MzpmNDozNzo1YzpkZTpkMTo2ODpkODA/BgNVHREEODA2hjRzcGlmZmU6\nLy8zNmNiNTJjZC00MDU4LWY4MTEtMDQzMi02Nzk4YTI0MGM1ZDMuY29uc3VsMAoG\nCCqGSM49BAMCA0gAMEUCIBp46tRDot7GFyDXu7egq7lXBvn+UUHD5MmlFvdWmtnm\nAiEAwKBzEMcLd5kCBgFHNGyksRAMh/AGdEW859aL6z0u4gM=\n-----END CERTIFICATE-----\n"
            ],
            "Active": true,
            "CreateIndex": 24,
            "ModifyIndex": 24
        }
    ]
}
```

The old root certificate will be automatically removed once enough time has elapsed
for any leaf certificates signed by it to expire.

---
layout: "docs"
page_title: "Connect - Certificate Management"
sidebar_current: "docs-connect-ca"
description: |-
  An overview of the Connect Certificate Authority mechanisms.
---

# Connect Certificate Management

The certificate management in Connect is done centrally through the Consul
servers using the configured CA (Certificate Authority) provider. A CA provider
controls the active root certificate and performs leaf certificate signing for 
proxies to use for mutual TLS. Currently, the only supported provider is the 
built-in Consul CA, which generates and stores the root certificate and key on
the Consul servers and can be configured with a custom key/certificate if needed.

The CA provider is initialized either on cluster bootstrapping, or (if Connect is
disabled initially) when a leader server is elected that has Connect enabled. 
During the cluster's initial bootstrapping, the CA provider can be configured 
through the [Agent configuration](docs/agent/options.html#connect_ca_config)
and afterward can only be updated through the [Update CA Configuration endpoint]
(/api/connect/ca.html#update-ca-configuration).

### Consul CA (Certificate Authority) Provider

By default, if no provider is configured when Connect is enabled, the Consul
provider will be used and a private key/root certificate will be generated
and used as the active root certificate for the cluster. To see this in action,
start Consul in [dev mode](/docs/agent/options.html#_dev) and query the 
[list CA Roots endpoint](/api/connect/ca.html#list-ca-root-certificates):

```bash
$ curl localhost:8500/v1/connect/ca/roots
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

#### Specifying a Private Key and Root Certificate

The above root certificate has been automatically generated during the cluster's
bootstrap, but it is possible to configure the Consul CA provider to use a specific
private key and root certificate.

To view the current CA configuration, use the [Get CA Configuration endpoint]
(/api/connect/ca.html#get-ca-configuration):

```bash
$ curl localhost:8500/v1/connect/ca/configuration
{
    "Provider": "consul",
    "Config": {
        "RotationPeriod": "2160h"
    },
    "CreateIndex": 5,
    "ModifyIndex": 5
}
```

This is the default Connect CA configuration if nothing is explicitly set when
Connect is enabled - the PrivateKey and RootCert fields have not been set, so those have
been generated (as seen above in the roots list).

There are two ways to have the Consul CA use a custom private key and root certificate:
either through the `ca_config` section of the [Agent configuration]
(docs/agent/options.html#connect_ca_config) (which can only be used during the cluster's
initial bootstrap) or through the [Update CA Configuration endpoint]
(/api/connect/ca.html#update-ca-configuration).

Currently consul requires that root certificates are valid [SPIFFE SVID Signing certificates]
(https://github.com/spiffe/spiffe/blob/master/standards/X509-SVID.md) and that the URI encoded
in the SAN is the cluster identifier created at bootstrap with the ".consul" TLD. In this 
example, we will set the URI SAN to `spiffe://36cb52cd-4058-f811-0432-6798a240c5d3.consul`.

In order to use the Update CA Configuration HTTP endpoint, the private key and certificate
must be passed via JSON:

```bash
$ jq -n --arg key "$(cat root.key)"  --arg cert "$(cat root.crt)" '
{
    "Provider": "consul",
    "Config": {
        "PrivateKey": $key,
        "RootCert": $cert,
        "RotationPeriod": "2160h"
    }
}' > ca_config.json
```

The resulting `ca_config.json` file can then be used to update the active root certificate:

```bash
$ cat ca_config.json
{
  "Provider": "consul",
  "Config": {
    "PrivateKey": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEArqiy1c3pbT3cSkjdEM1APALUareU...",
    "RootCert": "-----BEGIN CERTIFICATE-----\nMIIDijCCAnKgAwIBAgIJAOFZ66em1qC7MA0GCSqGSIb3...",
    "RotationPeriod": "2160h"
  }
}

$ curl --request PUT --data @ca_config.json localhost:8500/v1/connect/ca/configuration

...

[INFO] connect: CA rotated to new root under provider "consul"
``` 

The cluster is now using the new private key and root certificate. Updating the CA config
this way also triggered a certificate rotation, which will be covered in the next section.

#### Root Certificate Rotation

Whenever the CA's configuration is updated in a way that causes the root key to
change, a special rotation process will be triggered in order to smoothly transition to
the new certificate.

First, an intermediate CA certificate is requested from the new root, which is then
cross-signed by the old root. This cross-signed certificate is then distributed
alongside any newly-generated leaf certificates used by the proxies once the new root
becomes active, and provides a chain of trust back to the old root certificate in the
event that a certificate signed by the new root is presented to a proxy that has not yet
updated its bundle of trusted root CA certificates to include the new root.

After the cross-signed certificate has been successfully generated and the new root
certificate or CA provider has been set up, the new root becomes the active one
and is immediately used for signing any new incoming certificate requests.

If we check the [list CA roots endpoint](/api/connect/ca.html#list-ca-root-certificates)
after the config update in the previous section, we can see both the old and new root
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

### External CA (Certificate Authority) Providers

#### Vault

Currently, the only supported external CA (Certificate Authority) provider is Vault. The 
Vault provider can be used by setting the `ca_provider = "vault"` field in the Connect 
configuration:

```hcl
connect {
    enabled = true
    ca_provider = "vault"
    ca_config {
        address = "http://localhost:8200"
        token = "..."
        root_pki_path = "connect-root"
        intermediate_pki_path = "connect-intermediate"
    }
}
```

The `root_pki_path` can be set to either a new or existing PKI backend; if no CA has been
initialized at the path, a new root CA will be generated. From this root PKI, Connect will
generate an intermediate CA at `intermediate_pki_path`. This intermediate CA is used so that
Connect can manage its lifecycle/rotation - it will never touch or overwrite any existing data
at `root_pki_path`. The intermediate CA is used for signing leaf certificates used by the 
services and proxies in Connect to verify identity.

To update the configuration for the Vault provider, the process is the same as for the Consul CA
provider above: use the [Update CA Configuration endpoint](/api/connect/ca.html#update-ca-configuration)
or the `consul connect ca set-config` command:

```bash
$ cat ca_config.json
{
  "Provider": "vault",
  "Config": {
    address = "http://localhost:8200"
    token = "..."
    root_pki_path = "connect-root-2"
    intermediate_pki_path = "connect-intermediate"
  }
}

$ consul connect ca set-config -config-file=ca_config.json

...

[INFO] connect: CA rotated to new root under provider "vault"
``` 

If the PKI backend at `connect-root-2` in this case has a different root certificate (or if it's
unmounted and hasn't been initialized), the rotation process will be triggered, as described above
in the [Root Certificate Rotation](#root-certificate-rotation) section.

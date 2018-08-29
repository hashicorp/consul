---
layout: "docs"
page_title: "Connect - Certificate Management"
sidebar_current: "docs-connect-ca-consul"
description: |-
  Consul ships with a built-in CA system so that Connect can be easily enabled out of the box. The built-in CA generates and stores the root certificate and private key on Consul servers. It can also be configured with a custom certificate and private key if needed.
---

# Built-In CA

Consul ships with a built-in CA system so that Connect can be
easily enabled out of the box. The built-in CA generates and stores the
root certificate and private key on Consul servers. It can also be
configured with a custom certificate and private key if needed.

If Connect is enabled and no CA provider is specified, the built-in
CA is the default provider used. The provider can be
[updated and rotated](/docs/connect/ca.html#root-certificate-rotation)
at any point to migrate to a new provider.

-> This page documents the specifics of the built-in CA provider.
Please read the [certificate management overview](/docs/connect/ca.html)
page first to understand how Consul manages certificates with configurable
CA providers.

## Configuration

The built-in CA provider has no required configuration. Enabling Connect
alone will configure the built-in CA provider and will automatically generate
a root certificate and private key:

```hcl
connect {
  enabled = true
}
```

A number of optional configuration options are supported. The
first key is the value used in API calls while the second key (after the `/`)
is used if configuring in an agent configuration file.

  * `PrivateKey` / `private_key` (`string: ""`) - A PEM-encoded private key
    for signing operations. This must match the private key used for the root
    certificate if it is manually specified. If this is blank, a private key
    is automatically generated.

  * `RootCert` / `root_cert` (`string: ""`) - A PEM-encoded root certificate
    to use. If this is blank, a root certificate is automatically generated
    using the private key specified. If this is specified, the certificate
    must be a valid
    [SPIFFE SVID signing certificate](https://github.com/spiffe/spiffe/blob/master/standards/X509-SVID.md)
    and the URI in the SAN must match the cluster identifier created at
    bootstrap with the ".consul" TLD. The cluster identifier can be found
    using the [CA List Roots endpoint](/api/connect/ca.html#list-ca-root-certificates).

There are also [common CA configuration options](/docs/agent/options.html#common-ca-config-options)
that are supported by all CA providers.

## Specifying a Custom Private Key and Root Certificate

By default, a root certificate and private key will be automatically
generated during the cluster's bootstrap. It is possible to configure
the Consul CA provider to use a specific private key and root certificate.
This is particularly useful if you have an external PKI system that doesn't
currently integrate with Consul directly.

To view the current CA configuration, use the [Get CA Configuration endpoint]
(/api/connect/ca.html#get-ca-configuration):

```bash
$ curl localhost:8500/v1/connect/ca/configuration
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

This is the default Connect CA configuration if nothing is explicitly set when
Connect is enabled - the PrivateKey and RootCert fields have not been set, so those have
been generated (as seen above in the roots list).

There are two ways to have the Consul CA use a custom private key and root certificate:
either through the `ca_config` section of the [Agent configuration]
(/docs/agent/options.html#connect_ca_config) (which can only be used during the cluster's
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
        "LeafCertTTL": "72h",
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
    "LeafCertTTL": "72h",
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
this way also triggered a certificate rotation.

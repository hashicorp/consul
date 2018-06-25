---
layout: "docs"
page_title: "Connect - Certificate Management"
sidebar_current: "docs-connect-ca-vault"
description: |-
  Consul can be used with Vault to manage and sign certificates. The Vault CA provider uses the Vault PKI secrets engine to generate and sign certificates.
---

# Vault as a Connect CA

Consul can be used with [Vault](https://www.vaultproject.io) to
manage and sign certificates.
The Vault CA provider uses the
[Vault PKI secrets engine](https://www.vaultproject.io/docs/secrets/pki/index.html)
to generate and sign certificates.

-> This page documents the specifics of the built-in CA provider.
Please read the [certificate management overview](/docs/connect/ca.html)
page first to understand how Consul manages certificates with configurable
CA providers.

## Requirements

Prior to using Vault as a CA provider for Consul, the following requirements
must be met:

  * **Vault 0.10.3 or later.** Consul uses URI SANs in the PKI engine which
    were introduced in Vault 0.10.3. Prior versions of Vault are not
    compatible with Connect.

## Configuration

The Vault CA is enabled by setting the `ca_provider` to `"vault"` and
setting the required configuration values. An example configuration
is shown below:

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

The set of configuration options is listed below. The
first key is the value used in API calls while the second key (after the `/`)
is used if configuring in an agent configuration file.

  * `Address` / `address` (`string: <required>`) - The address of the Vault
    server.

  * `Token` / `token` (`string: <required>`) - A token for accessing Vault.
    This is write-only and will not be exposed when reading the CA configuration.
    This token must have proper privileges for the PKI paths configured.

  * `RootPKIPath` / `root_pki_path` (`string: <required>`) - The path to
    a PKI secrets engine for the root certificate. If the path doesn't
    exist, Consul will attempt to mount and configure this automatically.

  * `IntermediatePKIPath` / `intermediate_pki_path` (`string: <required>`) -
    The path to a PKI secrets engine for the generated intermediate certificate.
    This certificate will be signed by the configured root PKI path. If this
    path doesn't exist, Consul will attempt to mount and configure this
    automatically.

## Root and Intermediate PKI Paths

The Vault CA provider uses two separately configured
[PKI secrets engines](https://www.vaultproject.io/docs/secrets/pki/index.html)
for managing Connect certificates.

The `RootPKIPath` is the PKI engine for the root certificate. Consul will
use this root certificate to sign the intermediate certificate. Consul will
never attempt to write or modify any data within the root PKI path.

The `IntermediatePKIPath` is the PKI engine used for storing the intermediate
signed with the root certificate. The intermediate is used to sign all leaf
certificates and Consul may periodically generate new intermediates for
automatic rotation. Therefore, Consul requires write access to this path.

If either path does not exist, then Consul will attempt to mount and
initialize it. This requires additional privileges by the Vault token in use.
If the paths already exist, Consul will use them as configured.

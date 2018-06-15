---
layout: "docs"
page_title: "Creating Certificates"
sidebar_current: "docs-guides-creating-certificates"
description: |-
  Learn how to create certificates for Consul.
---

# Creating Certificates

Correctly configuring TLS can be a complex process, especially given the wide
range of deployment methodologies. This guide will provide you with a
production ready TLS configuration.

~> Note that while Consul's TLS configuration will be production ready, key
   management and rotation is a complex subject not covered by this guide.
   [Vault][vault] is the suggested solution for key generation and management.

The first step to configuring TLS for Consul is generating certificates. In
order to prevent unauthorized cluster access, Consul requires all certificates
be signed by the same Certificate Authority (CA). This should be a _private_ CA
and not a public one like [Let's Encrypt][letsencrypt] as any certificate
signed by this CA will be allowed to communicate with the cluster.

~> Consul certificates may be signed by intermediate CAs as long as the root CA
   is the same. Append all intermediate CAs to the `cert_file`.

### Certificate Authority

There are a variety of tools for managing your own CA, [like the PKI secret
backend in Vault][vault-pki], but for the sake of simplicity this guide will
use [cfssl][cfssl]. You can generate a private CA certificate and key with
[cfssl][cfssl]:

```shell
$ # Generate a default CSR
$ cfssl print-defaults csr > ca-csr.json
```
Change the `key` field to use RSA with a size of 2048

```json
{
    "CN": "example.net",
    "hosts": [
        "example.net",
        "www.example.net"
    ],
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "US",
            "ST": "CA",
            "L": "San Francisco"
        }
    ]
}
```

```shell
$ # Generate the CA's private key and certificate
$ cfssl gencert -initca ca-csr.json | cfssljson -bare consul-ca
```

The CA key (`consul-ca-key.pem`) will be used to sign certificates for Consul
nodes and must be kept private. The CA certificate (`consul-ca.pem`) contains
the public key necessary to validate Consul certificates and therefore must be
distributed to every node that requires access.

### Node Certificates

Once you have a CA certificate and key you can generate and sign the
certificates Consul will use directly. TLS certificates commonly use the
fully-qualified domain name of the system being identified as the certificate's
Common Name (CN). However, hosts (and therefore hostnames and IPs) are often
ephemeral in Consul clusters.  Not only would signing a new certificate per
Consul node be difficult, but using a hostname provides no security or
functional benefits to Consul. To fulfill the desired security properties
(above) Consul certificates are signed with their region and role such as:

* `client.node.global.consul` for a client node in the `global` region
* `server.node.us-west.consul` for a server node in the `us-west` region

To create certificates for the client and server in the cluster with
[cfssl][cfssl], create the following configuration file as `cfssl.json` to increase the default certificate expiration time:

```json
{
  "signing": {
    "default": {
      "expiry": "87600h",
      "usages": [
        "signing",
        "key encipherment",
        "server auth",
        "client auth"
      ]
    }
  }
}
```

```shell
$ # Generate a certificate for the Consul server
$ echo '{"key":{"algo":"rsa","size":2048}}' | cfssl gencert -ca=consul-ca.pem -ca-key=consul-ca-key.pem -config=cfssl.json \
    -hostname="server.node.global.consul,localhost,127.0.0.1" - | cfssljson -bare server

# Generate a certificate for the Consul client
$ echo '{"key":{"algo":"rsa","size":2048}}' | cfssl gencert -ca=consul-ca.pem -ca-key=consul-ca-key.pem -config=cfssl.json \
    -hostname="client.node.global.consul,localhost,127.0.0.1" - | cfssljson -bare client

# Generate a certificate for the CLI
$ echo '{"key":{"algo":"rsa","size":2048}}' | cfssl gencert -ca=consul-ca.pem -ca-key=consul-ca-key.pem -profile=client \
    - | cfssljson -bare cli
```

Using `localhost` and `127.0.0.1` as subject alternate names (SANs) allows
tools like `curl` to be able to communicate with Consul's HTTP API when run on
the same host. Other SANs may be added including a DNS resolvable hostname to
allow remote HTTP requests from third party tools.

You should now have the following files:

* `cfssl.json` - cfssl configuration.
* `consul-ca.csr` - CA signing request.
* `consul-ca-key.pem` - CA private key. Keep safe!
* `consul-ca.pem` - CA public certificate.
* `cli.csr` - Consul CLI certificate signing request.
* `cli-key.pem` - Consul CLI private key.
* `cli.pem` - Consul CLI certificate.
* `client.csr` - Consul client node certificate signing request for the `global` region.
* `client-key.pem` - Consul client node private key for the `global` region.
* `client.pem` - Consul client node public certificate for the `global` region.
* `server.csr` - Consul server node certificate signing request for the `global` region.
* `server-key.pem` - Consul server node private key for the `global` region.
* `server.pem` - Consul server node public certificate for the `global` region.

Each Consul node should have the appropriate key (`-key.pem`) and certificate
(`.pem`) file for its region and role. In addition each node needs the CA's
public certificate (`consul-ca.pem`).

[cfssl]: https://cfssl.org/
[letsencrypt]: https://letsencrypt.org/
[vault]: https://www.vaultproject.io/
[vault-pki]: https://www.vaultproject.io/docs/secrets/pki/index.html

---
layout: "docs"
page_title: "Creating and Configuring TLS Certificates"
sidebar_current: "docs-guides-creating-certificates"
description: |-
  Learn how to create certificates for Consul.
---

# Creating and Configuring TLS Certificates

Setting you cluster up with TLS is an important step towards a secure
deployment. Correct TLS configuration is a prerequisite of our [Security
Model](/docs/internals/security.html). Correctly configuring TLS can be a
complex process however, especially given the wide range of deployment
methodologies. This guide will provide you with a production ready TLS
configuration.

~> More advanced topics like key management and rotation are not covered by this
guide. [Vault][vault] is the suggested solution for key generation and
management.

This guide has the following chapters:

1. [Creating Certificates](#creating-certificates)
1. [Configuring Agents](#configuring-agents)
1. [Configuring the Consul CLI for HTTPS](#configuring-the-consul-cli-for-https)
1. [Configuring the Consul UI for HTTPS](#configuring-the-consul-ui-for-https)

This guide is structured in way that you build knowledge with every step. It is
recommended to read the whole guide before starting with the actual work,
because you can save time if you are aware of some of the more advanced things
in Chapter [3](#configuring-the-consul-cli-for-https) and
[4](#configuring-the-consul-ui-for-https).

### Reference Material

- [Encryption](/docs/agent/encryption.html)
- [Security Model](/docs/internals/security.html)

## Creating Certificates

### Estimated Time to Complete

2 minutes

### Prerequisites

This guide assumes you have Consul 1.4.1 (or newer) in your PATH.

### Introduction

The first step to configuring TLS for Consul is generating certificates. In
order to prevent unauthorized cluster access, Consul requires all certificates
be signed by the same Certificate Authority (CA). This should be a _private_ CA
and not a public one like [Let's Encrypt][letsencrypt] as any certificate
signed by this CA will be allowed to communicate with the cluster.

### Step 1: Create a Certificate Authority

There are a variety of tools for managing your own CA, [like the PKI secret
backend in Vault][vault-pki], but for the sake of simplicity this guide will
use Consul's builtin TLS helpers:

```shell
$ consul tls ca create
==> Saved consul-agent-ca.pem
==> Saved consul-agent-ca-key.pem
```

The CA certificate (`consul-agent-ca.pem`) contains the public key necessary to
validate Consul certificates and therefore must be distributed to every node
that runs a consul agent.

~> The CA key (`consul-agent-ca-key.pem`) will be used to sign certificates for Consul
nodes and must be kept private. Possession of this key allows anyone to run Consul as
a trusted server and access all Consul data including ACL tokens.


### Step 2: Create individual Server Certificates

Create a server certificate for datacenter `dc1` and domain `consul`, if your
datacenter or domain is different please use the appropriate flags:

```shell
$ consul tls cert create -server
==> WARNING: Server Certificates grants authority to become a
    server and access all state in the cluster including root keys
    and all ACL tokens. Do not distribute them to production hosts
    that are not server nodes. Store them as securely as CA keys.
==> Using consul-agent-ca.pem and consul-agent-ca-key.pem
==> Saved dc1-server-consul-0.pem
==> Saved dc1-server-consul-0-key.pem
```

Please repeat this process until there is an *individual* certificate for each
server. The command can be called over and over again, it will automatically add
a suffix.

In order to authenticate Consul servers, servers are provided with a special
certificate - one that contains `server.dc1.consul` in the `Subject Alternative
Name`. If you enable
[`verify_server_hostname`](/docs/agent/options.html#verify_server_hostname),
only agents that provide such certificate are allowed to boot as a server.
Without `verify_server_hostname = true` an attacker could compromise a Consul
client agent and restart the agent as a server in order to get access to all the
data in your cluster! This is why server certificates are special, and only
servers should have them provisioned.

~> Server keys, like the CA key, must be kept private - they effectively allow
access to all Consul data.

### Step 3: Create Client Certificates

Create a client certificate:

```shell
$ consul tls cert create -client
==> Using consul-agent-ca.pem and consul-agent-ca-key.pem
==> Saved dc1-client-consul-0.pem
==> Saved dc1-client-consul-0-key.pem
```

Client certificates are also signed by your CA, but they do not have that
special `Subject Alternative Name` which means that if `verify_server_hostname`
is enabled, they cannot start as a server.

## Configuring Agents

### Prerequisites

For this section you need access to your existing or new Consul cluster and have
the certificates from the previous chapters available.

### Notes on example configurations

The example configurations from this as well as the following chapters are in
json. You can copy each one of the examples in its own file in a directory
([`-config-dir`](/docs/agent/options.html#_config_dir)) from where consul will
load all the configuration. This is just one way to do it, you can also put them
all into one file if you prefer that.

### Introduction

By now you have created the certificates you need to enable TLS in your cluster.
The next steps show how to configure TLS for a brand new cluster. If you already
have a cluster in production without TLS please see the [encryption
guide][guide] for the steps needed to introduce TLS without downtime.

### Step 1: Setup Consul servers with certificates

This step describes how to setup one of your consul servers, you want to make
sure to repeat the process for the other ones as well with their individual
certificates.

The following files need to be copied to your Consul server:

* `consul-agent-ca.pem`: CA public certificate.
* `dc1-server-consul-0.pem`: Consul server node public certificate for the `dc1` datacenter.
* `dc1-server-consul-0-key.pem`: Consul server node private key for the `dc1` datacenter.

Here is an example agent TLS configuration for Consul servers which mentions the
copied files:

```json
{
  "verify_incoming": true,
  "verify_outgoing": true,
  "verify_server_hostname": true,
  "ca_file": "consul-agent-ca.pem",
  "cert_file": "dc1-server-consul-0.pem",
  "key_file": "dc1-server-consul-0-key.pem",
  "ports": {
    "http": -1,
    "https": 8501
  }
}
```

This configuration disables the HTTP port to make sure there is only encryted
communication. Existing clients that are not yet prepared to talk HTTPS won't be
able to connect afterwards. This also affects builtin tooling like `consul
members` and the UI. The next chapters will demonstrate how to setup secure
access.

After a Consul agent restart, your servers should be only talking TLS.

### Step 2: Setup Consul clients with certificates

Now copy the following files to your Consul clients:

* `consul-agent-ca.pem`: CA public certificate.
* `dc1-client-consul-0.pem`: Consul client node public certificate.
* `dc1-client-consul-0-key.pem`: Consul client node private key.

Here is an example agent TLS configuration for Consul agents which mentions the
copied files:

```json
{
  "verify_incoming": true,
  "verify_outgoing": true,
  "verify_server_hostname": true,
  "ca_file": "consul-agent-ca.pem",
  "cert_file": "dc1-client-consul-0.pem",
  "key_file": "dc1-client-consul-0-key.pem",
  "ports": {
    "http": -1,
    "https": 8501
  }
}
```

This configuration disables the HTTP port to make sure there is only encryted
communication. Existing clients that are not yet prepared to talk HTTPS won't be
able to connect afterwards. This also affects builtin tooling like `consul
members` and the UI. The next chapters will demonstrate how to setup secure
access.

After a Consul agent restart, your agents should be only talking TLS.

## Configuring the Consul CLI for HTTPS

If your cluster is configured to only communicate via HTTPS, you will need to
create additional certificates in order to be able to continue to access the API
and the UI:

```shell
$ consul tls cert create -cli
==> Using consul-agent-ca.pem and consul-agent-ca-key.pem
==> Saved dc1-cli-consul-0.pem
==> Saved dc1-cli-consul-0-key.pem
```

If you are trying to get members of you cluster, the CLI will return an error:

```shell
$ consul members
Error retrieving members:
  Get http://127.0.0.1:8500/v1/agent/members?segment=_all:
  dial tcp 127.0.0.1:8500: connect: connection refused
$ consul members -http-addr="https://localhost:8501"
Error retrieving members:
  Get https://localhost:8501/v1/agent/members?segment=_all:
  x509: certificate signed by unknown authority
```

But it will work again if you provide the certificates you provided:

```shell
$ consul members -ca-file=consul-agent-ca.pem -client-cert=dc1-cli-consul-0.pem \
  -client-key=dc1-cli-consul-0-key.pem -http-addr="https://localhost:8501"
  Node     Address         Status  Type    Build     Protocol  DC   Segment
  ...
```

This process can be cumbersome to type each time, so the Consul CLI also
searches environment variables for default values. Set the following
environment variables in your shell:

```shell
$ export CONSUL_HTTP_ADDR=https://localhost:8501
$ export CONSUL_CACERT=consul-agent-ca.pem
$ export CONSUL_CLIENT_CERT=dc1-cli-consul-0.pem
$ export CONSUL_CLIENT_KEY=dc1-cli-consul-0-key.pem
```

* `CONSUL_HTTP_ADDR` is the URL of the Consul agent and sets the default for
  `-http-addr`.
* `CONSUL_CACERT` is the location of your CA certificate and sets the default
  for `-ca-file`.
* `CONSUL_CLIENT_CERT` is the location of your CLI certificate and sets the
  default for `-client-cert`.
* `CONSUL_CLIENT_KEY` is the location of your CLI key and sets the default for
  `-client-key`.

After these environment variables are correctly configured, the CLI will
respond as expected.

### Note on SANs for Server and Client Certificates

Using `localhost` and `127.0.0.1` as `Subject Alternative Names` in server
and client certificates allows tools like `curl` to be able to communicate with
Consul's HTTPS API when run on the same host. Other SANs may be added during
server/client certificates creation with `-additional-dnsname` or 
`-additional-ipaddress`to allow remote HTTPS requests from other hosts.

## Configuring the Consul UI for HTTPS

If your servers and clients are configured now like above, you won't be able to
access the builtin UI anymore. We recommend that you pick one (or two for
availability) Consul agent you want to run the UI on and follow the instructions
to get the UI up and running again.

### Step 1: Which interface to bind to?

Depending on your setup you might need to change to which interface you are
binding because thats `127.0.0.1` by default for the UI. Either via the
[`addresses.https`](/docs/agent/options.html#https) or
[client_addr](/docs/agent/options.html#client_addr) option which also impacts
the DNS server. The Consul UI is unproteced which means you need to put some
auth in front of it if you want to make it publicly available!

Binding to `0.0.0.0` should work:

```json
{
  "ui": true,
  "client_addr": "0.0.0.0",
  "enable_script_checks": false,
  "disable_remote_exec": true
}
```

~> Since your Consul agent is now available to the network, please make sure
that [`enable_script_checks`](/docs/agent/options.html#_enable_script_checks) is
set to `false` and
[`disable_remote_exec`](https://www.consul.io/docs/agent/options.html#disable_remote_exec)
is set to `true`.

### Step 2: verify_incoming_rpc

Your Consul agent will deny the connection straight away because
`verify_incoming` is enabled.

> If set to true, Consul requires that all incoming connections make use of TLS
> and that the client provides a certificate signed by a Certificate Authority
> from the ca_file or ca_path. This applies to both server RPC and to the HTTPS
> API.

Since the browser doesn't present a certificate signed by our CA, you cannot
access the UI. If you `curl` your HTTPS UI the following happens:

```shell
$ curl https://localhost:8501/ui/ -k -I
curl: (35) error:14094412:SSL routines:SSL3_READ_BYTES:sslv3 alert bad certificate
```

This is the Consul HTTPS server denying your connection because you are not
presenting a client certificate signed by your Consul CA. There is a combination
of options however that allows us to keep using `verify_incoming` for RPC, but
not for HTTPS:

```json
{
  "verify_incoming": false,
  "verify_incoming_rpc": true
}
```

~> This is the only time we are changing the value of the existing option
`verify_incoming` to false. Make sure to only change it on the agent running the
UI!

With the new configuration, it should work:

```shell
$ curl https://localhost:8501/ui/ -k -I
HTTP/2 200
...
```

### Step 3: Subject Alternative Name

This step will take care of setting up the domain you want to use to access the
Consul UI. Unless you only need to access the UI over localhost or 127.0.0.1 you
will need to go complete this step.

```shell
$ curl https://consul.example.com:8501/ui/ \
  --resolve 'consul.example.com:8501:127.0.0.1' \
  --cacert consul-agent-ca.pem
curl: (51) SSL: no alternative certificate subject name matches target host name 'consul.example.com'
...
```

The above command simulates a request a browser is making when you are trying to
use the domain `consul.example.com` to access your UI. The problem this time is
that your domain is not in `Subject Alternative Name` of the Certificate. We can
fix that by creating a certificate that has our domain:

```shell
$ consul tls cert create -server -additional-dnsname consul.example.com
...
```

And if you put your new cert into the configuration of the agent you picked to
serve the UI and restart Consul, it works now:

```shell
$ curl https://consul.example.com:8501/ui/ \
  --resolve 'consul.example.com:8501:127.0.0.1' \
  --cacert consul-agent-ca.pem -I
HTTP/2 200
...
```

### Step 4: Trust the Consul CA

So far we have provided curl with our CA so that it can verify the connection,
but if we stop doing that it will complain and so will our browser if you visit
your UI on https://consul.example.com:

```shell
$ curl https://consul.example.com:8501/ui/ \
  --resolve 'consul.example.com:8501:127.0.0.1'
curl: (60) SSL certificate problem: unable to get local issuer certificate
...
```

You can fix that by trusting your Consul CA (`consul-agent-ca.pem`) on your machine,
please use Google to find out how to do that on your OS.

```shell
$ curl https://consul.example.com:8501/ui/ \
  --resolve 'consul.example.com:8501:127.0.0.1' -I
HTTP/2 200
...
```

## Summary

When you have completed this guide, your Consul cluster will have TLS enabled
and will encrypt all RPC and HTTP traffic (assuming you disabled the HTTP port).
The other pre-requisites for a secure Consul deployment are:

* [Enable gossip encryption](/docs/agent/encryption.html#gossip-encryption)
* [Configure ACLs][acl] with default deny

[letsencrypt]: https://letsencrypt.org/
[vault]: https://www.vaultproject.io/
[vault-pki]: https://www.vaultproject.io/docs/secrets/pki/index.html
[guide]: /docs/agent/encryption.html#configuring-tls-on-an-existing-cluster
[acl]: /docs/guides/acl.html


---
layout: "docs"
page_title: "Encryption"
sidebar_current: "docs-agent-encryption"
description: |-
  The Consul agent supports encrypting all of its network traffic. The exact method of this encryption is described on the encryption internals page. There are two separate systems, one for gossip traffic and one for RPC.
---

# Encryption

The Consul agent supports encrypting all of its network traffic. The exact
method of this encryption is described on the
[encryption internals page](/docs/internals/security.html). There are two
separate systems, one for gossip traffic and one for RPC.

## Gossip Encryption

Enabling gossip encryption only requires that you set an encryption key when
starting the Consul agent. The key can be set by setting the `encrypt` parameter
in a configuration file for the agent. The key must be 16-bytes that are base64
encoded. The easiest method to obtain a cryptographically suitable key is by
using `consul keygen`.

```text
$ consul keygen
cg8StVXbQJ0gPvMd9o7yrg==
```

With that key, you can enable encryption on the agent. You can verify
encryption is enabled because the output will include "Encrypted: true".

```text
$ cat encrypt.json
{"encrypt": "cg8StVXbQJ0gPvMd9o7yrg=="}

$ consul agent -data=/tmp/consul -config-file encrypt.json
==> Starting Consul agent...
==> Starting Consul agent RPC...
==> Consul agent running!
         Node name: 'Armons-MacBook-Air.local'
        Datacenter: 'dc1'
    Advertise addr: '10.1.10.12'
          RPC addr: '127.0.0.1:8400'
         HTTP addr: '127.0.0.1:8500'
          DNS addr: '127.0.0.1:8600'
         Encrypted: true
            Server: false (bootstrap: false)
...
```

All nodes within a Consul cluster must share the same encryption key in
order to send and receive cluster information.

# RPC Encryption with TLS

Consul supports using TLS to verify the authenticity of servers and clients. For this
to work, Consul requires that all clients and servers have key pairs that are generated
by a single Certificate Authority. This can be a private CA, used only internally. The
CA then signs keys for each of the agents. [Here](https://langui.sh/2009/01/18/openssl-self-signed-ca/)
is a tutorial on generating both a CA and signing keys using OpenSSL. Client certificates
must have extended key usage enabled for client and server authentication.

There are a number of things to consider when setting up TLS for Consul. Either we can
use TLS just to verify the authenticity of the servers, or we can also verify the authenticity
of clients. The former can be used to prevent unauthorized access. This behavior is controlled
using either the `verify_incoming` and `verify_outgoing` [options](/docs/agent/options.html).

If `verify_outgoing` is set, then agents verify the authenticity of Consuls for outgoing
connections. This means server nodes must present a certificate signed by the `ca_file` that
the agent has. This option must be set on all agents, and there must be a `ca_file` provided
to check the certificate against. If this is set, then all server nodes must have an appropriate
key pair set using `cert_file` and `key_file`.

If `verify_incoming` is set, then the servers verify the authenticity of all incoming
connections. Servers will also disallow any non-TLS connections. If this is set, then all
clients must have a valid key pair set using `cert_file` and `key_file`. To force clients to
use TLS, `verify_outgoing` must also be set.

TLS is used to secure the RPC calls between agents, but gossip between nodes is done over UDP
and is secured using a symmetric key. See above for enabling gossip encryption.


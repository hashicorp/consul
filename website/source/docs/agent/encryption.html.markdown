---
layout: "docs"
page_title: "Encryption"
sidebar_current: "docs-agent-encryption"
---

# Encryption

The Consul agent supports encrypting all of its network traffic. The exact
method of this encryption is described on the
[encryption internals page](/docs/internals/security.html).

## Enabling Encryption

Enabling encryption only requires that you set an encryption key when
starting the Consul agent. The key can be set using the `-encrypt` flag
on `consul agent` or by setting the `encrypt_key` in a configuration file.
It is advisable to put the key in a configuration file to avoid other users
from being able to discover it by inspecting running processes.
The key must be 16-bytes that are base64 encoded. The easiest method to
obtain a cryptographically suitable key is by using `consul keygen`.

```
$ consul keygen
cg8StVXbQJ0gPvMd9o7yrg==
```

With that key, you can enable encryption on the agent. You can verify
encryption is enabled because the output will include "Encrypted: true".

```
$ consul agent -data=/tmp/consul -encrypt=cg8StVXbQJ0gPvMd9o7yrg==
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


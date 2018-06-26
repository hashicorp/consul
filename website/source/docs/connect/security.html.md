---
layout: "docs"
page_title: "Connect - Security"
sidebar_current: "docs-connect-security"
description: |-
  Connect enables secure service-to-service communication over mutual TLS. This
  provides both in-transit data encryption as well as authorization. This page
  will document how to secure Connect.
---

# Connect Security

Connect enables secure service-to-service communication over mutual TLS. This
provides both in-transit data encryption as well as authorization. This page
will document how to secure Connect. For a full security model reference,
see the dedicated [Consul security model](/docs/internals/security.html) page.

Connect will function in any Consul configuration. However, unless the checklist
below is satisfied, Connect is not providing the security guarantees it was
built for. The checklist below can be incrementally adopted towards full
security if you prefer to operate in less secure models initially.

~> **Warning**: The checklist below should not be considered exhaustive. Please
read and understand the [Consul security model](/docs/internals/security.html)
in depth to assess whether your deployment satisfies the security requirements
of Consul.

## Checklist

### ACLs Enabled with Default Deny

Consul must be configured to use ACLs with a default deny policy. This forces
all requests to have explicit anonymous access or provide an ACL token. The
configuration also forces all service-to-service communication to be explicitly
whitelisted via an allow [intention](/docs/connect/intentions.html).

To learn how to enable ACLs, please see the
[guide on ACLs](/docs/guides/acl.html).

**If ACLs are enabled but are in default allow mode**, then services will be
able to communicate by default. Additionally, if a proper anonymous token
is not configured, this may allow anyone to edit intentions. We do not recommend
this. **If ACLs are not enabled**, deny intentions will still be enforced, but anyone
may edit intentions. This renders the security of the created intentions
effectively useless.

### TCP and UDP Encryption Enabled

TCP and UDP encryption must be enabled to prevent plaintext communication
between Consul agents. At a minimum, `verify_outgoing` should be enabled
to verify server authenticity with each server having a unique TLS certificate.
`verify_incoming` provides additional agent verification, but doesn't directly
affect Connect since requests must also always contain a valid ACL token.
Clients calling Consul APIs should be forced over encrypted connections.

See the [Consul agent encryption page](/docs/agent/encryption.html) to
learn more about configuring agent encryption.

**If encryption is not enabled**, a malicious actor can sniff network
traffic or perform a man-in-the-middle attack to steal ACL tokens, always
authorize connections, etc.

### Prevent Unauthorized Access to the Config and Data Directories

The configuration and data directories of the Consul agent on both
clients and servers should be protected from unauthorized access. This
protection must be done outside of Consul via access control systems provided
by your target operating system.

The [full Consul security model](/docs/internals/security.html) explains the
risk of unauthorized access for both client agents and server agents. In
general, the blast radius of unauthorized access for client agent directories
is much smaller than servers. However, both must be protected against
unauthorized access.

### Prevent Non-Connect Traffic to Services

For services that are using
[proxies](/docs/connect/proxies.html)
(are not [natively integrated](/docs/connect/native.html)),
network access via their unencrypted listeners must be restricted
to only the proxy. This requires at a minimum restricting the listener
to bind to loopback only. More complex solutions may involve using
network namespacing techniques provided by the underlying operating system.

For scenarios where multiple services are running on the same machine
without isolation, these services must all be trusted. We call this the
**trusted multi-tenancy** deployment model. Any service could theoretically
connect to any other service via the loopback listener, bypassing Connect
completely. In this scenario, all services must be trusted _or_ isolation
mechanisms must be used.

For developer or operator access to a service, we recommend
using a local Connect proxy. This is documented in the
[development and debugging guide](/docs/connect/dev.html).

**If non-proxy traffic can communicate with the service**, this traffic
will not be encrypted or authorized via Connect.

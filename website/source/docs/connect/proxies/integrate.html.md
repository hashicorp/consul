---
layout: "docs"
page_title: "Connect - Proxy Integration"
sidebar_current: "docs-connect-proxies-integrate"
description: |-
  A Connect-aware proxy enables unmodified applications to use Connect. A per-service proxy sidecar transparently handles inbound and outbound service connections, automatically wrapping and verifying TLS connections.
---

# Connect Custom Proxy Integration

Any proxy can be extended to support Connect. Consul ships with a built-in
proxy for a good development and out of the box experience, but understand
that production users will require other proxy solutions.

A proxy must serve one or both of the following two roles: it must accept
inbound connections or establish outbound connections identified as a
particular service. One or both of these may be implemented depending on
the case, although generally both must be supported.

## Accepting Inbound Connections

For inbound connections, the proxy must accept TLS connections on some port.
The certificate served should be created by the
[`/v1/agent/connect/ca/leaf/`](/api/agent/connect.html) API endpoint.
The client certificate should be validated against the root certificates
provided by the
[`/v1/agent/connect/ca/roots`](/api/agent/connect.html) endpoint.
After validating the client certificate from the caller, the proxy should
call the
[`/v1/agent/connect/authorize`](/api/agent/connect.html) endpoint to
authorize the connection.

All of these API endpoints operate on agent-local data that is updated
in the background. The leaf and roots should be updated in the background
by the proxy, but the authorize endpoint is expected to be called in the
connection path. The endpoints introduce only microseconds of additional
latency on the connection.

The leaf and root cert endpoints support blocking queries. These should be
used if possible to get near-immediate updates for root cert rotations,
leaf expiry, etc.

## Establishing Outbound Connections

For outbound connections, the proxy should communicate to a
Connect-capable endpoint for a service and provide a client certificate
from the
[`/v1/agent/connect/ca/leaf/`](/api/agent/connect.html) API endpoint.
The certificate served by the remote endpoint can be verified against the
root certificates from the
[`/v1/agent/connect/ca/roots`](/api/agent/connect.html) endpoint.

## Managed Mode Support

Any custom proxy can also run as a [custom managed proxy](/docs/connect/proxies.html#custom-managed-proxy).
If you want the proxy you're integrating to support this mode, then it should accept
two environment variables that Consul populates on process startup. These
are both required to make the necessary API requests for configuration.

  * `CONSUL_PROXY_TOKEN` - The ACL token to use for all requests to proxy-related
    API endpoints.

  * `CONSUL_PROXY_ID` - The service ID for requesting configuration for the
    proxy from [`/v1/agent/connect/proxy/`](/api/agent/connect.html).

---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-client"
description: |-
  These options configure the Consul client 
---

# Client Options

These options configure the Consul client. See also the datacenter options. 

* <a name="client_addr"></a><a href="#client_addr">`client_addr`</a> Equivalent to the
  `-client` command-line flag.The address to which
  Consul will bind client interfaces, including the HTTP and DNS servers. By
  default, this is "127.0.0.1", allowing only loopback connections. In Consul
  1.0 and later this can be set to a space-separated list of addresses to bind
  to, or a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template that can potentially resolve to multiple addresses.

* <a name="limits"></a><a href="#limits">`limits`</a> Available in Consul 0.9.3 and later, this
  is a nested object that configures limits that are enforced by the agent. Currently, this only
  applies to agents in client mode, not Consul servers. The following parameters are available:

    *   <a name="rpc_rate"></a><a href="#rpc_rate">`rpc_rate`</a> - Configures the RPC rate
        limiter by setting the maximum request rate that this agent is allowed to make for RPC
        requests to Consul servers, in requests per second. Defaults to infinite, which disables
        rate limiting.
    *   <a name="rpc_rate"></a><a href="#rpc_max_burst">`rpc_max_burst`</a> - The size of the token
        bucket used to recharge the RPC rate limiter. Defaults to 1000 tokens, and each token is
        good for a single RPC call to a Consul server. See https://en.wikipedia.org/wiki/Token_bucket
        for more details about how token bucket rate limiters operate.

* <a name="session_ttl_min"></a><a href="#session_ttl_min">`session_ttl_min`</a>
  The minimum allowed session TTL. This ensures sessions are not created with
  TTL's shorter than the specified limit. It is recommended to keep this limit
  at or above the default to encourage clients to send infrequent heartbeats.
  Defaults to 10s.
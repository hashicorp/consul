---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-enterprise"
description: |-
  The 
---

* <a name="segment"></a><a href="#segment">`segment`</a> (Enterprise-only) Equivalent to the
  [`-segment` command-line flag](#_segment).

* <a name="segments"></a><a href="#segments">`segments`</a> (Enterprise-only) This is a list of nested objects that allows setting
  the bind/advertise information for network segments. This can only be set on servers. See the
  [Network Segments Guide](https://learn.hashicorp.com/consul/day-2-operations/network-segments) for more details.
    * <a name="segment_name"></a><a href="#segment_name">`name`</a> - The name of the segment. Must be a string between
    1 and 64 characters in length.
    * <a name="segment_bind"></a><a href="#segment_bind">`bind`</a> - The bind address to use for the segment's gossip layer.
    Defaults to the [`-bind`](#_bind) value if not provided.
    * <a name="segment_port"></a><a href="#segment_port">`port`</a> - The port to use for the segment's gossip layer (required).
    * <a name="segment_advertise"></a><a href="#segment_advertise">`advertise`</a> - The advertise address to use for the
    segment's gossip layer. Defaults to the [`-advertise`](#_advertise) value if not provided.
    * <a name="segment_rpc_listener"></a><a href="#segment_rpc_listener">`rpc_listener`</a> - If true, a separate RPC listener will
    be started on this segment's [`-bind`](#_bind) address on the rpc port. Only valid if the segment's bind address differs from the
    [`-bind`](#_bind) address. Defaults to false.


* <a name="non_voting_server"></a><a href="#non_voting_server">`non_voting_server`</a> - Equivalent to the
  [`-non-voting-server` command-line flag](#_non_voting_server).
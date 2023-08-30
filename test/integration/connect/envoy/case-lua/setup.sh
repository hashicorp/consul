#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s2"
Protocol = "http"
EnvoyExtensions = [
  {
    Name = "builtin/lua",
    Arguments = {
      ProxyType = "connect-proxy"
      Listener = "inbound"
      Script = <<-EOF
function envoy_on_request(request_handle)
  meta = request_handle:streamInfo():dynamicMetadata()
  m = meta:get("consul")
  request_handle:headers():add("x-consul-service", m["service"])
  request_handle:headers():add("x-consul-namespace", m["namespace"])
  request_handle:headers():add("x-consul-datacenter", m["datacenter"])
  request_handle:headers():add("x-consul-trust-domain", m["trust-domain"])
end
      EOF
    }
  }
]
'

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s1"
Protocol = "tcp"
EnvoyExtensions = [
  {
    Name = "builtin/lua",
    Arguments = {
      ProxyType = "connect-proxy"
      Listener = "inbound"
      Script = <<-EOF
function envoy_on_request(request_handle)
  meta = request_handle:streamInfo():dynamicMetadata()
  m = meta:get("consul")
  request_handle:headers():add("x-consul-service", m["service"])
  request_handle:headers():add("x-consul-namespace", m["namespace"])
  request_handle:headers():add("x-consul-datacenter", m["datacenter"])
  request_handle:headers():add("x-consul-trust-domain", m["trust-domain"])
end
      EOF
    }
  }
]
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry alpha '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry alpha '
kind = "exported-services"
name = "default"
services = [
  {
    name = "s2"
    consumers = [
      {
        peer = "alpha-to-primary"
      }
    ]
  }
]
'

upsert_config_entry alpha '
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

register_services alpha

gen_envoy_bootstrap s2 19002 alpha
gen_envoy_bootstrap mesh-gateway 19003 alpha true

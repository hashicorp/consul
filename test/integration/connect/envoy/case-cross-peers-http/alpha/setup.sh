#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}

function docker_exec {
  if ! docker.exe exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}

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

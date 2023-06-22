#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}


upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind            = "service-resolver"
name            = "s2"
default_subset  = "v2"
connect_timeout = "30s"
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
'

register_services primary

# s2 is retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2-v1 19001
gen_envoy_bootstrap s2-v2 19002
gen_envoy_bootstrap s2 19003

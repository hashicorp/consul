#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s3"
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
redirect {
  service        = "s3"
  service_subset = "v2"
}
'

register_services primary

# s2, s3, and s3-v1 are retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
gen_envoy_bootstrap s3-v1 19002
gen_envoy_bootstrap s3-v2 19003
gen_envoy_bootstrap s3 19004

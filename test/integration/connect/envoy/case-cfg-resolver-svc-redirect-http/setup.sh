#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
redirect {
  service = "s3"
}
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary
gen_envoy_bootstrap s3 19002 primary

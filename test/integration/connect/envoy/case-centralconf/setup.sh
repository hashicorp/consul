#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  envoy_prometheus_bind_addr = "0.0.0.0:1234"
}
'

upsert_config_entry primary '
kind     = "service-defaults"
name     = "s1"
protocol = "http"
'

upsert_config_entry primary '
kind     = "service-defaults"
name     = "s2"
protocol = "http"
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary


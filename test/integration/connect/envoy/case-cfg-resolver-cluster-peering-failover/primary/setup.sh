#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "tcp"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
failover = {
  "*" = {
    targets = [{ peer = "primary-to-alpha" }]
  }
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "virtual-s2"
redirect = {
  service = "s2"
  peer    = "primary-to-alpha"
}
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

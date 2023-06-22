#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

source helpers.bash


upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "tcp"
}
'

upsert_config_entry primary '
kind = "ingress-gateway"
name = "ingress-gateway"
listeners = [
  {
    protocol = "tcp"
    port     = 10000
    services = [
      {
        name = "s2"
      }
    ]
  }
]
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

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s2 19001 primary

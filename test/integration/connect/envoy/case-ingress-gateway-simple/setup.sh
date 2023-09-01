#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry primary '
kind = "ingress-gateway"
name = "ingress-gateway"
Defaults {
  MaxConnections        = 10
  MaxPendingRequests    = 20
  MaxConcurrentRequests = 30
  PassiveHealthCheck {
    Interval     = 5000000000
  }
}
listeners = [
  {
    port     = 9999
    protocol = "tcp"
    services = [
      {
        name               = "s1"
        MaxConnections     = 100
        MaxPendingRequests = 200
      }
    ]
  }
]
'

register_services primary

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

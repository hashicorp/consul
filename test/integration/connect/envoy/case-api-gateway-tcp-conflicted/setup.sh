#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    port = 9999
    protocol = "tcp"
    name = "listener"
  }
]
'

upsert_config_entry primary '
kind = "tcp-route"
name = "api-gateway-route-1"
services = [
  {
    name = "s1"
  }
]
parents = [
  {
    name = "api-gateway"
  }
]
'

upsert_config_entry primary '
kind = "tcp-route"
name = "api-gateway-route-2"
services = [
  {
    name = "s2"
  }
]
parents = [
  {
    kind = "api-gateway"
    name = "api-gateway"
  }
]
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

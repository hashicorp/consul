#!/bin/bash

set -euo pipefail

upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    port = 9999
    protocol = "tcp"
  }
]
'

upsert_config_entry primary '
kind = "tcp-route"
name = "api-gateway-route"
services = [
  {
    name = "s1"
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
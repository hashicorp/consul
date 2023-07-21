#!/bin/bash

set -euo pipefail

upsert_config_entry primary '
kind = "terminating-gateway"
name = "terminating-gateway"
services = [
  {
    name = "s2"
  }
]
'

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind           = "service-resolver"
name           = "s2"
default_subset = "v1"
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

# terminating gateway will act as s2's proxy
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s3 19001
gen_envoy_bootstrap terminating-gateway 20000 primary true

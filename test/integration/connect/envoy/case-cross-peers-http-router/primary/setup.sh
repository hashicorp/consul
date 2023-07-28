#!/bin/bash

set -euo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap mesh-gateway 19001 primary true

#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

set -eEuo pipefail

# Configure proxy-defaults with max_request_headers_kb setting
upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  max_request_headers_kb = 96
}
'

# Configure service defaults for HTTP protocol
upsert_config_entry primary '
kind = "service-defaults"
name = "s1"
protocol = "http"
'

upsert_config_entry primary '
kind = "service-defaults" 
name = "s2"
protocol = "http"
'

# Register services
register_services primary

# Generate Envoy bootstrap configs
gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

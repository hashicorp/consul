#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Configure service defaults for s2 with max_request_headers_kb
upsert_config_entry primary '
kind = "service-defaults"
name = "s2"
protocol = "http"
MaxRequestHeadersKB = 96
'

# Configure terminating gateway
upsert_config_entry primary '
kind = "terminating-gateway"
name = "terminating-gateway"
services = [
  {
    name = "s2"
  }
]
'

register_services primary

gen_envoy_bootstrap terminating-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
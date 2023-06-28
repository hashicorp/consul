#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

upsert_config_entry primary '
kind = "terminating-gateway"
name = "terminating-gateway"
services = [
  {
    name = "s4"
  }
]
'

upsert_config_entry primary '
kind     = "service-defaults"
name     = "s4"
protocol = "http"
'

register_services primary

gen_envoy_bootstrap terminating-gateway 20000 primary true
gen_envoy_bootstrap s1 19000

#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


source helpers.bash

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

register_services primary

gen_envoy_bootstrap terminating-gateway 20000 primary true
gen_envoy_bootstrap s1 19000

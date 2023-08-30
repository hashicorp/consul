#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

# Copy lambda config files into the register dir
find ${CASE_DIR} -maxdepth 1 -name '*_l*.json' -type f -exec cp -f {} workdir/${CLUSTER}/register \;

upsert_config_entry primary '
kind = "terminating-gateway"
name = "terminating-gateway"
services = [
  {
    name = "l2"
  }
]
'

register_services primary
register_lambdas primary

# wait for Lambda config entries
wait_for_config_entry service-defaults l1
wait_for_config_entry service-defaults l2

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap terminating-gateway 20000 primary true

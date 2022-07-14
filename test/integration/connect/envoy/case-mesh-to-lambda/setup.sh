#!/bin/bash

set -eEuo pipefail

# Copy lambda config files into the register dir
# TODO: not sure if we want to make this step more generic and add it to the init_workdir func?
find ${CASE_DIR} -maxdepth 1 -name '*lambda_*.json' -type f -exec cp -f {} workdir/${CLUSTER}/register \;

# wait for tgw config entry
wait_for_config_entry terminating-gateway terminating-gateway

register_services primary
register_lambdas primary

# wait for Lambda config entries
wait_for_config_entry service-defaults l1
wait_for_config_entry service-defaults l2

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap terminating-gateway 20000 primary true

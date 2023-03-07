#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry terminating-gateway terminating-gateway
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-resolver s2

register_services primary

# terminating gateway will act as s2's proxy
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s3 19001
gen_envoy_bootstrap terminating-gateway 20000 primary true

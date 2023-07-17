#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry terminating-gateway terminating-gateway

register_services primary

gen_envoy_bootstrap terminating-gateway 20000 primary true
gen_envoy_bootstrap s1 19000

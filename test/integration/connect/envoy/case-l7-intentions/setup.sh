#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry service-defaults s2
wait_for_config_entry service-intentions s2

register_services primary

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

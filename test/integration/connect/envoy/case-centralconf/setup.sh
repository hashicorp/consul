#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-defaults s1
wait_for_config_entry service-defaults s2

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

export REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 s2-sidecar-proxy"

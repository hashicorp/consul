#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry ingress-gateway ingress-gateway
wait_for_config_entry proxy-defaults global

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

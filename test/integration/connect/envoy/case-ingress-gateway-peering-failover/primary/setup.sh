#!/bin/bash

set -euo pipefail

wait_for_config_entry ingress-gateway ingress-gateway
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-resolver s2
wait_for_config_entry service-resolver virtual-s2

register_services primary

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s2 19001 primary
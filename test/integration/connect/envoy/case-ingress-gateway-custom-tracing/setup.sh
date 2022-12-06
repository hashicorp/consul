#!/bin/bash

set -euo pipefail

wait_for_config_entry ingress-gateway ingress-gateway-all-0
wait_for_config_entry ingress-gateway ingress-gateway-client-100
wait_for_config_entry ingress-gateway ingress-gateway-overall-0
wait_for_config_entry ingress-gateway ingress-gateway-overall-100

register_services primary

gen_envoy_bootstrap ingress-gateway-all-0 20000 primary true
gen_envoy_bootstrap ingress-gateway-client-0 20001 primary true
gen_envoy_bootstrap ingress-gateway-overall-0 20002 primary true
gen_envoy_bootstrap ingress-gateway-overall-100 20003 primary true
gen_envoy_bootstrap s1 19000
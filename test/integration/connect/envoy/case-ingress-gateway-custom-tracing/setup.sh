#!/bin/bash

set -euo pipefail

wait_for_config_entry ingress-gateway ingress-gateway-random-sampling-0
wait_for_config_entry ingress-gateway ingress-gateway-random-sampling-100
wait_for_config_entry ingress-gateway ingress-gateway-client-sampling-0
wait_for_config_entry ingress-gateway ingress-gateway-client-sampling-100

register_services primary

gen_envoy_bootstrap ingress-gateway-random-sampling-0 20000 primary true
gen_envoy_bootstrap ingress-gateway-random-sampling-100 20001 primary true
gen_envoy_bootstrap ingress-gateway-client-sampling-0 20002 primary true
gen_envoy_bootstrap ingress-gateway-client-sampling-100 20003 primary true

gen_envoy_bootstrap s1 19000

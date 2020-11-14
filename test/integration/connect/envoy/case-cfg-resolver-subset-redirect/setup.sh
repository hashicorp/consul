#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-resolver s2
wait_for_config_entry service-resolver s3

register_services primary

# s2, s3, and s3-v1 are retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
gen_envoy_bootstrap s3-v1 19002
gen_envoy_bootstrap s3-v2 19003
gen_envoy_bootstrap s3 19004

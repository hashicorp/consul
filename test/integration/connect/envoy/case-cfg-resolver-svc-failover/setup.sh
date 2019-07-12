#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-resolver s2
wait_for_config_entry service-resolver s3

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001    # unused
gen_envoy_bootstrap s3 19002    # unused
gen_envoy_bootstrap s3-v1 19003 # unused
gen_envoy_bootstrap s3-v2 19004

export REQUIRED_SERVICES="
s1 s1-sidecar-proxy
s2 s2-sidecar-proxy
s3 s3-sidecar-proxy
s3-v1 s3-v1-sidecar-proxy
s3-v2 s3-v2-sidecar-proxy
"

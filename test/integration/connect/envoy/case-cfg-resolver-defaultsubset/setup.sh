#!/bin/bash

set -euo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-resolver s2

# s2 is retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2-v1 19001
gen_envoy_bootstrap s2-v2 19002
gen_envoy_bootstrap s2 19003

export REQUIRED_SERVICES="
s1 s1-sidecar-proxy
s2 s2-sidecar-proxy
s2-v1 s2-v1-sidecar-proxy
s2-v2 s2-v2-sidecar-proxy
"

#!/bin/bash

set -euo pipefail

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap mesh-gateway 19001 primary true

wait_for_config_entry proxy-defaults global

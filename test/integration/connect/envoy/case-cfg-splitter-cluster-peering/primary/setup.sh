#!/bin/bash

set -euo pipefail

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

wait_for_config_entry proxy-defaults global

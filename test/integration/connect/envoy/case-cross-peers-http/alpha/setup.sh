#!/bin/bash

set -euo pipefail

register_services alpha

gen_envoy_bootstrap s2 19002 alpha
gen_envoy_bootstrap mesh-gateway 19003 alpha true

wait_for_config_entry proxy-defaults global alpha
wait_for_config_entry exported-services default alpha

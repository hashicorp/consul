#!/bin/bash

set -euo pipefail

register_services primary

# wait for service registration
wait_for_agent_service_register s1
wait_for_agent_service_register s2
wait_for_agent_service_register s2-v1

# force s2-v1 into a warning state
set_ttl_check_state service:s2-v1 warn

# wait for bootstrap to apply config entries
wait_for_config_entry proxy-defaults global
wait_for_config_entry service-resolver s2

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
gen_envoy_bootstrap s2-v1 19002

#!/bin/bash

set -eEuo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry service-defaults s2
wait_for_config_entry service-resolver s2

# also wait for replication to make it to the remote dc
wait_for_config_entry service-defaults s2 secondary
wait_for_config_entry service-resolver s2 secondary

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

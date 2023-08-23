#!/bin/bash

set -eEuo pipefail

# Setup deny intention
setup_upsert_l4_intention s1 s2 deny

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

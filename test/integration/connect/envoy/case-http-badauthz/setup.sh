#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

register_services primary

# Setup deny intention
setup_upsert_l4_intention s1 s2 deny

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

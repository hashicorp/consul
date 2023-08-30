#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry service-defaults s2 secondary
wait_for_config_entry service-resolver s2 secondary

register_services secondary

gen_envoy_bootstrap s2 19002 secondary
gen_envoy_bootstrap mesh-gateway 19003 secondary true
retry_default docker_consul secondary curl -s  "http://localhost:8500/v1/catalog/service/consul?dc=primary" >/dev/null

#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}

function docker_exec {
  if ! docker.exe exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}

set -euo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind           = "service-resolver"
name           = "s2"
default_subset = "test"
subsets = {
  "test" = {
    only_passing = true
  }
}
'

register_services primary

# wait for service registration
wait_for_agent_service_register s1
wait_for_agent_service_register s2
wait_for_agent_service_register s2-v1

# force s2-v1 into a warning state
set_ttl_check_state service:s2-v1 warn

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
gen_envoy_bootstrap s2-v1 19002

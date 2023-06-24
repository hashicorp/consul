#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

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

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}



upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
redirect {
  service = "s3"
}
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary
gen_envoy_bootstrap s3 19002 primary

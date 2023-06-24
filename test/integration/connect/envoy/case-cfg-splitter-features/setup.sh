#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

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
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
'

upsert_config_entry primary '
kind = "service-splitter"
name = "s2"
splits = [
  {
    weight         = 50,
    service_subset = "v2"
    request_headers {
      set {
        x-split-leg = "v2"
      }
      remove = ["x-bad-req"]
    }
    response_headers {
      add {
        x-svc-version = "v2"
      }
      remove = ["x-bad-resp"]
    }
  },
  {
    weight         = 50,
    service_subset = "v1"
    request_headers {
      set {
        x-split-leg = "v1"
      }
      remove = ["x-bad-req"]
    }
    response_headers {
      add {
        x-svc-version = "v1"
      }
      remove = ["x-bad-resp"]
    }
  },
]
'

register_services primary

# s2 is retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2-v1 19001
gen_envoy_bootstrap s2-v2 19002
gen_envoy_bootstrap s2 19003

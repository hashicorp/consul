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
kind = "ingress-gateway"
name = "ingress-gateway"
listeners = [
  {
    port     = 9999
    protocol = "http"
    services = [
      {
        name = "router"
        request_headers {
          add {
            x-foo        = "bar-req"
            x-existing-1 = "appended-req"
          }
          set {
            x-existing-2 = "replaced-req"
            x-client-ip  = "%DOWNSTREAM_REMOTE_ADDRESS_WITHOUT_PORT%"
          }
          remove = ["x-bad-req"]
        }
        response_headers {
          add {
            x-foo        = "bar-resp"
            x-existing-1 = "appended-resp"
          }
          set {
            x-existing-2 = "replaced-resp"
          }
          remove = ["x-bad-resp"]
        }
      }
    ]
  }
]
'

upsert_config_entry primary '
kind = "service-router"
// This is a "virtual" service name and will not have a backing
// service definition. It must match the name defined in the ingress
// configuration.
name = "router"
routes = [
  {
    match {
      http {
        path_prefix = "/s1/"
      }
    }
    destination {
      service        = "s1"
      prefix_rewrite = "/"
    }
  },
  {
    match {
      http {
        path_prefix = "/s2/"
      }
    }
    destination {
      service        = "s2"
      prefix_rewrite = "/"
    }
  }
]
'

register_services primary

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

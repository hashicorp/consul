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
kind     = "service-defaults"
name     = "s2"
protocol = "http"
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s2"
sources {
  name = "s1"
  permissions = [
    // paths
    {
      action = "allow"
      http { path_exact = "/exact" }
    },
    {
      action = "allow"
      http { path_prefix = "/prefix" }
    },
    {
      action = "allow"
      http { path_regex = "/reg[ex]{2}" }
    },
    // headers
    {
      action = "allow"
      http {
        path_exact = "/hdr-present"
        header = [{
          name    = "x-test-debug"
          present = true
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-exact"
        header = [{
          name  = "x-test-debug"
          exact = "exact"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-prefix"
        header = [{
          name   = "x-test-debug"
          prefix = "prefi"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-suffix"
        header = [{
          name   = "x-test-debug"
          suffix = "uffix"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-regex"
        header = [{
          name  = "x-test-debug"
          regex = "reg[ex]{2}"
        }]
      }
    },
    // methods
    {
      action = "allow"
      http {
        path_exact = "/method-match"
        methods    = ["GET", "PUT"]
      }
    }
  ]
}
'

register_services primary

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

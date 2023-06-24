#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

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

  echo "$BODY" | docker_consul "$DC" consul config write -
}

set -euo pipefail

upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "http"
    hostname = "*.consul.example"
  },
  {
    name = "listener-two"
    port = 9998
    protocol = "http"
    hostname = "foo.bar.baz"
  },
  {
    name = "listener-three"
    port = 9997
    protocol = "http"
    hostname = "*.consul.example"
  },
  {
    name = "listener-four"
    port = 9996
    protocol = "http"
    hostname = "*.consul.example"
  },
  {
    name = "listener-five"
    port = 9995
    protocol = "http"
    hostname = "foo.bar.baz"
  }
]
'

upsert_config_entry primary '
Kind      = "proxy-defaults"
Name      = "global"
Config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-one"
hostnames = ["test.consul.example"]
rules = [
  {
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-one"
  },
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-two"
hostnames = ["foo.bar.baz"]
rules = [
  {
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-two"
  },
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-three"
hostnames = ["foo.bar.baz"]
rules = [
  {
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-three"
  },
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-four"
rules = [
  {
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-four"
  },
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-five"
rules = [
  {
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-five"
  },
]
'

function docker_exec {
  if ! docker.exe exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul_exec {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}



function register_services {
  local DC=${1:-primary}
  wait_for_leader "$DC"
  docker_consul_exec ${DC} bash -c "consul services register workdir/${DC}/register/service_*.hcl"
}

register_services primary

function gen_envoy_bootstrap {
  SERVICE=$1
  ADMIN_PORT=$2
  DC=${3:-primary}
  IS_GW=${4:-0}
  EXTRA_ENVOY_BS_ARGS="${5-}"

  PROXY_ID="$SERVICE"
  if ! is_set "$IS_GW"; then
    PROXY_ID="$SERVICE-sidecar-proxy"
  fi

  if output=$(docker_consul_for_proxy_bootstrap "$DC" connect envoy -bootstrap \
    -proxy-id $PROXY_ID \
    -envoy-version "$ENVOY_VERSION" \
    -admin-bind 0.0.0.0:$ADMIN_PORT ${EXTRA_ENVOY_BS_ARGS} 2>&1); then

    # All OK, write config to file
    echo "$output" >workdir/${DC}/envoy/$SERVICE-bootstrap.json
  else
    status=$?
    # Command failed, instead of swallowing error (printed on stdout by docker
    # it seems) by writing it to file, echo it
    echo "$output"
    return $status
  fi
}

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
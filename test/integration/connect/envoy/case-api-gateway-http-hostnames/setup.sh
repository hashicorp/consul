#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

source helpers.bash

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

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
Defaults {
  MaxConnections        = 5
  MaxPendingRequests    = 3
  MaxConcurrentRequests = 4
}
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "http"
    hostname = "*.consul.example"
  }
]
'

upsert_config_entry primary '
Kind = "proxy-defaults"
Name = "global"
Config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-default"
hostnames = ["default.consul.example"]
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
  }
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-override"
hostnames = ["override.consul.example"]
rules = [
  {
    services = [
      {
        name = "s2"
        Limits {
          MaxConnections        = 2
          MaxPendingRequests    = 1
          MaxConcurrentRequests = 2
        }
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-one"
  }
]
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s1"
sources {
  name = "api-gateway"
  action = "allow"
}
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s2"
sources {
  name = "api-gateway"
  action = "allow"
}
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "tcp"
  },
  {
    name = "listener-two"
    port = 9998
    protocol = "tcp"
  }
]
'

upsert_config_entry primary '
kind = "tcp-route"
name = "api-gateway-route-one"
services = [
  {
    name = "s1"
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
kind = "tcp-route"
name = "api-gateway-route-two"
services = [
  {
    name = "s2"
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-two"
    kind = "api-gateway"
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
  action = "deny"
}
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

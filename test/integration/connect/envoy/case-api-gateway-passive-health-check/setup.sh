#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Two upstream services:
#   s1 — outlier detection comes from the gateway-level Defaults.PassiveHealthCheck
#   s2 — outlier detection comes from the route service-level Limits.PassiveHealthCheck
#        which overrides the gateway defaults

upsert_config_entry primary '
Kind      = "proxy-defaults"
Name      = "global"
Config {
  protocol = "http"
}
'

# Gateway with gateway-wide default passive health check applied to all routes.
upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name     = "listener-one"
    port     = 9999
    protocol = "http"
  }
]
defaults {
  passive_health_check {
    interval             = "10s"
    max_failures         = 3
    enforcing_consecutive_5xx = 80
    max_ejection_percent = 25
    base_ejection_time   = "30s"
  }
}
'

# Route for s1 — inherits gateway-level defaults, no service-level override.
upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-s1"
rules = [
  {
    matches = [{ path = { match = "prefix"  value = "/s1" } }]
    services = [{ name = "s1" }]
  }
]
parents = [
  {
    name        = "api-gateway"
    sectionName = "listener-one"
  }
]
'

# Route for s2 — service-level Limits.PassiveHealthCheck overrides the gateway defaults.
upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-s2"
rules = [
  {
    matches = [{ path = { match = "prefix"  value = "/s2" } }]
    services = [
      {
        name = "s2"
        limits {
          passive_health_check {
            interval             = "20s"
            max_failures         = 5
            enforcing_consecutive_5xx = 60
            max_ejection_percent = 50
            base_ejection_time   = "60s"
          }
        }
      }
    ]
  }
]
parents = [
  {
    name        = "api-gateway"
    sectionName = "listener-one"
  }
]
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s1"
sources {
  name   = "api-gateway"
  action = "allow"
}
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s2"
sources {
  name   = "api-gateway"
  action = "allow"
}
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Two upstream services:
#   s1 — "canary" backend (receives traffic when X-Canary header IS present)
#   s2 — "default" backend (receives traffic when X-Canary header is ABSENT)

upsert_config_entry primary '
Kind      = "proxy-defaults"
Name      = "global"
Config {
  protocol = "http"
}
'

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
'

# Route traffic using header-invert rules:
#
#   Rule 1 — X-Canary present  + invert = true  → match when X-Canary is ABSENT  → s2 (default)
#   Rule 2 — X-Canary present  + invert = false → match when X-Canary IS present → s1 (canary)
#
# Because rules are evaluated top-to-bottom, absent-header traffic hits rule 1
# and present-header traffic falls through to rule 2.
upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-invert"
rules = [
  {
    matches = [
      {
        headers = [
          {
            name   = "x-canary"
            match  = "present"
            invert = true
          }
        ]
      }
    ]
    services = [
      {
        name = "s2"
      }
    ]
  },
  {
    matches = [
      {
        headers = [
          {
            name  = "x-canary"
            match = "present"
          }
        ]
      }
    ]
    services = [
      {
        name = "s1"
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

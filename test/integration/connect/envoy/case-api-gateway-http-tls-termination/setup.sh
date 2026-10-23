#!/bin/bash
# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

# Zero-touch downstream TLS termination: the api-gateway opts into
# Connect-managed TLS via a gateway-level `TLS { Enabled = true }` block and
# attaches NO custom certificate to the listener. The gateway must therefore
# terminate downstream HTTPS using its auto-issued Connect leaf certificate,
# which carries the "*.api-gateway.<domain>" DNS SANs.
upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
tls {
  enabled = true
}
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "http"
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
kind = "service-intentions"
name = "s1"
sources {
  name = "api-gateway"
  action = "allow"
}
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000


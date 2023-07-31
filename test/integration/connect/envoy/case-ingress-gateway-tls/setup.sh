#!/bin/bash

set -euo pipefail

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
tls {
  enabled = true
}
listeners = [
  {
    port     = 9998
    protocol = "http"
    services = [
      {
        name = "s1"
      }
    ]
  },
  {
    port     = 9999
    protocol = "http"
    services = [
      {
        name  = "s1"
        hosts = ["test.example.com"]
      }
    ]
  }
]
'

register_services primary

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

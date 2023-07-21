#!/bin/bash

set -euo pipefail

upsert_config_entry primary '
kind     = "service-defaults"
name     = "s1"
protocol = "grpc"
'

upsert_config_entry primary '
kind = "ingress-gateway"
name = "ingress-gateway"
listeners = [
  {
    port     = 9999
    protocol = "grpc"
    services = [
      {
        name  = "s1"
        hosts = ["localhost:9999"]
      }
    ]
  }
]
'

register_services primary

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

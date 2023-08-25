#!/bin/bash

set -eEuo pipefail

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
listeners = [
  {
    protocol = "http"
    port     = 9999
    services = [
      {
        name = "peer-s2"
      }
    ]
  },
  {
    protocol = "http"
    port     = 10000
    services = [
      {
        name = "peer-s1"
      }
    ]
  },
  {
    protocol = "http"
    port     = 10001
    services = [
      {
        name = "s1"
      }
    ]
  },
  {
    protocol = "http"
    port     = 10002
    services = [
      {
        name = "split"
      }
    ]
  }
]
'

upsert_config_entry primary '
kind = "service-resolver"
name = "peer-s1"
redirect = {
  service = "s1"
  peer    = "primary-to-alpha"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "peer-s2"
redirect = {
  service = "s2"
  peer    = "primary-to-alpha"
}
'

upsert_config_entry primary '
kind = "service-splitter"
name = "split"
splits = [
  {
    Weight  = 50
    Service = "peer-s1"
  },
  {
    Weight  = 50
    Service = "peer-s2"
  },
]
'

register_services primary

gen_envoy_bootstrap ingress-gateway 20000 primary true
gen_envoy_bootstrap s1 19000 primary

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
kind = "service-splitter"
name = "split-s2"
splits = [
  {
    Weight  = 50
    Service = "local-s2"
    ResponseHeaders {
      Set {
        "x-test-split" = "primary"
      }
    }
  },
  {
    Weight  = 50
    Service = "peer-s2"
    ResponseHeaders {
      Set {
        "x-test-split" = "alpha"
      }
    }
  },
]
'

upsert_config_entry primary '
kind = "service-resolver"
name = "local-s2"
redirect = {
  service = "s2"
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

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

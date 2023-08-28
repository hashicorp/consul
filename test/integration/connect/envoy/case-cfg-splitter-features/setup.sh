#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
subsets = {
  "v1" = {
    filter = "Service.Meta.version == v1"
  }
  "v2" = {
    filter = "Service.Meta.version == v2"
  }
}
'

upsert_config_entry primary '
kind = "service-splitter"
name = "s2"
splits = [
  {
    weight         = 50,
    service_subset = "v2"
    request_headers {
      set {
        x-split-leg = "v2"
      }
      remove = ["x-bad-req"]
    }
    response_headers {
      add {
        x-svc-version = "v2"
      }
      remove = ["x-bad-resp"]
    }
  },
  {
    weight         = 50,
    service_subset = "v1"
    request_headers {
      set {
        x-split-leg = "v1"
      }
      remove = ["x-bad-req"]
    }
    response_headers {
      add {
        x-svc-version = "v1"
      }
      remove = ["x-bad-resp"]
    }
  },
]
'

register_services primary

# s2 is retained just to have a honeypot for bad envoy configs to route into
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2-v1 19001
gen_envoy_bootstrap s2-v2 19002
gen_envoy_bootstrap s2 19003

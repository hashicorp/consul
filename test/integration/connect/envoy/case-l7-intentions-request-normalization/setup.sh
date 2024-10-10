#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry primary '
kind     = "service-defaults"
name     = "s2"
protocol = "http"
'

upsert_config_entry primary '
kind = "mesh"
http {
  incoming {
    request_normalization {
      insecure_disable_path_normalization = false // explicitly set to the default for clarity
      merge_slashes                       = true
      path_with_escaped_slashes_action    = "UNESCAPE_AND_FORWARD"
      headers_with_underscores_action     = "REJECT_REQUEST"
    }
  }
}
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s2"
sources {
  name = "s1"
  permissions = [
    // paths
    {
      action = "deny"
      http {
        path_exact = "/value/supersecret"
      }
    },
    // headers
    {
      action = "deny"
      http {
        header = [{
          name  = "x-check"
          contains = "bad"
          ignore_case = true
        }]
      }
    },
    {
      action = "deny"
      http {
        header = [{
          name  = "x-check"
          exact = "exactbad"
          ignore_case = true
        }]
      }
    },
    {
      action = "deny"
      http {
        header = [{
          name   = "x-check"
          prefix = "prebad-"
          ignore_case = true
        }]
      }
    },
    {
      action = "deny"
      http {
        header = [{
          name   = "x-check"
          suffix = "-sufbad"
          ignore_case = true
        }]
      }
    },
    // redundant with above case, but included for real-world example
    // and to cover values containing ".".
    {
      action = "deny"
      http {
        header = [{
          name   = "Host"
          suffix = "bad.com"
          ignore_case = true
        }]
      }
    }
  ]
}
'

register_services primary

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

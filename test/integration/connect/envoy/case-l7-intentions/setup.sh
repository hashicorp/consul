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
kind = "service-intentions"
name = "s2"
sources {
  name = "s1"
  permissions = [
    // paths
    {
      action = "allow"
      http { path_exact = "/exact" }
    },
    {
      action = "allow"
      http { path_prefix = "/prefix" }
    },
    {
      action = "allow"
      http { path_regex = "/reg[ex]{2}" }
    },
    // headers
    {
      action = "allow"
      http {
        path_exact = "/hdr-present"
        header = [{
          name    = "x-test-debug"
          present = true
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-exact"
        header = [{
          name  = "x-test-debug"
          exact = "exact"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-exact-ignore-case"
        header = [{
          name  = "x-test-debug"
          exact = "foo.bar.com"
          ignore_case = true
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-prefix"
        header = [{
          name   = "x-test-debug"
          prefix = "prefi"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-prefix-ignore-case"
        header = [{
          name   = "x-test-debug"
          prefix = "foo.bar"
          ignore_case = true
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-suffix"
        header = [{
          name   = "x-test-debug"
          suffix = "uffix"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-suffix-ignore-case"
        header = [{
          name   = "x-test-debug"
          suffix = "bar.com"
          ignore_case = true
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-contains"
        header = [{
          name   = "x-test-debug"
          contains = "contains"
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-contains-ignore-case"
        header = [{
          name   = "x-test-debug"
          contains = "contains"
          ignore_case = true
        }]
      }
    },
    {
      action = "allow"
      http {
        path_exact = "/hdr-regex"
        header = [{
          name  = "x-test-debug"
          regex = "reg[ex]{2}"
        }]
      }
    },
    // methods
    {
      action = "allow"
      http {
        path_exact = "/method-match"
        methods    = ["GET", "PUT"]
      }
    }
  ]
}
'

register_services primary

gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

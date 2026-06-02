#!/bin/bash
# Copyright IBM Corp. 2024, 2026
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
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name = "https"
    port = 9999
    protocol = "http"
    tls {
      sds {
        cluster_name  = "sds-cluster"
        cert_resource = "wildcard.ingress.consul"
      }
    }
  }
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route"
hostnames = ["foo.example.com"]
rules = [
  {
    services = [
      {
        name = "s1"
        weight = 1
        tls {
          sds {
            cert_resource = "foo.example.com"
          }
        }
      },
      {
        name = "s2"
        weight = 1
        tls {
          sds {
            cert_resource = "foo.example.com"
          }
        }
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "https"
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

upsert_config_entry primary '
kind = "service-intentions"
name = "s2"
sources {
  name = "api-gateway"
  action = "allow"
}
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001


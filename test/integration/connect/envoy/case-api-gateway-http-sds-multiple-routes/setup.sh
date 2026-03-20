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
name = "api-gateway-route-one"
hostnames = ["foo.example.com"]
rules = [
  {
    services = [
      {
        name = "s1"
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
kind = "http-route"
name = "api-gateway-route-two"
hostnames = ["www.example.com"]
rules = [
  {
    services = [
      {
        name = "s2"
        tls {
          sds {
            cert_resource = "www.example.com"
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


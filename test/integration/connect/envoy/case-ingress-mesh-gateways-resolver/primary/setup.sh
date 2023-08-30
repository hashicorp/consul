#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

upsert_config_entry primary '
kind = "ingress-gateway"
name = "ingress-gateway"
listeners = [
  {
    protocol = "tcp"
    port     = 9999
    services = [
      {
        name = "s2"
      }
    ]
  },
  {
    protocol = "tcp"
    port     = 10000
    services = [
      {
        name = "s1"
      }
    ]
  }
]
'

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
mesh_gateway {
  mode = "local"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
redirect {
  service    = "s2"
  datacenter = "secondary"
}
'

upsert_config_entry primary '
kind = "service-defaults"
name = "s1"
mesh_gateway {
  mode = "remote"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s1"
redirect {
  service    = "s1"
  datacenter = "secondary"
}
'

register_services primary

gen_envoy_bootstrap mesh-gateway 19002 primary true
gen_envoy_bootstrap ingress-gateway 20000 primary true
retry_default docker_consul primary curl -s "http://localhost:8500/v1/catalog/service/consul?dc=secondary" >/dev/null

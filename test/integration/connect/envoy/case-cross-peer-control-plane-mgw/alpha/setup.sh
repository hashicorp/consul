#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -euo pipefail

upsert_config_entry alpha '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "tcp"
}
'

upsert_config_entry alpha '
kind = "mesh"
peering {
  peer_through_mesh_gateways = true
}
'

upsert_config_entry alpha '
kind = "exported-services"
name = "default"
services = [
  {
    name = "s2"
    consumers = [
      {
        peer = "alpha-to-primary"
      }
    ]
  }
]
'

register_services alpha

gen_envoy_bootstrap s2 19002 alpha
gen_envoy_bootstrap mesh-gateway 19003 alpha true

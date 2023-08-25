#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -eEuo pipefail

upsert_config_entry primary '
kind     = "service-defaults"
name     = "s2"
protocol = "http"
mesh_gateway {
  mode = "remote"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "s2"
failover = {
  "*" = {
    datacenters = ["secondary"]
  }
}
'

# also wait for replication to make it to the remote dc
wait_for_config_entry service-defaults s2 secondary
wait_for_config_entry service-resolver s2 secondary

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

node_name = "sec"
connect {
  enabled                            = true
  enable_mesh_gateway_wan_federation = true
}
primary_gateways = [
  "consul-primary-client:4431",
]
primary_gateways_interval = "5s"
retry_interval_wan        = "5s"

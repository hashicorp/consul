#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

upsert_config_entry alpha '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry alpha '
kind = "service-router"
name = "s2"
routes = [
  {
    match { http { path_prefix = "/s3/" } }
    destination {
      service        = "s3"
      prefix_rewrite = "/"
    }
  },
]
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
gen_envoy_bootstrap s3 19004 alpha

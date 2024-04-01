#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s2"
Protocol = "http"
EnvoyExtensions = [
  {
    Name = "builtin/wasm"
    Arguments = {
      Protocol = "http"
      ListenerType = "inbound"
      PluginConfig = {
        VmConfig = {
          Code  = {
            Local = {
              Filename = "/workdir/primary/data/dummy.wasm"
            }
          }
        }
        Configuration  = "plugin configuration"
      }
    }
  }
]
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Create proxy defaults
upsert_config_entry primary '
Kind = "proxy-defaults"
Name = "global"
Config {
  protocol = "http"
}
'

# Create service defaults for static-server
upsert_config_entry primary '
Kind = "service-defaults"
Name = "static-server"
Protocol = "http"
'

# Create API Gateway
upsert_config_entry primary '
Kind = "api-gateway"
Name = "api-gateway"
Listeners = [
  {
    Name = "listener-1"
    Port = 9999
    Protocol = "http"
  }
]
'

# Create HTTP route
upsert_config_entry primary '
Kind = "http-route"
Name = "http-route"
Rules = [
  {
    Matches = [
      {
        Path = {
          Match = "prefix"
          Value = "/"
        }
      }
    ]
    Services = [
      {
        Name = "static-server"
        Weight = 100
      }
    ]
  }
]
'

# Create service defaults for API Gateway with LUA extension
upsert_config_entry primary '
Kind = "service-defaults"
Name = "api-gateway"
Protocol = "http"
EnvoyExtensions = [
  {
    Name = "builtin/lua"
    Arguments = {
      ProxyType = "api-gateway"
      Listener = "outbound"
      Script = "function envoy_on_request(request_handle) request_handle:headers():add(\"x-lua-added\", \"test-value\") end"
    }
  }
]
'

# Register services
register_services primary

# Generate bootstrap configs
gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap static-server 19000 
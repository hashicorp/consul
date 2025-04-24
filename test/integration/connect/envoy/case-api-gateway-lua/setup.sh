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
    Name = "listener"
    Port = 8080
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
    Services = [
      {
        Name = "static-server"
      }
    ]
  }
]
'


# Create service defaults for API Gateway with LUA extension
upsert_config_entry primary '
Kind = "service-defaults"
Name = "api-gateway"
EnvoyExtensions = [
  {
    Name = "builtin/lua"
    Arguments = {
      ProxyType = "api-gateway"
      Listener = "outbound"
      Script = "function envoy_on_response(response_handle) response_handle:headers():add('x-test', 'test') end"
    }
  }
]
'


# Register services
register_service primary static-server "{ \"service\": { \"name\": \"static-server\", \"port\": 8080 } }"
register_service primary api-gateway "{ \"service\": { \"name\": \"api-gateway\", \"kind\": \"api-gateway\", \"port\": 8080 } }"

# Generate bootstrap configs
gen_envoy_bootstrap api-gateway primary api-gateway-sidecar-proxy api-gateway-bootstrap.json
gen_envoy_bootstrap static-server primary static-server-sidecar-proxy static-server-bootstrap.json
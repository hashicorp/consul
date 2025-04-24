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

# Create service defaults for s1 and s2
upsert_config_entry primary '
Kind = "service-defaults"
Name = "s1"
Protocol = "http"
'

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s2"
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
kind = "http-route"
name = "api-gateway-route-one"
rules = [
  {
    services = [
      {
        name = "s1"
      },
      {
        name = "s2"
        weight = 2
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-one"
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
      Listener = "inbound"
      Script = "function envoy_on_request(request_handle) request_handle:headers():add(\"x-lua-added\", \"test-value\") end"
    }
  }
]
'

# Create service intentions
upsert_config_entry primary '
Kind = "service-intentions"
Name = "s1"
Sources = [
  {
    Name = "api-gateway"
    Action = "allow"
  }
]
'

upsert_config_entry primary '
Kind = "service-intentions"
Name = "s2"
Sources = [
  {
    Name = "api-gateway"
    Action = "allow"
  }
]
'

# Register services
register_services primary

# Generate bootstrap configs
gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001

# Debug: Check if Envoy is running
echo "Checking if Envoy is running..."
docker ps | grep envoy
echo "Checking if port 20000 is listening..."
docker exec $(docker ps -q --filter name=envoy) netstat -tulpn | grep 20000 || true 
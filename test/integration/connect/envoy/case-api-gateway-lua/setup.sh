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
# Create API Gateway
upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "http"
  }
]
'

# Create HTTP route
upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-one"
rules = [
  {
    matches = [
      {
        path = {
          match = "prefix"
          value = "/echo"
        }
      }
    ]
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    kind = "api-gateway"
    name = "api-gateway"
  }
]
'

# Create service defaults for API Gateway with LUA extension
upsert_config_entry primary "$(cat <<'EOF'
Kind = "service-defaults"
Name = "api-gateway"
Protocol = "http"
EnvoyExtensions = [
  {
    name  = "builtin/lua"
    required = true
    arguments = {
      proxyType = "api-gateway"
      listener = "outbound"
      script = <<EOT
        function envoy_on_request(request_handle)
          request_handle:headers():add("x-lua-added-onrequest", "header-added-on-request")
        end
      EOT
    }
  },
  {
    name     = "builtin/lua"
    required = true
    arguments = {
      proxyType = "api-gateway"
      listener  = "outbound"
      script    = <<EOT
        function envoy_on_response(response_handle)
          response_handle:headers():add("x-lua-added-onresponse", "header-added-on-response")
          if response_handle:headers():get(":status") == "404" then
              response_handle:headers():replace(":status", "200")
              local json = '{"message":"body modified by Lua script","status":"success"}'
              response_handle:body():setBytes(json)


              response_handle:headers():remove("x-envoy-upstream-service-time")
              response_handle:headers():remove("x-powered-by")
              response_handle:headers():replace("cache-control", "no-store")
              response_handle:headers():remove("content-length")
              response_handle:headers():replace("content-encoding", "identity")
              response_handle:headers():replace("content-type", "application/json")
          end
        end
      EOT
    }
  }
]
EOF
)"

upsert_config_entry primary "$(cat <<'EOF'
Kind = "service-defaults"
Name = "s1"
Protocol = "http"
EnvoyExtensions = [
  {
    Name = "builtin/lua"
    Arguments = {
      ProxyType = "connect-proxy"
      Listener = "inbound"
      Script = "function envoy_on_request(request_handle) request_handle:headers():add(\"x-lua-added\", \"test-value\") end"
    }
  },
  {
    name     = "builtin/lua"
    required = true
    arguments = {
      proxyType = "connect-proxy"
      listener  = "outbound"
      script    = <<EOT
        function envoy_on_response(response_handle)
          if response_handle:headers():get(":status") == "404" then
              response_handle:headers():replace(":status", "200")
              local json = '{"message":"body modified by Lua script","status":"success"}'
              response_handle:body():setBytes(json)

              response_handle:headers():remove("x-envoy-upstream-service-time")
              response_handle:headers():remove("x-powered-by")
              response_handle:headers():replace("cache-control", "no-store")
              response_handle:headers():remove("content-length")
              response_handle:headers():replace("content-encoding", "identity")
              response_handle:headers():replace("content-type", "application/json")
          end
        end
      EOT
    }
  }
]
EOF
)"

# Create service intentions
upsert_config_entry primary '
Kind = "service-intentions"
Name = "*"
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
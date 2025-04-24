#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -euo pipefail

# Create proxy defaults
cat > proxy-defaults.hcl <<EOF
Kind = "proxy-defaults"
Name = "global"
Config {
  protocol = "http"
}
EOF

consul config write proxy-defaults.hcl

# Create service defaults for static-server
cat > service-defaults.hcl <<EOF
Kind = "service-defaults"
Name = "static-server"
Protocol = "http"
EOF

consul config write service-defaults.hcl

# Create API Gateway
cat > api-gateway.hcl <<EOF
Kind = "api-gateway"
Name = "api-gateway"
Listeners = [
  {
    Name = "listener"
    Port = 8080
    Protocol = "http"
  }
]
EOF

consul config write api-gateway.hcl

# Create HTTP route
cat > http-route.hcl <<EOF
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
EOF

consul config write http-route.hcl

# Create service defaults for API Gateway with LUA extension
cat > api-gateway-service-defaults.hcl <<EOF
Kind = "service-defaults"
Name = "api-gateway"
EnvoyExtensions = [
  {
    Name = "builtin/lua"
    Arguments = {
      Script = "function envoy_on_response(response_handle) response_handle:headers():add('x-test', 'test') end"
    }
  }
]
EOF

consul config write api-gateway-service-defaults.hcl

# Register services
consul services register static-server.hcl
consul services register api-gateway.hcl

# Generate bootstrap configs
consul connect envoy -bootstrap \
  -proxy-id api-gateway-sidecar-proxy \
  > api-gateway-bootstrap.json

consul connect envoy -bootstrap \
  -proxy-id static-server-sidecar-proxy \
  > static-server-bootstrap.json 
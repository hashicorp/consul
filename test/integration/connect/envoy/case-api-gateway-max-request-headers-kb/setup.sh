#!/bin/bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

set -eEuo pipefail

# Configure proxy-defaults with max_request_headers_kb setting
upsert_config_entry primary '
Kind = "proxy-defaults"
Name = "global"
Config {
  protocol                = "http"
  max_request_headers_kb  = 96
}
'

# Configure API Gateway
upsert_config_entry primary '
Kind = "api-gateway"
Name = "api-gateway"

Listeners = [
  {
    Name                = "http-listener"
    Port                = 9999
    Protocol            = "http"
    MaxRequestHeadersKB = 96
  }
]
'

# Configure HTTP route to connect gateway to backend service
upsert_config_entry primary '
Kind = "http-route"
Name = "my-gateway-route"

Parents = [
  {
    Kind        = "api-gateway"
    Name        = "api-gateway"
    SectionName = "http-listener"
  }
]

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
        Name = "s1"
      }
    ]
  }
]
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s1"
sources {
  name = "api-gateway"
  action = "allow"
}
'

# Register services
register_services primary

# Generate Envoy bootstrap configs
gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
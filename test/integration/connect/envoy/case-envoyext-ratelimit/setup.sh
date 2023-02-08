#!/bin/bash

set -eEuo pipefail

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s2"
Protocol = "http"
EnvoyExtensions = [
  {
    Name = "builtin/http/localratelimit",
    Arguments = {
      ProxyType = "connect-proxy"
      MaxTokens = 1,
      TokensPerFill = 1,
      FillInterval = 120,
      FilterEnabled = 100,
      FilterEnforced = 100,
    }
  }
]
'

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s1"
Protocol = "tcp"
EnvoyExtensions = [
  {
    Name = "builtin/http/localratelimit",
    Arguments = {
      ProxyType = "connect-proxy"
      MaxTokens = 1,
      TokensPerFill = 1,
      FillInterval = 120,
      FilterEnabled = 100,
      FilterEnforced = 100,
    }
  }
]
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

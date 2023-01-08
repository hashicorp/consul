#!/bin/bash

set -eEuo pipefail

upsert_config_entry primary '
Kind = "service-defaults"
Name = "s2"
Protocol = "http"
EnvoyExtensions = [
  {
    Name = "builtin/ratelimit",
    Arguments = {
      ProxyType = "connect-proxy"
      Listener = "inbound"
      maxTokens = 1,
      tokensPerFill = 1,
      fillInterval = 120,
    }
  }
]
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary

#!/bin/bash

set -euo pipefail

# manually setup config entries
echo '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
' | docker_consul config write -

echo '
kind = "service-defaults"
name = "s1"
protocol = "http"
' | docker_consul config write -
echo '
kind = "service-defaults"
name = "s2"
protocol = "http"
' | docker_consul config write -
echo '
kind = "service-defaults"
name = "s3"
protocol = "http"
' | docker_consul config write -

echo '
kind = "service-resolver"
name = "s2"
redirect {
  service = "s3"
}
' | docker_consul config write -

# retry because resolving the central config might race
retry_default gen_envoy_bootstrap s1 19000
retry_default gen_envoy_bootstrap s2 19001
retry_default gen_envoy_bootstrap s3 19002

export REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 s2-sidecar-proxy s3 s3-sidecar-proxy"

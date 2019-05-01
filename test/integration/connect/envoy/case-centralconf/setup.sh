#!/bin/bash

set -euo pipefail

# retry because resolving the central config might race
retry_default gen_envoy_bootstrap s1 19000
retry_default gen_envoy_bootstrap s2 19001

export REQUIRED_SERVICES="s1 s1-sidecar-proxy s2 s2-sidecar-proxy"
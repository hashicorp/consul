#!/bin/bash

set -eEuo pipefail

gen_envoy_bootstrap mesh-gateway 19001 secondary true
retry_default docker_consul secondary curl -s  "http://localhost:8500/v1/catalog/service/consul?dc=primary" >/dev/null

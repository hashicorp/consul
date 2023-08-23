#!/bin/bash

set -eEuo pipefail

register_services secondary

gen_envoy_bootstrap s2 19001 secondary
gen_envoy_bootstrap mesh-gateway 19003 secondary true
retry_default docker_consul secondary curl -s  "http://localhost:8500/v1/catalog/service/consul?dc=primary" >/dev/null

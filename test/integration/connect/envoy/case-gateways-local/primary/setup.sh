#!/bin/bash

set -eEuo pipefail

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap mesh-gateway 19002 primary true
retry_default docker_consul primary curl -s "http://localhost:8500/v1/catalog/service/consul?dc=secondary" >/dev/null

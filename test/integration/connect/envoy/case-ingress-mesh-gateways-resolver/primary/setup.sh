#!/bin/bash

set -eEuo pipefail

# wait for bootstrap to apply config entries
wait_for_config_entry ingress-gateway ingress-gateway
wait_for_config_entry proxy-defaults global

register_services primary

gen_envoy_bootstrap mesh-gateway 19002 primary true
gen_envoy_bootstrap ingress-gateway 20000 primary true
retry_default docker_consul primary curl -s "http://localhost:8500/v1/catalog/service/consul?dc=secondary" >/dev/null


#!/bin/bash

set -eEuo pipefail

register_services primary

gen_envoy_bootstrap s1 19000 primary
retry_default docker_consul primary curl -s "http://localhost:8500/v1/catalog/service/consul?dc=secondary" >/dev/null

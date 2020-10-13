#!/bin/bash

set -eEuo pipefail

# Setup deny intention
docker_consul primary intention create -deny s1 s2

#gen_envoy_bootstrap s1 19000 primary
#gen_envoy_bootstrap s2 19001 primary

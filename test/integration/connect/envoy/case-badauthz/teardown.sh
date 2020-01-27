#!/bin/bash

set -eEuo pipefail

# Remove deny intention
docker_consul primary intention delete s1 s2

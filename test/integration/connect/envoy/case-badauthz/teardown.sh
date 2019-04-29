#!/bin/bash

set -euo pipefail

# Remove deny intention
docker_consul intention delete s1 s2

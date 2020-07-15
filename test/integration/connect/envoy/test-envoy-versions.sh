#!/bin/bash

set -euo pipefail

unset CDPATH

cd "$(dirname "$0")"

# MISSING: 1.14.0
# MISSING: 1.12.5
versions=(
    1.14.3
    1.14.2
    1.14.1
    1.13.3
    1.13.2
    1.13.1
    1.13.0
    1.12.4
    1.12.3
    1.12.2
    1.12.1
    1.12.0
    1.11.2
    1.11.1
    1.11.0
    1.10.0
)

for v in "${versions[@]}"; do
    echo "ENVOY_VERSION=${v}"
    export ENVOY_VERSION="${v}"
    go test -tags integration
done

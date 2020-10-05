#!/bin/bash

set -euo pipefail

unset CDPATH

cd "$(dirname "$0")"

versions=(
    1.15.0
    1.14.4
    1.13.4
    1.12.6
)

for v in "${versions[@]}"; do
    echo "ENVOY_VERSION=${v}"
    export ENVOY_VERSION="${v}"
    go test -tags integration "$@"
done

#!/bin/bash

set -euo pipefail

unset CDPATH

cd "$(dirname "$0")"

## no rbac url_path support
    # 1.13.0

## does not exist in docker
    # 1.13.5
    # 1.14.0

## doesn't fully support the renamed filters
    # 1.13.1
    # 1.13.2
    # 1.13.3
    # 1.13.4
    # 1.13.6
    # 1.13.7
versions=(
    1.14.1
    1.14.2
    1.14.3
    1.14.4
    1.14.5
    1.14.6
    1.15.0
    1.15.1
    1.15.2
    1.15.3
    1.16.0
    1.16.1
    1.16.2
)

for v in "${versions[@]}"; do
    echo "ENVOY_VERSION=${v}"
    export ENVOY_VERSION="${v}"
    go test -tags integration "$@"
done

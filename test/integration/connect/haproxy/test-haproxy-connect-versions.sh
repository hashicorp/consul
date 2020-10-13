#!/bin/bash

set -euo pipefail

unset CDPATH

cd "$(dirname "$0")"

versions=(
    0.1.9
    0.9.0
)

for v in "${versions[@]}"; do
    echo "HAPROXY_CONSUL_CONNECT_VERSION=${v}"
    export HAPROXY_CONSUL_CONNECT_VERSION="${v}"
    go test -tags integration "$@"
done

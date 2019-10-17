#!/bin/bash

unset CDPATH

set -euo pipefail

cd "$(dirname "$0")"

if [[ $# -ne 1 ]]; then
    echo "usage: $0 <sdk|api>" >&2
    exit 1
fi
readonly mode="$1"

common_init() {
    rm -rf staging
    mkdir -p staging
}

stage_sdk() {
    cp -a sdk staging/consul-sdk

    # start from whatever versions work for the agent/cli
    cp go.{mod,sum} staging/consul-sdk

    (
    cd staging/consul-sdk

    go mod edit -module github.com/hashicorp/consul-sdk
    go mod edit -dropreplace github.com/hashicorp/consul/sdk
    go mod edit -dropreplace github.com/hashicorp/consul/api
    go mod edit -droprequire github.com/hashicorp/consul/sdk

    find . -name '*.go' -print0 | xargs -0 sed -i 's@"github.com/hashicorp/consul/sdk@"github.com/hashicorp/consul-sdk@g'

    go mod tidy
    go fmt ./...
    )

    echo "sdk: Differences between main repo and staged output:"
    echo "sdk: >>>>>>>>>>>>>>>>>>>>>>>>"
    diff -r ./sdk/ ./staging/consul-sdk/ || true
    echo "sdk: >>>>>>>>>>>>>>>>>>>>>>>>"

    echo "sdk: Now compare the diff against the last published version and decide if it needs to be pushed and tagged."
}

stage_api() {
    cp -a api staging/consul-api

    # start from whatever versions work for the agent/cli
    cp go.{mod,sum} staging/consul-api

    (
    cd staging/consul-api

    go mod edit -module github.com/hashicorp/consul-api
    go mod edit -dropreplace github.com/hashicorp/consul/sdk
    go mod edit -dropreplace github.com/hashicorp/consul/api
    go mod edit -droprequire github.com/hashicorp/consul/sdk
    go mod edit -droprequire github.com/hashicorp/consul/api

    # TODO: this should inherit from the latest published version
    go mod edit -require github.com/hashicorp/consul-sdk@v0.0.1

    if [[ "$mode" = "both" ]]; then
        go mod edit -replace github.com/hashicorp/consul-sdk=../consul-sdk
    fi

    find . -name '*.go' -print0 | xargs -0 sed -i 's@"github.com/hashicorp/consul/sdk@"github.com/hashicorp/consul-sdk@g'
    find . -name '*.go' -print0 | xargs -0 sed -i 's@"github.com/hashicorp/consul/api@"github.com/hashicorp/consul-api@g'

    go mod tidy
    go fmt ./...
    )

    echo "api: Differences between main repo and staged output:"
    echo "api: >>>>>>>>>>>>>>>>>>>>>>>>"
    diff -r ./api/ ./staging/consul-api/ || true
    echo "api: >>>>>>>>>>>>>>>>>>>>>>>>"

    echo "api: Now compare the diff against the last published version and decide if it needs to be pushed and tagged."
}

case "$mode" in
    sdk)
        common_init
        stage_sdk
        ;;
    api)
        common_init
        stage_api
        ;;
    both)
        common_init
        # this is a hack
        stage_sdk
        stage_api
        ;;
    *)
        echo "usage: $0 <sdk|api>" >&2
        exit 1
        ;;
esac

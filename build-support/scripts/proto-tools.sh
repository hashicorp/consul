#!/usr/bin/env bash

unset CDPATH

set -euo pipefail

readonly PROTOC_GEN_GO_VERSION="$(grep github.com/golang/protobuf go.mod | awk '{print $2}')"
readonly MOG_VERSION='v0.2.0'
readonly PROTOC_GO_INJECT_TAG_VERSION='v1.3.0'

function main {
    install_unversioned_tool goversion 'rsc.io/goversion@latest'

    install_versioned_tool \
        'protoc-gen-go' \
        'github.com/golang/protobuf' \
        "${PROTOC_GEN_GO_VERSION}" \
        'github.com/golang/protobuf/protoc-gen-go'

    install_unversioned_tool \
        protoc-gen-go-binary \
        'github.com/hashicorp/protoc-gen-go-binary@master'

    install_versioned_tool \
        'protoc-go-inject-tag' \
        'github.com/favadi/protoc-go-inject-tag' \
        "${PROTOC_GO_INJECT_TAG_VERSION}" \
        'github.com/favadi/protoc-go-inject-tag'

    install_versioned_tool \
        'mog' \
        'github.com/hashicorp/mog' \
        "${MOG_VERSION}" \
        'github.com/hashicorp/mog'

    return 0
}

function install_unversioned_tool {
    local command="$1"
    local install="$2"

    if ! command -v "${command}" &>/dev/null ; then
        echo "=== TOOL: ${install}"
        go install "${install}"
    fi

    return 0
}

function install_versioned_tool {
    local command="$1"
    local module="$2"
    local version="$3"
    local installbase="$4"

    local should_install=
    local got

    local expect="${module}@${version}"
    local install="${installbase}@${version}"

    if [[ -z "$version" ]]; then
        echo "cannot install '${command}' no version selected" >&2
        return 1
    fi

    if command -v "${command}" &>/dev/null ; then
        got="$(goversion -m $(which "${command}") | grep '\bmod\b' | grep "${module}" |
            awk '{print $2 "@" $3}')"
        if [[ "$expect" != "$got" ]]; then
            should_install=1
        fi
    else
        should_install=1
    fi

    if [[ -n $should_install ]]; then
        echo "=== TOOL: ${install}"
        go install "${install}"
    fi
    return 0
}

main "$@"
exit $?

#!/usr/bin/env bash

SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
SCRIPT_DIR=$(pwd)
pushd ../.. > /dev/null
SOURCE_DIR=$(pwd)
popd > /dev/null
pushd ../functions > /dev/null
FN_DIR=$(pwd)
popd > /dev/null
popd > /dev/null

source "${SCRIPT_DIR}/functions.sh"

unset CDPATH

set -euo pipefail

die() {
    echo "$1" >&2
    exit 1
}

usage() {
cat <<-EOF
Usage: ${SCRIPT_NAME} [<options ...>] <proto filepath>

Description:
    Generate all Go files from protobuf definitions. In addition to
    running the protoc generator it will also fixup build tags in the
    generated code and regenerate mog outputs and RPC stubs.

Options:
    --protoc-bin             Path to protoc.
    -h | --help              Print this help text.
EOF
}

err_usage() {
    err "$1"
    err ""
    err "$(usage)"
}

main() {
    local protoc_bin=

    while test $# -gt 0
    do
        case "$1" in
            -h | --help )
                usage
                return 0
                ;;
            --protoc-bin )
                protoc_bin="$2"
                shift 2
                ;;
        esac
    done

    if test -z "${protoc_bin}"
    then
        protoc_bin="$(command -v protoc)"
        if test -z "${protoc_bin}"
        then
            err_usage "ERROR: no proto-bin specified and protoc could not be discovered"
            return 1
        fi
    fi

    declare -a proto_files
    while IFS= read -r pkg; do
        pkg="${pkg#"./"}"
        proto_files+=( "$pkg" )
    done < <(find . -name '*.proto' | grep -v 'vendor/' | grep -v '.protobuf' | sort )

    for proto_file in "${proto_files[@]}"; do
        ${SCRIPT_DIR}/proto-gen.sh --grpc --protoc-bin "${protoc_bin}" "$proto_file"
    done

	echo "Generated all protobuf Go files"

    return 0
}

main "$@"
exit $?

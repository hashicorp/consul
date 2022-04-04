#!/usr/bin/env bash

readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"

unset CDPATH

set -euo pipefail

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
    local protoc_version=

    while test $# -gt 0
    do
        case "$1" in
            -h | --help )
                usage
                return 0
                ;;
            --protoc-version )
                protoc_version="$2"
                shift 2
                ;;
        esac
    done

    if test -z "${protoc_version}"
    then
        protoc_version="$(make --no-print-directory print-PROTOC_VERSION)"
        if test -z "${protoc_version}"
        then
            err_usage "ERROR: no proto-version specified and version could not be discovered"
            return 1
        fi
    fi

    # ensure the correct protoc compiler is installed
    protoc_install "${protoc_version}"
    if test -z "${protoc_bin}" ; then
        exit 1
    fi

    # ensure these tools are installed
    proto_tools_install
    # ${SCRIPT_DIR}/proto-tools.sh

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

#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


readonly SCRIPT_NAME="$(basename ${BASH_SOURCE[0]})"
readonly SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
readonly SOURCE_DIR="$(dirname "$(dirname "${SCRIPT_DIR}")")"
readonly FN_DIR="$(dirname "${SCRIPT_DIR}")/functions"

source "${SCRIPT_DIR}/functions.sh"

unset CDPATH

set -euo pipefail

usage() {
cat <<-EOF
Usage: ${SCRIPT_NAME} [<options ...>]

Description:
    Regenerates regenerates all Go files from protobuf definitions. In addition
    to running the protoc generator it will also fixup build tags in the
    generated code and regenerate mog outputs and RPC stubs.

Options:
    -h | --help              Print this help text.
EOF
}

function err_usage {
    err "$1"
    err ""
    err "$(usage)"
}

function main {
    while test $# -gt 0
    do
        case "$1" in
            -h | --help )
                usage
                return 0
                ;;
        esac
    done

    # clear old ratelimit.tmp files
    find . -name .ratelimit.tmp -delete

    local mods=$(find . -name 'buf.gen.yaml' -exec dirname {} \; | sort)
    for mod in $mods
    do
        status_stage "Generating protobuf module: $mod"
        (
            cd $mod
            buf generate
            for proto_file in $(buf ls-files)
            do
                postprocess_protobuf_code $proto_file
            done
        )
    done

    status "Generated all protobuf Go files"

    generate_mog_code

    status "Generated all mog Go files"

    generate_rate_limit_mappings $mods

    status "Generated gRPC rate limit mapping file"

    return 0
}

function postprocess_protobuf_code {
    local proto_path="${1:-}"
    if [[ -z "${proto_path}" ]]; then
        err "missing protobuf path argument"
        return 1
    fi

    local proto_go_path="${proto_path%%.proto}.pb.go"
    local proto_go_grpc_path="${proto_path%%.proto}_grpc.pb.go"
    local proto_go_bin_path="${proto_path%%.proto}.pb.binary.go"
    local proto_go_rpcglue_path="${proto_path%%.proto}.rpcglue.pb.go"

    status_stage "Post-Processing generated files for ${proto_path}"

    print_run protoc-go-inject-tag -input="${proto_go_path}" || {
        err "Failed to run protoc-go-inject-tag for ${proto_path}"
        return 1
    }

    local build_tags
    build_tags="$(head -n 2 "${proto_path}" | grep '^//go:build\|// +build' || true)"
    if test -n "${build_tags}"; then
       for file in "${proto_go_path}" "${proto_go_bin_path}" "${proto_go_grpc_path}"
       do
            if test -f "${file}" -a "$(head -n 2 ${file})" != "${build_tags}"
            then
                echo "Adding build tags to ${file}"
                echo -e "${build_tags}\n" >> "${file}.new"
                cat "${file}" >> "${file}.new"
                mv "${file}.new" "${file}"
            fi
        done
    fi

    # NOTE: this has to run after we fix up the build tags above
    rm -f "${proto_go_rpcglue_path}"
    print_run go run ${SOURCE_DIR}/internal/tools/proto-gen-rpc-glue/main.go -path "${proto_go_path}" || {
        err "Failed to generate consul rpc glue outputs from ${proto_path}"
        return 1
    }

    return 0
}

function generate_mog_code {
    local mog_order

    mog_order="$(go list -tags "${GOTAGS}" -deps ./proto/private/pb... | grep "consul/proto/private")"

    for FULL_PKG in ${mog_order}; do
        PKG="${FULL_PKG/#github.com\/hashicorp\/consul\/}"
        status_stage "Generating ${PKG}/*.pb.go into ${PKG}/*.gen.go with mog"
        find "$PKG" -name '*.gen.go' -delete
        if [[ -n "${GOTAGS}" ]]; then
            print_run mog -tags "${GOTAGS}" -source "./${PKG}/*.pb.go"
        else
            print_run mog -source "./${PKG}/*.pb.go"
        fi
    done

    return 0
}

function generate_rate_limit_mappings {
    local flags=(
      "-output ${SOURCE_DIR}/agent/grpc-middleware/rate_limit_mappings.gen.go"
    )
    for path in $@; do
      flags+=("-input $path")
    done

    print_run go run ${SOURCE_DIR}/internal/tools/protoc-gen-consul-rate-limit/postprocess/main.go ${flags[@]} || {
        err "Failed to generate gRPC rate limit mappings"
        return 1
    }
}

main "$@"
exit $?

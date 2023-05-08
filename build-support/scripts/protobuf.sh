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

    for mod in $(find . -name 'buf.gen.yaml' -exec dirname {} \; | sort)
    do
        (
            # This looks special and it is. First of all this is not just `buf generate`
            # from within the $mod directory because doing that would have caused global
            # file registration conflicts when Consul starts. TLDR there is that Go's
            # protobuf code tracks protobufs by their file paths so those filepaths all
            # must be unique.
            #
            # To work around those constraints we are trying to get the file descriptors
            # passed off to protoc-gen-go to include the top level path. The file paths
            # in the file descriptors will be relative to where `buf` is run. Therefore
            # we must run `buf` from the root of the repo but still tell it to only
            # generate the singular directory. The --template argument allows us to
            # point buf a particular configuration for what code to generate. The 
            # --path argument allows us to tell `buf` which files/directories to 
            # operate on. Hopefully in the future `buf` will be able to add prefixes
            # to file descriptor paths and we can modify this to work in a more natural way.
            buf generate --template ${mod}/buf.gen.yaml --path ${mod}
            cd $mod
            for proto_file in $(buf ls-files)
            do
                postprocess_protobuf_code $proto_file
            done
        )
    done

    status "Generated all protobuf Go files"

    generate_mog_code

    status "Generated all mog Go files"

    return 0
}

function postprocess_protobuf_code {
    local proto_path="${1:-}"
    if [[ -z "${proto_path}" ]]; then
        err "missing protobuf path argument"
        return 1
    fi

    local proto_go_path="${proto_path%%.proto}.pb.go"
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
       for file in "${proto_go_bin_path}" "${proto_go_grpc_path}"
       do
            if test -f "${file}"
            then
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

    mog_order="$(go list -tags "${GOTAGS}" -deps ./proto/pb... | grep "consul/proto")"

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

main "$@"
exit $?

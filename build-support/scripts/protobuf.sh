#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


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

			# Check that the resources in a given module have valid versions.
			# TODO: should we only do this for ./proto-public?
			validate_resource_version

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

    generate_protoset_file

    status "Generated protoset file"

    return 0
}

# List all of the files in the current buf module and check for a valid version string.
# A version string is treated as the second element in a directory path. E.g. /path/[VERSION]/file.proto
function validate_resource_version {
	for FILE in $(buf ls-files)
    do
		# Split the path by / and return the second element.
		VERSION=$(echo "$FILE" | cut -d "/" -f 2)

		# If the version is empty or ends in .proto, skip.
		# E.g. /some_path/file.proto
		if [ "$VERSION" == "" ] || [ "${VERSION##*.}" = "proto" ]; then
			continue
		fi

		# Return an error if the version string does not match the regex.
		if ! [[ "$VERSION" =~ v[0-9]+((alpha|beta)[1-9])? ]]; then
			err "Invalid version string \"$VERSION\" in module $FILE"
			return 1
		fi
	done
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

function generate_protoset_file {
  local pkg_dir="${SOURCE_DIR}/pkg"
  mkdir -p "$pkg_dir"
  print_run buf build -o "${pkg_dir}/consul.protoset"
}

main "$@"
exit $?

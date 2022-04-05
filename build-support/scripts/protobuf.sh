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
    Installs protoc, various supporting Go tools, and then regenerates all Go
    files from protobuf definitions. In addition to running the protoc
    generator it will also fixup build tags in the generated code and
    regenerate mog outputs and RPC stubs.

Options:
    --protoc-version         Version of protoc to install. It defaults to what is specified in the makefile.
    --tools-only             Install all required tools but do not generate outputs.
    -h | --help              Print this help text.
EOF
}

function err_usage {
    err "$1"
    err ""
    err "$(usage)"
}

function main {
    local protoc_version=
    local tools_only=

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
            --tools-only )
                tools_only=1
                shift
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

    if [[ -n $tools_only ]]; then
        return 0
    fi

    # Compute some data from dependencies in non-local variables.
    go mod download
    golang_proto_path="$(go list -f '{{ .Dir }}' -m github.com/golang/protobuf)"
    # golang_proto_mod_path="$(sed -e 's,\(.*\)github.com.*,\1,' <<< "${golang_proto_path}")"
    golang_proto_mod_path="$(go env GOMODCACHE)"

    declare -a proto_files
    while IFS= read -r pkg; do
        pkg="${pkg#"./"}"
        proto_files+=( "$pkg" )
    done < <(find . -name '*.proto' | grep -v 'vendor/' | grep -v '.protobuf' | sort )

    for proto_file in "${proto_files[@]}"; do
        generate_protobuf_code "${proto_file}"
    done

    status "Generated all protobuf Go files"

    generate_mog_code

    status "Generated all mog Go files"

    return 0
}

# Installs the version of protoc specified by the first argument.
#
# Will set 'protoc_bin'
function protoc_install {
    local protoc_version="${1:-}"
    local protoc_os

    if test -z "${protoc_version}"
    then
        protoc_version="$(make --no-print-directory print-PROTOC_VERSION)"
        if test -z "${protoc_version}"
        then
            err "ERROR: no protoc-version specified and version could not be discovered"
            return 1
        fi
    fi

    case "$(uname)" in
        Darwin)
            protoc_os="osx"
            ;;
        Linux)
            protoc_os="linux"
            ;;
        *)
            err "unexpected OS: $(uname)"
            return 1
    esac

    local protoc_zip="protoc-${protoc_version}-${protoc_os}-x86_64.zip"
    local protoc_url="https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/${protoc_zip}"
    local protoc_root=".protobuf/protoc-${protoc_os}-${protoc_version}"
    # This is updated for use outside of the function.
    protoc_bin="${protoc_root}/bin/protoc"

    if [[ -x "${protoc_bin}" ]]; then
        status "protocol buffer compiler version already installed: ${protoc_version}"
        return 0
    fi

    status_stage "installing protocol buffer compiler version: ${protoc_version}"

    mkdir -p .protobuf/tmp
    if [[ ! -f .protobuf/tmp/${protoc_zip} ]]; then \
        ( cd .protobuf/tmp && curl -sSL "${protoc_url}" -o "${protoc_zip}" )
    fi

    mkdir -p "${protoc_root}"
    unzip -d "${protoc_root}" ".protobuf/tmp/${protoc_zip}"
    chmod -R a+Xr "${protoc_root}"
    chmod +x "${protoc_bin}"

    return 0
}

function proto_tools_install {
    local protoc_gen_go_version
    local mog_version
    local protoc_go_inject_tag_version

    protoc_gen_go_version="$(grep github.com/golang/protobuf go.mod | awk '{print $2}')"
    mog_version="$(make --no-print-directory print-MOG_VERSION)"
    protoc_go_inject_tag_version="$(make --no-print-directory print-PROTOC_GO_INJECT_TAG_VERSION)"

    # echo "go: ${protoc_gen_go_version}"
    # echo "mog: ${mog_version}"
    # echo "tag: ${protoc_go_inject_tag_version}"

    install_versioned_tool \
        'protoc-gen-go' \
        'github.com/golang/protobuf' \
        "${protoc_gen_go_version}" \
        'github.com/golang/protobuf/protoc-gen-go'

    install_unversioned_tool \
        protoc-gen-go-binary \
        'github.com/hashicorp/protoc-gen-go-binary@master'

    install_versioned_tool \
        'protoc-go-inject-tag' \
        'github.com/favadi/protoc-go-inject-tag' \
        "${protoc_go_inject_tag_version}" \
        'github.com/favadi/protoc-go-inject-tag'

    install_versioned_tool \
        'mog' \
        'github.com/hashicorp/mog' \
        "${mog_version}" \
        'github.com/hashicorp/mog'

    return 0
}

function install_unversioned_tool {
    local command="$1"
    local install="$2"

    if ! command -v "${command}" &>/dev/null ; then
        status_stage "installing tool: ${install}"
        go install "${install}"
    else
        debug "skipping tool: ${install} (installed)"
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
        err "cannot install '${command}' no version selected"
        return 1
    fi

    if [[ "$version" = "@DEV" ]]; then
        if ! command -v "${command}" &>/dev/null ; then
            err "dev version of '${command}' requested but not installed"
            return 1
        fi
        status "skipping tool: ${installbase} (using development version)"
        return 0
    fi

    if command -v "${command}" &>/dev/null ; then
        got="$(go version -m $(which "${command}") | grep '\bmod\b' | grep "${module}" |
            awk '{print $2 "@" $3}')"
        if [[ "$expect" != "$got" ]]; then
            should_install=1
        fi
    else
        should_install=1
    fi

    if [[ -n $should_install ]]; then
        status_stage "installing tool: ${install}"
        go install "${install}"
    else
        debug "skipping tool: ${install} (installed)"
    fi
    return 0
}

function generate_protobuf_code {
    local proto_path="${1:-}"
    if [[ -z "${proto_path}" ]]; then
        err "missing protobuf path argument"
        return 1
    fi

    if [[ -z "${golang_proto_path}" ]]; then
        err "golang_proto_path was not set"
        return 1
    fi
    if [[ -z "${golang_proto_mod_path}" ]]; then
        err "golang_proto_mod_path was not set"
        return 1
    fi

    local proto_go_path="${proto_path%%.proto}.pb.go"
    local proto_go_bin_path="${proto_path%%.proto}.pb.binary.go"
    local proto_go_rpcglue_path="${proto_path%%.proto}.rpcglue.pb.go"

    local go_proto_out='paths=source_relative,plugins=grpc:'

    status_stage "Generating ${proto_path} into ${proto_go_path} and ${proto_go_bin_path}"

    rm -f "${proto_go_path}" ${proto_go_bin_path}" ${proto_go_rpcglue_path}"

    print_run ${protoc_bin} \
        -I="${golang_proto_path}" \
        -I="${golang_proto_mod_path}" \
        -I="${SOURCE_DIR}" \
        --go_out="${go_proto_out}${SOURCE_DIR}" \
        --go-binary_out="${SOURCE_DIR}" \
        "${proto_path}" || {

      err "Failed to run protoc for ${proto_path}"
      return 1
    }

    print_run protoc-go-inject-tag -input="${proto_go_path}" || {
        err "Failed to run protoc-go-inject-tag for ${proto_path}"
        return 1
    }

    local build_tags
    build_tags="$(head -n 2 "${proto_path}" | grep '^//go:build\|// +build' || true)"
    if test -n "${build_tags}"; then
        echo -e "${build_tags}\n" >> "${proto_go_bin_path}.new"
        cat "${proto_go_bin_path}" >> "${proto_go_bin_path}.new"
        mv "${proto_go_bin_path}.new" "${proto_go_bin_path}"
    fi

    # NOTE: this has to run after we fix up the build tags above
    rm -f "${proto_go_rpcglue_path}"
    print_run go run ./internal/tools/proto-gen-rpc-glue/main.go -path "${proto_go_path}" || {
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

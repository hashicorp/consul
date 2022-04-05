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
    local expect_dev="${module}@(devel)"
    local install="${installbase}@${version}"

    if [[ -z "$version" ]]; then
        err "cannot install '${command}' no version selected"
        return 1
    fi

    if command -v "${command}" &>/dev/null ; then
        got="$(go version -m $(which "${command}") | grep '\bmod\b' | grep "${module}" |
            awk '{print $2 "@" $3}')"
        if [[ "$expect_dev" = "$got" ]]; then
            status "skipping tool: ${install} (using development version)"
            return 0
        elif [[ "$expect" != "$got" ]]; then
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

main "$@"
exit $?

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
    Installs various supporting Go tools.

Options:
    -protobuf                Just install tools for protobuf.
    -lint                    Just install tools for linting.
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
            -protobuf )
                proto_tools_install
                return 0
                ;;
            -lint )
                lint_install
                return 0
                ;;
            -h | --help )
                usage
                return 0
                ;;
        esac
    done

    # ensure these tools are installed
    tools_install
}

function proto_tools_install {
    local protoc_gen_go_version
    local protoc_gen_go_grpc_version
    local buf_version
    local mog_version
    local protoc_go_inject_tag_version

    protoc_gen_go_version="$(grep github.com/golang/protobuf go.mod | awk '{print $2}')"
    protoc_gen_go_grpc_version="$(make --no-print-directory print-PROTOC_GEN_GO_GRPC_VERSION)"
    mog_version="$(make --no-print-directory print-MOG_VERSION)"
    protoc_go_inject_tag_version="$(make --no-print-directory print-PROTOC_GO_INJECT_TAG_VERSION)"
    buf_version="$(make --no-print-directory print-BUF_VERSION)"

    # echo "go: ${protoc_gen_go_version}"
    # echo "mog: ${mog_version}"
    # echo "tag: ${protoc_go_inject_tag_version}"

    install_versioned_tool \
       'buf' \
        'github.com/bufbuild/buf' \
        "${buf_version}" \
        'github.com/bufbuild/buf/cmd/buf'

    install_versioned_tool \
        'protoc-gen-go' \
        'github.com/golang/protobuf' \
        "${protoc_gen_go_version}" \
        'github.com/golang/protobuf/protoc-gen-go'

    install_versioned_tool \
        'protoc-gen-go-grpc' \
        'google.golang.org/grpc/cmd/protoc-gen-go-grpc' \
        "${protoc_gen_go_grpc_version}" \
        'google.golang.org/grpc/cmd/protoc-gen-go-grpc'

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

function lint_install {
    local golangci_lint_version
    golangci_lint_version="$(make --no-print-directory print-GOLANGCI_LINT_VERSION)"

    install_unversioned_tool \
        'lint-consul-retry' \
        'github.com/hashicorp/lint-consul-retry@master'

    install_unversioned_tool \
        'enumcover' \
        'github.com/reillywatson/enumcover/cmd/enumcover@master'

    install_versioned_tool \
        'golangci-lint' \
        'github.com/golangci/golangci-lint' \
        "${golangci_lint_version}" \
        'github.com/golangci/golangci-lint/cmd/golangci-lint'
}

function tools_install {
    local mockery_version

    mockery_version="$(make --no-print-directory print-MOCKERY_VERSION)"

    install_versioned_tool \
        'mockery' \
        'github.com/vektra/mockery/v2' \
        "${mockery_version}" \
        'github.com/vektra/mockery/v2'

    lint_install
    proto_tools_install

    return 0
}

function install_unversioned_tool {
    local command="$1"
    local install="$2"

    if ! command -v "${command}" &>/dev/null ; then
        echo "installing tool: ${install}"
        go install "${install}"
    else
        echo "skipping tool: ${install} (installed)"
    fi

    return 0
}

function install_versioned_tool {
    local command="$1"
    local module="$2"
    local version="$3"
    local installbase="$4"

    local should_install=
    local install_reason=
    local got
    local vgot
    local vneed

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
        echo "skipping tool: ${installbase} (using development version)"
        return 0
    fi

    if command -v "${command}" &>/dev/null ; then
        got="$(go version -m $(which "${command}") | grep '\bmod\b' | grep "${module}" |
            awk '{print $2 "@" $3}')"
        if [[ "$expect" != "$got" ]]; then
            should_install=1
            install_reason="upgrade"
        fi

        # check that they were compiled with the current version of go
        set +o pipefail
        vgot="$(go version -m $(which "${command}") | head -n 1 | grep -o 'go[0-9.]\+')"
        vneed="$(go version | head -n 1 | awk '{print $3}')"
        set -o pipefail
        if [[ "$vgot" != "$vneed" ]]; then
            should_install=1
            install_reason="go toolchain upgrade"
        fi
    else
        should_install=1
        install_reason="install"
    fi

    if [[ -n $should_install ]]; then
        echo "installing tool (${install_reason}): ${install}"
        go install "${install}"
    else
        echo "skipping tool: ${install} (installed)"
    fi
    return 0
}

main "$@"
exit $?

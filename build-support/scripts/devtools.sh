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
    -lint                    Just install tools for linting.
    -bindata                 Just install tools for static assets.
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
            -bindata )
                bindata_install
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

function lint_install {
    local golangci_lint_version
    golangci_lint_version="$(make --no-print-directory print-GOLANGCI_LINT_VERSION)"

    install_unversioned_tool \
        'lint-consul-retry' \
        'github.com/hashicorp/lint-consul-retry@master'

    install_versioned_tool \
        'golangci-lint' \
        'github.com/golangci/golangci-lint' \
        "${golangci_lint_version}" \
        'github.com/golangci/golangci-lint/cmd/golangci-lint'
}

function bindata_install {
    install_unversioned_tool \
        'go-bindata' \
        'github.com/hashicorp/go-bindata/go-bindata@bf7910a'

    install_unversioned_tool \
        'go-bindata-assetfs' \
        'github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@38087fe'
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
    bindata_install

    return 0
}

main "$@"
exit $?

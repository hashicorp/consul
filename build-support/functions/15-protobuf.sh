# Installs the version of protoc specified by the first argument.
#
# Will set 'protoc_bin'
function protoc_install {
    local protoc_version="${1:-}"
    local protoc_os

    # TODO ERROR IF no version

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
        status_stage "protocol buffer compiler version already installed: ${protoc_version}"
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

    echo "go: ${protoc_gen_go_version}"
    echo "mog: ${mog_version}"
    echo "tag: ${protoc_go_inject_tag_version}"

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
    local expect_dev="${module}@(devel)"
    local install="${installbase}@${version}"

    if [[ -z "$version" ]]; then
        echo "cannot install '${command}' no version selected" >&2
        return 1
    fi

    if command -v "${command}" &>/dev/null ; then
        got="$(go version -m $(which "${command}") | grep '\bmod\b' | grep "${module}" |
            awk '{print $2 "@" $3}')"
        if [[ "$expect_dev" = "$got" ]]; then
            echo "=== TOOL: ${install} (skipped; using development version)"
        elif [[ "$expect" != "$got" ]]; then
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

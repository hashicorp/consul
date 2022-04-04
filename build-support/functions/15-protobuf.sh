# Installs the version of protoc specified by the first argument.
#
# Will set 'protoc_bin'
function protoc_install {
    local protoc_version="${1:-}"
    local protoc_os

    # TODO ERROR IF no version

    if test -z "${protoc_version}"
    then
        protoc_version="$(make print-PROTOC_VERSION)"
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

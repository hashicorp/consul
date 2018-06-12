# GPG Key ID to use for publically released builds
HASHICORP_GPG_KEY="348FFC4C"

# Default Image Names
UI_BUILD_CONTAINER_DEFAULT="consul-build-ui"
UI_LEGACY_BUILD_CONTAINER_DEFAULT="consul-build-ui-legacy"
GO_BUILD_CONTAINER_DEFAULT="consul-build-go"

# Whether to colorize shell output
COLORIZE=1


# determine GOPATH and the first GOPATH to use for intalling binaries
GOPATH=${GOPATH:-$(go env GOPATH)}
case $(uname) in
    CYGWIN*)
        GOPATH="$(cygpath $GOPATH)"
        ;;
esac
MAIN_GOPATH=$(cut -d: -f1 <<< "${GOPATH}")

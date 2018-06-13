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


# Build debugging output is off by default
if test -z "${BUILD_DEBUG}"
then
   BUILD_DEBUG=0
fi

# default publish host is github.com - only really useful to use something else for testing
if test -z "${PUBLISH_GIT_HOST}"
then
   PUBLISH_GIT_HOST=github.com
fi

# default publish repo is hashicorp/consul - useful to override for testing as well as in the enterprise repo
if test -z "${PUBLISH_GIT_REPO}"
then
   PUBLISH_GIT_REPO=hashicorp/consul.git
fi
# GPG Key ID to use for publically released builds
HASHICORP_GPG_KEY="348FFC4C"

# Default Image Names
UI_BUILD_CONTAINER_DEFAULT="consul-build-ui"
GO_BUILD_CONTAINER_DEFAULT="consul-build-go"

# Whether to colorize shell output
if tput reset &>/dev/null ; then
   COLORIZE=${COLORIZE-1}
else
   COLORIZE=0
fi

# determine GOPATH and the first GOPATH to use for installing binaries
if command -v go >/dev/null; then
   GOPATH=${GOPATH:-$(go env GOPATH)}
   case $(uname) in
         CYGWIN*)
            GOPATH="$(cygpath $GOPATH)"
            ;;
   esac
   MAIN_GOPATH=$(cut -d: -f1 <<< "${GOPATH}")
fi

# Build debugging output is off by default
BUILD_DEBUG=${BUILD_DEBUG-0}

# default publish host is github.com - only really useful to use something else for testing
PUBLISH_GIT_HOST="${PUBLISH_GIT_HOST-github.com}"

# default publish repo is hashicorp/consul - useful to override for testing as well as in the enterprise repo
PUBLISH_GIT_REPO="${PUBLISH_GIT_REPO-hashicorp/consul.git}"

CONSUL_PKG_NAME="consul"

if test "$(uname)" == "Darwin"
then
   SED_EXT="-E"
else
   SED_EXT="-r"
fi

CONSUL_BINARY_TYPE=oss

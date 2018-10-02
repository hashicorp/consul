#!/bin/bash
pushd $(dirname ${BASH_SOURCE[0]}) > /dev/null
SCRIPT_DIR=$(pwd)
pushd ../.. > /dev/null
SOURCE_DIR=$(pwd)
pushd build-support/docker > /dev/null
IMG_DIR=$(pwd)
popd > /dev/null

source "${SCRIPT_DIR}/functions.sh"

IMAGE="travis-img-v0.13"
CONTAINER="travis-cnt"
GOOS="linux"
GOARCH="amd64"
TEST_BINARY="flake.test"

function usage {
cat <<-EOF
Usage: test-flake [<options ...>]

Description:

  test-flake surfaces flakiness in tests by constraining CPU resources.
  
  Single or package-wide tests are run for multiple iterations with a configurable
  amount of CPU resources. 

  0.15 CPUs and 30 iterations are configured as sane defaults.

  See Docker docs for more info on tuning 'cpus' param: 
  https://docs.docker.com/config/containers/resource_constraints/#cpu

Options:

  --pkg=""             Target package
  --test=""            Target test (requires pkg flag)
  --cpus=0.15          Amount of CPU resources for container
  --n=30               Number of times to run tests

Examples:

  ./test-flake.sh --pkg connect/proxy
  ./test-flake.sh --pkg connect/proxy --cpus 0.20
  ./test-flake.sh --pkg connect/proxy --test Listener
  ./test-flake.sh --pkg connect/proxy --test TestUpstreamListener
  ./test-flake.sh --pkg connect/proxy --test TestUpstreamListener -n 100
EOF
}

function build_repro_env {
    # Arguments:
    #   $1 - pkg, Target package
    #   $2 - test, Target tests
    #   $3 - cpus, Amount of CPU resources for container
    #   $4 - n, Number of times to run tests

    APP=$(pwd | awk '{n=split($0, a, "/"); print a[n]}')

    status_stage -e "App:\t\t$APP"
    status_stage -e "Package:\t$1"
    status_stage -e "Test:\t\t$2"
    status_stage -e "CPUs:\t\t$3"
    status_stage -e "Iterations:\t$4"
    echo

    status_stage "----> Cleaning up old containers..."
    if docker ps -a | grep $CONTAINER ; then
        docker rm $(docker ps -a | grep $CONTAINER | awk '{print $1;}')
    fi

    status_stage '----> Rebuilding image...'
    (cd $IMG_DIR && docker build -q -t $IMAGE --no-cache -f Test-Flake.dockerfile .)

    status_stage "--> Building app binary..."
    env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/$APP

    status_stage "-> Building test binary..."
    env GOOS=$GOOS GOARCH=$GOARCH go test -c "./$1" -o $TEST_BINARY

    status_stage "> Running container..."
    status_stage

    docker run \
    --rm \
    --name $CONTAINER \
    --cpus="$3" \
    -v $SOURCE_DIR:/home/travis/go/$APP \
    -e TEST_BINARY="$TEST_BINARY" \
    -e TEST_PKG="$1" \
    -e TEST="$2" \
    -e ITERATIONS="$4" \
    -e APP="$APP" \
    $IMAGE
}

function err_usage {
   err "$1"
   err ""
   err "$(usage)"
}

function main {
   declare pkg=""
   declare test=""
   declare cpus=""
   declare n=""
   
   
   while test $# -gt 0
   do
      case "$1" in
         -h | --help )
            usage
            return 0
            ;;

         --pkg )
            if test -z "$2"
            then
               err_usage "ERROR: option pkg requires an argument"
               return 1
            fi
            
            pkg="$2"
            shift 2
            ;;

         --test )
            test="$2"
            shift 2
            ;;

         --cpus )
            if test -n "$2"
            then
               cpus="$2"
            else
               cpus="0.15"
            fi
            
            shift 2
            ;;

          --n )
            if test -n "$2"
            then
               n="$2"
            else
               n="30"
            fi

            shift 2
            ;;

         * )
            err_usage "ERROR: Unknown argument: '$1'"
            return 1
            ;;
      esac
   done
   
   build_repro_env "${pkg}" "${test}" "${cpus}" "${n}" || return 1
   
   return 0
}

main "$@"
exit $?
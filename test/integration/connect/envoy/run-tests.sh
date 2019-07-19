#!/bin/bash

set -euo pipefail

# DEBUG=1 enables set -x for this script so echos every command run
DEBUG=${DEBUG:-}

# FILTER_TESTS="<pattern>" skips any test whose CASENAME doesn't match the
# pattern. CASENAME is combination of the name from the case-<name> dir and the
# envoy version for example: "http, envoy 1.8.0". The pattern is passed to grep
# over that string.
FILTER_TESTS=${FILTER_TESTS:-}

# STOP_ON_FAIL exits after a case fails so the workdir state can be viewed and
# the components interacted with to debug the failure. This is useful when tests
# only fail when run as part of a whole suite but work in isolation.
STOP_ON_FAIL=${STOP_ON_FAIL:-}

# ENVOY_VERSIONS is the list of envoy versions to run each test against
ENVOY_VERSIONS=${ENVOY_VERSIONS:-"1.10.0 1.9.1 1.8.0"}

if [ ! -z "$DEBUG" ] ; then
  set -x
fi

DIR=$(cd -P -- "$(dirname -- "$0")" && pwd -P)

cd $DIR

FILTER_TESTS=${FILTER_TESTS:-}
LEAVE_CONSUL_UP=${LEAVE_CONSUL_UP:-}
PROXY_LOGS_ON_FAIL=${PROXY_LOGS_ON_FAIL:-}

source helpers.bash

RESULT=1
CLEANED_UP=0

PREV_CMD=""
THIS_CMD=""

function cleanup {
  local STATUS="$?"
  local CMD="$THIS_CMD"

  if [ "$CLEANED_UP" != 0 ] ; then
    return
  fi
  CLEANED_UP=1

  if [ $STATUS -ne 0 ] ; then
    # We failed due to set -e catching an error, output some useful info about
    # that error.
    echo "ERR: command exited with $STATUS"
    echo "     command: $CMD"
  fi

  docker-compose down
}
trap cleanup EXIT
# Magic to capture commands and statuses so we can show them when we exit due to
# set -e This is useful for debugging setup.sh failures.
trap 'PREV_CMD=$THIS_CMD; THIS_CMD=$BASH_COMMAND' DEBUG

# Start the volume container
docker-compose up -d workdir

for c in ./case-*/ ; do
  for ev in $ENVOY_VERSIONS ; do
    CASE_NAME=$( basename $c | cut -c6- )
    CASE_ENVOY_VERSION="envoy $ev"
    CASE_STR="$CASE_NAME, $CASE_ENVOY_VERSION"
    echo "================================================"
    echoblue "CASE $CASE_STR"
    echo "- - - - - - - - - - - - - - - - - - - - - - - -"

    export ENVOY_VERSION=$ev

    if [ ! -z "$FILTER_TESTS" ] && echo "$CASE_STR" | grep -v "$FILTER_TESTS" > /dev/null ; then
      echo "   SKIPPED: doesn't match FILTER_TESTS=$FILTER_TESTS"
      continue 1
    fi

    # Wipe state
    docker-compose up wipe-volumes
    # Note, we use explicit set of dirs so we don't delete .gitignore. Also,
    # don't wipe logs between runs as they are already split and we need them to
    # upload as artifacts later.
    rm -rf workdir/{consul,envoy,bats,statsd}
    mkdir -p workdir/{consul,envoy,bats,statsd,logs}

    # Reload consul config from defaults
    cp consul-base-cfg/* workdir/consul

    # Add any overrides if there are any (no op if not)
    cp -f ${c}*.hcl workdir/consul 2>/dev/null || :

    # Push the state to the shared docker volume (note this is because CircleCI
    # can't use shared volumes)
    docker cp workdir/. envoy_workdir_1:/workdir

    # Start consul now as setup script needs it up
    docker-compose up -d consul

    # Copy all the test files
    cp ${c}*.bats workdir/bats
    cp helpers.bash workdir/bats

    # Run test case setup (e.g. generating Envoy bootstrap, starting containers)
    source ${c}setup.sh

    # Push the state to the shared docker volume (note this is because CircleCI
    # can't use shared volumes)
    docker cp workdir/. envoy_workdir_1:/workdir

    # Start containers required
    if [ ! -z "$REQUIRED_SERVICES" ] ; then
      docker-compose up --build -d $REQUIRED_SERVICES
    fi

    # Execute tests
    THISRESULT=1
    if docker-compose up --build --exit-code-from verify verify ; then
      echo "- - - - - - - - - - - - - - - - - - - - - - - -"
      echoblue -n "CASE $CASE_STR"
      echo -n ": "
      echogreen "✓ PASS"
    else
      echo "- - - - - - - - - - - - - - - - - - - - - - - -"
      echoblue -n "CASE $CASE_STR"
      echo -n ": "
      echored "⨯ FAIL"
      if [ $RESULT -eq 1 ] ; then
        RESULT=0
      fi
      THISRESULT=0
    fi
    echo "================================================"

    # Hack consul into the list of containers to stop and dump logs for.
    REQUIRED_SERVICES="$REQUIRED_SERVICES consul"

    # Teardown
    if [ -f "${c}teardown.sh" ] ; then
      source "${c}teardown.sh"
    fi
    if [ ! -z "$REQUIRED_SERVICES" ] ; then
      if [[ "$THISRESULT" == 0 ]] ; then
        mkdir -p workdir/logs/$c/$ENVOY_VERSION
        for cont in $REQUIRED_SERVICES; do
          docker-compose logs --no-color $cont 2>&1 > workdir/logs/$c/$ENVOY_VERSION/$cont.log
        done
      fi
      docker-compose stop $REQUIRED_SERVICES
    fi

    if [ $RESULT -eq 0 ] && [ ! -z "$STOP_ON_FAIL" ] ; then
      echo "  => STOPPING because STOP_ON_FAIL set"
      break 2
    fi
  done
done

cleanup

if [ $RESULT -eq 1 ] ; then
  echogreen "✓ PASS"
else
  echored "⨯ FAIL"
  exit 1
fi

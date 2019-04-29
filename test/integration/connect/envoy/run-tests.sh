#!/bin/bash

set -euo pipefail

# DEBUG=1 enables set -x for this script so echos every command run
DEBUG=${DEBUG:-}

# FILTER_TESTS="<pattern>" skips any test whose CASENAME doesn't match the
# pattern. CASENAME is combination of the name from the case-<name> dir and the
# envoy version for example: "http, envoy 1.8.0". The pattern is passed to grep
# over that string.
FILTER_TESTS=${FILTER_TESTS:-}

# LEAVE_CONSUL_UP=1 leaves the consul container running at the end which can be
# useful for debugging.
LEAVE_CONSUL_UP=${LEAVE_CONSUL_UP:-}

# QUIESCE_SECS=1 will cause the runner to sleep for 1 second after setup but
# before verify container is run this is useful for CI which seems to pass more
# reliably with this even though docker-compose up waits for containers to
# start, and our tests retry.
QUIESCE_SECS=${QUIESCE_SECS:-}

# ENVOY_VERSIONS is the list of envoy versions to run each test against
ENVOY_VERSIONS=${ENVOY_VERSIONS:-"1.8.0 1.9.1"}

if [ ! -z "$DEBUG" ] ; then
  set -x
fi

DIR=$(cd -P -- "$(dirname -- "$0")" && pwd -P)

cd $DIR

FILTER_TESTS=${FILTER_TESTS:-}
LEAVE_CONSUL_UP=${LEAVE_CONSUL_UP:-}
PROXY_LOGS_ON_FAIL=${PROXY_LOGS_ON_FAIL:-}

mkdir -p workdir/{consul,envoy,bats,statsd,logs}

source helpers.bash

RESULT=1
CLEANED_UP=0

function cleanup {
  if [ "$CLEANED_UP" != 0 ] ; then
    return
  fi
  CLEANED_UP=1

  if [ -z "$LEAVE_CONSUL_UP" ] ; then
    docker-compose down
  fi
}
trap cleanup EXIT

# Start the volume container
docker-compose up -d workdir

for c in ./case-*/ ; do
  for ev in $ENVOY_VERSIONS ; do
    CASENAME="$( basename $c | cut -c6- ), envoy $ev"
    echo ""
    echo "==> CASE $CASENAME"

    export ENVOY_VERSION=$ev

    if [ ! -z "$FILTER_TESTS" ] && echo "$CASENAME" | grep -v "$FILTER_TESTS" > /dev/null ; then
      echo "   SKIPPED: doesn't match FILTER_TESTS=$FILTER_TESTS"
      continue 1
    fi

    # Wipe state
    docker-compose up wipe-volumes

    # Reload consul config from defaults
    cp consul-base-cfg/* workdir/consul

    # Add any overrides if there are any (no op if not)
    cp -f ${c}*.hcl workdir/consul 2>/dev/null || :

    # Push the state to the shared docker volume (note this is because CircleCI
    # can't use shared volumes)
    docker cp workdir/. envoy_workdir_1:/workdir

    # Start Consul first we do this here even though typically nothing stopped
    # it because it sometimes seems to be killed by something else (OOM killer)?
    docker-compose up -d consul

    # Reload consul
    echo "Reloading Consul config"
    if ! retry 10 2 docker_consul reload ; then
      # Clean up everything before we abort
      #docker-compose down
      echored "⨯ FAIL - couldn't reload consul config"
      exit 1
    fi

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
      docker-compose up -d $REQUIRED_SERVICES
    fi

    if [ ! -z "$QUIESCE_SECS" ] ; then
      echo "Sleeping for $QUIESCE_SECS seconds"
      sleep $QUIESCE_SECS
    fi

    # Execute tests
    THISRESULT=1
    if docker-compose up --build --abort-on-container-exit --exit-code-from verify verify ; then
      echo -n "==> CASE $CASENAME: "
      echogreen "✓ PASS"
    else
      echo -n "==> CASE $CASENAME: "
      echored "⨯ FAIL"
      if [ $RESULT -eq 1 ] ; then
        RESULT=0
      fi
      THISRESULT=0
    fi

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
  done
done

cleanup

if [ $RESULT -eq 1 ] ; then
  echogreen "✓ PASS"
else
  echored "⨯ FAIL"
  exit 1
fi

#!/bin/bash

set -eEuo pipefail

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
ENVOY_VERSIONS=${ENVOY_VERSIONS:-"1.10.0 1.11.2 1.12.2 1.13.0"}

if [ ! -z "$DEBUG" ] ; then
  set -x
fi

DIR=$(cd -P -- "$(dirname -- "$0")" && pwd -P)

cd $DIR

LEAVE_CONSUL_UP=${LEAVE_CONSUL_UP:-}
PROXY_LOGS_ON_FAIL=${PROXY_LOGS_ON_FAIL:-}

source helpers.bash

RESULT=1
CLEANED_UP=0

function cleanup {
  local STATUS="$?"

  if [ "$CLEANED_UP" != 0 ] ; then
    return
  fi
  CLEANED_UP=1

  if [ "$STATUS" -ne 0 ]
  then
    capture_logs
  fi

  docker-compose down -v --remove-orphans
}
trap cleanup EXIT

function command_error {
  echo "ERR: command exited with status $1" 1>&2
  echo "     command:   $2" 1>&2
  echo "     line:      $3" 1>&2
  echo "     function:  $4" 1>&2
  echo "     called at: $5" 1>&2
  # printf '%s\n' "${FUNCNAME[@]}"
  # printf '%s\n' "${BASH_SOURCE[@]}"
  # printf '%s\n' "${BASH_LINENO[@]}"
}

trap 'command_error $? "${BASH_COMMAND}" "${LINENO}" "${FUNCNAME[0]:-main}" "${BASH_SOURCE[0]}:${BASH_LINENO[0]}"' ERR

# Cleanup from any previous unclean runs.
docker-compose down -v --remove-orphans

# Start the volume container
docker-compose up -d workdir

function init_workdir {
  local DC="$1"

  if test -z "$DC"
  then
    DC=primary
  fi

  # Note, we use explicit set of dirs so we don't delete .gitignore. Also,
  # don't wipe logs between runs as they are already split and we need them to
  # upload as artifacts later.
  rm -rf workdir/${DC}
  mkdir -p workdir/${DC}/{consul,envoy,bats,statsd,data}

  # Reload consul config from defaults
  cp consul-base-cfg/* workdir/${DC}/consul/

  # Add any overrides if there are any (no op if not)
  find ${CASE_DIR} -maxdepth 1 -name '*.hcl' -type f -exec cp -f {} workdir/${DC}/consul \;

  # Copy all the test files
  find ${CASE_DIR} -maxdepth 1 -name '*.bats' -type f -exec cp -f {} workdir/${DC}/bats \;
  # Copy DC specific bats
  cp helpers.bash workdir/${DC}/bats

  # Add any DC overrides
  if test -d "${CASE_DIR}/${DC}"
  then
    find ${CASE_DIR}/${DC} -type f -name '*.hcl' -exec cp -f {} workdir/${DC}/consul \;
    find ${CASE_DIR}/${DC} -type f -name '*.bats' -exec cp -f {} workdir/${DC}/bats \;
  fi
  
  if test -d "${CASE_DIR}/data"
  then
    cp -r ${CASE_DIR}/data/* workdir/${DC}/data
  fi
  
  return 0
}

function start_consul {
  local DC=${1:-primary}

  # Start consul now as setup script needs it up
  docker-compose rm -s -v -f consul-${DC} || true
  docker-compose up -d consul-${DC}
}

function pre_service_setup {
  local DC=${1:-primary}

  # Run test case setup (e.g. generating Envoy bootstrap, starting containers)
  if [ -f "${CASE_DIR}${DC}/setup.sh" ]
  then
    source ${CASE_DIR}${DC}/setup.sh
  else
    source ${CASE_DIR}setup.sh
  fi
}

function start_services {
  # Push the state to the shared docker volume (note this is because CircleCI
  # can't use shared volumes)
  docker cp workdir/. envoy_workdir_1:/workdir
  
  # Start containers required
  if [ ! -z "$REQUIRED_SERVICES" ] ; then
    docker-compose rm -s -v -f $REQUIRED_SERVICES || true
    docker-compose up --build -d $REQUIRED_SERVICES
  fi

  return 0
}

function verify {
  local DC=$1
  if test -z "$DC"
  then
    DC=primary
  fi

  # Execute tests
  res=0

  echo "- - - - - - - - - - - - - - - - - - - - - - - -"
  echoblue -n "CASE $CASE_STR"
  echo -n ": "

  # Nuke any previous case's verify container.
  docker-compose rm -s -v -f verify-${DC} || true

  if docker-compose up --abort-on-container-exit --exit-code-from verify-${DC} verify-${DC} ; then
    echogreen "✓ PASS"
  else
    echored "⨯ FAIL"
    res=1
  fi
  echo "================================================"

  return $res
}

function capture_logs {
  echo "Capturing Logs for $CASE_STR"
  mkdir -p "$LOG_DIR"
  services="$REQUIRED_SERVICES consul-primary"
  if is_set $REQUIRE_SECONDARY
  then
    services="$services consul-secondary"
  fi

  if [ -f "${CASE_DIR}capture.sh" ]
  then
    echo "Executing ${CASE_DIR}capture.sh"
    source ${CASE_DIR}capture.sh || true
  fi


  for cont in $services
  do
    echo "Capturing log for $cont"
    docker-compose logs --no-color "$cont" 2>&1 > "${LOG_DIR}/${cont}.log"
  done
}

function stop_services {

  # Teardown
  if [ -f "${CASE_DIR}teardown.sh" ] ; then
    source "${CASE_DIR}teardown.sh"
  fi
  docker-compose rm -s -v -f $REQUIRED_SERVICES || true
}

function initVars {
  source "defaults.sh"
  if [ -f "${CASE_DIR}vars.sh" ] ; then
    source "${CASE_DIR}vars.sh"
  fi
}

function global_setup {
  if [ -f "${CASE_DIR}global-setup.sh" ] ; then
    source "${CASE_DIR}global-setup.sh"
  fi
}

function runTest {
  initVars

  # Initialize the workdir
  init_workdir primary

  if is_set $REQUIRE_SECONDARY
  then
    init_workdir secondary
  fi

  global_setup

  # Wipe state
  docker-compose up wipe-volumes

  # Push the state to the shared docker volume (note this is because CircleCI
  # can't use shared volumes)
  docker cp workdir/. envoy_workdir_1:/workdir

  start_consul primary
  if [ $? -ne 0 ]
  then
    capture_logs
    return 1
  fi

  if is_set $REQUIRE_SECONDARY
  then
    start_consul secondary
    if [ $? -ne 0 ]
    then
      capture_logs
      return 1
    fi
  fi

  echo "Setting up the primary datacenter"
  pre_service_setup primary
  if [ $? -ne 0 ]
  then
    echo "Setting up the primary datacenter failed"
    capture_logs
    return 1
  fi

  if is_set $REQUIRE_SECONDARY
  then
    echo "Setting up the secondary datacenter"
    pre_service_setup secondary
    if [ $? -ne 0 ]
    then
      echo "Setting up the secondary datacenter failed"
      capture_logs
      return 1
    fi
  fi

  echo "Starting services"
  start_services
  if [ $? -ne 0 ]
  then
    capture_logs
    return 1
  fi

  # Run the verify container and report on the output
  verify primary
  TESTRESULT=$?

  if is_set $REQUIRE_SECONDARY && test "$TESTRESULT" -eq 0
  then
    verify secondary
    SECONDARYRESULT=$?

    if [ "$SECONDARYRESULT" -ne 0 ]
    then
      TESTRESULT=$SECONDARYRESULT
    fi
  fi

  if [ "$TESTRESULT" -ne 0 ]
  then
    capture_logs
  fi

  stop_services primary

  if is_set $REQUIRE_SECONDARY
  then
    stop_services secondary
  fi


  return $TESTRESULT
}


RESULT=0

for c in ./case-*/ ; do
  for ev in $ENVOY_VERSIONS ; do
    export CASE_DIR="${c}"
    export CASE_NAME=$( basename $c | cut -c6- )
    export CASE_ENVOY_VERSION="envoy $ev"
    export CASE_STR="$CASE_NAME, $CASE_ENVOY_VERSION"
    export ENVOY_VERSION="${ev}"
    export LOG_DIR="workdir/logs/${CASE_DIR}/${ENVOY_VERSION}"
    echo "================================================"
    echoblue "CASE $CASE_STR"
    echo "- - - - - - - - - - - - - - - - - - - - - - - -"

    if [ ! -z "$FILTER_TESTS" ] && echo "$CASE_STR" | grep -v "$FILTER_TESTS" > /dev/null ; then
      echo "   SKIPPED: doesn't match FILTER_TESTS=$FILTER_TESTS"
      continue 1
    fi

    if ! runTest
    then
      RESULT=1
    fi

    if [ $RESULT -ne 0 ] && [ ! -z "$STOP_ON_FAIL" ] ; then
      echo "  => STOPPING because STOP_ON_FAIL set"
      break 2
    fi
  done
done

cleanup

if [ $RESULT -eq 0 ] ; then
  echogreen "✓ PASS"
else
  echored "⨯ FAIL"
  exit 1
fi

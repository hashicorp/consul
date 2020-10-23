#!/usr/bin/env bash

set -eEuo pipefail

# DEBUG=1 enables set -x for this script so echos every command run
DEBUG=${DEBUG:-}

# HAPROXY_CONSUL_CONNECT_VERSION to run each test against
HAPROXY_CONSUL_CONNECT_VERSION=${HAPROXY_CONSUL_CONNECT_VERSION:-"0.9.0"}
export HAPROXY_CONSUL_CONNECT_VERSION

if [ ! -z "$DEBUG" ] ; then
  set -x
fi

source helpers.bash

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
  mkdir -p workdir/${DC}/{consul,haproxy,bats,statsd,data}

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
  docker-compose kill consul-${DC} || true
  docker-compose rm -v -f consul-${DC} || true
  docker-compose up -d consul-${DC}
}

function pre_service_setup {
  local DC=${1:-primary}

  # Run test case setup (e.g. generating Envoy bootstrap, starting containers)
  if [ -f "${CASE_DIR}/${DC}/setup.sh" ]
  then
    source ${CASE_DIR}/${DC}/setup.sh
  else
    source ${CASE_DIR}/setup.sh
  fi
}

function start_services {
  # Push the state to the shared docker volume (note this is because CircleCI
  # can't use shared volumes)
  docker cp workdir/. haproxy_workdir_1:/workdir

  # Start containers required
  if [ ! -z "$REQUIRED_SERVICES" ] ; then
    docker-compose kill $REQUIRED_SERVICES || true
    docker-compose rm -v -f $REQUIRED_SERVICES || true
    docker-compose up --build -d $REQUIRED_SERVICES
  fi
  return 0
}

function verify {
  local DC=$1
  if test -z "$DC"; then
    DC=primary
  fi

  # Execute tests
  res=0

  # Nuke any previous case's verify container.
  docker-compose kill verify-${DC} || true
  docker-compose rm -v -f verify-${DC} || true

  if docker-compose up --abort-on-container-exit --exit-code-from verify-${DC} verify-${DC} ; then
    echogreen "✓ PASS"
  else
    echored "⨯ FAIL"
    res=1
  fi

  return $res
}

function capture_logs {
  # exported to prevent docker-compose warning about unset var
  export LOG_DIR="workdir/logs/${CASE_DIR}/${HAPROXY_CONSUL_CONNECT_VERSION}"

  init_vars

  echo "Capturing Logs"
  mkdir -p "$LOG_DIR"
  services="$REQUIRED_SERVICES consul-primary"
  if is_set $REQUIRE_SECONDARY
  then
    services="$services consul-secondary"
  fi

  if [ -f "${CASE_DIR}/capture.sh" ]
  then
    echo "Executing ${CASE_DIR}/capture.sh"
    source ${CASE_DIR}/capture.sh || true
  fi

  for cont in $services
  do
    echo "Capturing log for $cont"
    docker-compose logs --no-color "$cont" 2>&1 > "${LOG_DIR}/${cont}.log"
  done
}

function stop_services {
  # Teardown
  if [ -f "${CASE_DIR}/teardown.sh" ] ; then
    source "${CASE_DIR}/teardown.sh"
  fi
  docker-compose kill $REQUIRED_SERVICES || true
  docker-compose rm -v -f $REQUIRED_SERVICES || true
}

function init_vars {
  source "defaults.sh"
  if [ -f "${CASE_DIR}/vars.sh" ] ; then
    source "${CASE_DIR}/vars.sh"
  fi
}

function global_setup {
  if [ -f "${CASE_DIR}/global-setup.sh" ] ; then
    source "${CASE_DIR}/global-setup.sh"
  fi
}

function pre_verify_setup {
  if [ -f "${CASE_DIR}/pre-verify-setup.sh" ] ; then
    source "${CASE_DIR}/pre-verify-setup.sh"
  fi
}

function run_tests {
  CASE_DIR="${CASE_DIR?CASE_DIR must be set to the path of the test case}"
  CASE_NAME=$( basename $CASE_DIR | cut -c6- )
  export CASE_NAME

  export LOG_DIR="workdir/logs/${CASE_DIR}/${HAPROXY_CONSUL_CONNECT_VERSION}"

  init_vars

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
  docker cp workdir/. haproxy_workdir_1:/workdir

  start_consul primary

  if is_set $REQUIRE_SECONDARY; then
    start_consul secondary
  fi

  echo "Setting up the primary datacenter"
  pre_service_setup primary

  if is_set $REQUIRE_SECONDARY; then
    echo "Setting up the secondary datacenter"
    pre_service_setup secondary
  fi

  echo "Starting services"
  start_services

  # Setup before tests execution
  pre_verify_setup

  # Run the verify container and report on the output
  verify primary

  if is_set $REQUIRE_SECONDARY; then
    verify secondary
  fi
}

function test_teardown {
    # Set a log dir to prevent docker-compose warning about unset var
    export LOG_DIR="workdir/logs/"

    init_vars

    stop_services primary

    if is_set $REQUIRE_SECONDARY; then
      stop_services secondary
    fi
}

function suite_setup {
    # Set a log dir to prevent docker-compose warning about unset var
    export LOG_DIR="workdir/logs/"
    # Cleanup from any previous unclean runs.
    docker-compose down --volumes --timeout 0 --remove-orphans

    # Start the volume container
    docker-compose up -d workdir
}

function suite_teardown {
    # Set a log dir to prevent docker-compose warning about unset var
    export LOG_DIR="workdir/logs/"

    docker-compose down --volumes --timeout 0 --remove-orphans
}


case "${1-}" in
  "")
    echo "command required"
    exit 1 ;;
  *)
    "$@" ;;
esac


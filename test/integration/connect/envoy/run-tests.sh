#!/usr/bin/env bash

set -eEuo pipefail

readonly self_name="$0"

readonly HASHICORP_DOCKER_PROXY="docker.mirror.hashicorp.services"

# DEBUG=1 enables set -x for this script so echos every command run
DEBUG=${DEBUG:-}

# ENVOY_VERSION to run each test against
ENVOY_VERSION=${ENVOY_VERSION:-"1.22.2"}
export ENVOY_VERSION

export DOCKER_BUILDKIT=1

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

readonly WORKDIR_SNIPPET='-v envoy_workdir:/workdir'

function network_snippet {
    local DC="$1"
    echo "--net container:envoy_consul-${DC}_1"
}

function init_workdir {
  local CLUSTER="$1"

  if test -z "$CLUSTER"
  then
    CLUSTER=primary
  fi

  # Note, we use explicit set of dirs so we don't delete .gitignore. Also,
  # don't wipe logs between runs as they are already split and we need them to
  # upload as artifacts later.
  rm -rf workdir/${CLUSTER}
  mkdir -p workdir/${CLUSTER}/{consul,register,envoy,bats,statsd,data}

  # Reload consul config from defaults
  cp consul-base-cfg/*.hcl workdir/${CLUSTER}/consul/

  # Add any overrides if there are any (no op if not)
  find ${CASE_DIR} -maxdepth 1 -name '*.hcl' -type f -exec cp -f {} workdir/${CLUSTER}/consul \;

  # Copy all the test files
  find ${CASE_DIR} -maxdepth 1 -name '*.bats' -type f -exec cp -f {} workdir/${CLUSTER}/bats \;
  # Copy CLUSTER specific bats
  cp helpers.bash workdir/${CLUSTER}/bats

  # Add any CLUSTER overrides
  if test -d "${CASE_DIR}/${CLUSTER}"
  then
    find ${CASE_DIR}/${CLUSTER} -type f -name '*.hcl' -exec cp -f {} workdir/${CLUSTER}/consul \;
    find ${CASE_DIR}/${CLUSTER} -type f -name '*.bats' -exec cp -f {} workdir/${CLUSTER}/bats \;
  fi

  # move all of the registration files OUT of the consul config dir now
  find workdir/${CLUSTER}/consul -type f -name 'service_*.hcl' -exec mv -f {} workdir/${CLUSTER}/register \;

  # copy the ca-certs for SDS so we can verify the right ones are served
  mkdir -p workdir/test-sds-server/certs
  cp test-sds-server/certs/ca-root.crt workdir/test-sds-server/certs/ca-root.crt

  if test -d "${CASE_DIR}/data"
  then
    cp -r ${CASE_DIR}/data/* workdir/${CLUSTER}/data
  fi

  return 0
}

function docker_kill_rm {
  local name
  local todo=()
  for name in "$@"; do
    name="envoy_${name}_1"
    if docker container inspect $name &>/dev/null; then
      if [[ "$name" == envoy_tcpdump-* ]]; then
        echo -n "Gracefully stopping $name..."
        docker stop $name &> /dev/null
        echo "done"
      fi
      todo+=($name)
    fi
  done

  if [[ ${#todo[@]} -eq 0 ]]; then
      return 0
  fi

  echo -n "Killing and removing: ${todo[@]}..."
  docker rm -v -f ${todo[@]} &> /dev/null
  echo "done"
}

function start_consul {
  local DC=${1:-primary}

  # Start consul now as setup script needs it up
  docker_kill_rm consul-${DC}

  # 8500/8502 are for consul
  # 9411 is for zipkin which shares the network with consul
  # 16686 is for jaeger ui which also shares the network with consul
  ports=(
    '-p=8500:8500'
    '-p=8502:8502'
    '-p=9411:9411'
    '-p=16686:16686'
  )
  case "$DC" in
    secondary)
      ports=(
        '-p=9500:8500'
        '-p=9502:8502'
      )
      ;;
    alpha)
      ports=(
        '-p=9510:8500'
        '-p=9512:8502'
      )
      ;;
  esac

  license="${CONSUL_LICENSE:-}"
  # load the consul license so we can pass it into the consul
  # containers as an env var in the case that this is a consul
  # enterprise test
  if test -z "$license" -a -n "${CONSUL_LICENSE_PATH:-}"
  then
    license=$(cat $CONSUL_LICENSE_PATH)
  fi

  # Run consul and expose some ports to the host to make debugging locally a
  # bit easier.
  #
  docker run -d --name envoy_consul-${DC}_1 \
    --net=envoy-tests \
    $WORKDIR_SNIPPET \
    --hostname "consul-${DC}" \
    --network-alias "consul-${DC}" \
    -e "CONSUL_LICENSE=$license" \
    ${ports[@]} \
    consul-dev \
    agent -dev -datacenter "${DC}" \
    -config-dir "/workdir/${DC}/consul" \
    -client "0.0.0.0" >/dev/null
}

function start_partitioned_client {
  local PARTITION=${1:-ap1}

  # Start consul now as setup script needs it up
  docker_kill_rm consul-${PARTITION}

  license="${CONSUL_LICENSE:-}"
  # load the consul license so we can pass it into the consul
  # containers as an env var in the case that this is a consul
  # enterprise test
  if test -z "$license" -a -n "${CONSUL_LICENSE_PATH:-}"
  then
    license=$(cat $CONSUL_LICENSE_PATH)
  fi

  sh -c "rm -rf /workdir/${PARTITION}/data"

  # Run consul and expose some ports to the host to make debugging locally a
  # bit easier.
  #
  docker run -d --name envoy_consul-${PARTITION}_1 \
    --net=envoy-tests \
    $WORKDIR_SNIPPET \
    --hostname "consul-${PARTITION}" \
    --network-alias "consul-${PARTITION}" \
    -e "CONSUL_LICENSE=$license" \
    consul-dev agent \
    -datacenter "primary" \
    -retry-join "consul-primary" \
    -grpc-port 8502 \
    -data-dir "/tmp/consul" \
    -config-dir "/workdir/${PARTITION}/consul" \
    -client "0.0.0.0" >/dev/null
}

function pre_service_setup {
  local CLUSTER=${1:-primary}

  # Run test case setup (e.g. generating Envoy bootstrap, starting containers)
  if [ -f "${CASE_DIR}/${CLUSTER}/setup.sh" ]
  then
    source ${CASE_DIR}/${CLUSTER}/setup.sh
  else
    source ${CASE_DIR}/setup.sh
  fi
}

function start_services {
  # Push the state to the shared docker volume (note this is because CircleCI
  # can't use shared volumes)
  docker cp workdir/. envoy_workdir_1:/workdir

  # Start containers required
  if [ ! -z "$REQUIRED_SERVICES" ] ; then
    docker_kill_rm $REQUIRED_SERVICES
    run_containers $REQUIRED_SERVICES
  fi

  return 0
}

function verify {
  local CLUSTER="$1"
  if test -z "$CLUSTER"; then
    CLUSTER="primary"
  fi

  # Execute tests
  res=0

  # Nuke any previous case's verify container.
  docker_kill_rm verify-${CLUSTER}

  echo "Running ${CLUSTER} verification step for ${CASE_DIR}..."

  # need to tell the PID 1 inside of the container that it won't be actual PID
  # 1 because we're using --pid=host so we use TINI_SUBREAPER
  if docker run --name envoy_verify-${CLUSTER}_1 -t \
    -e TINI_SUBREAPER=1 \
    -e ENVOY_VERSION \
    $WORKDIR_SNIPPET \
    --pid=host \
    $(network_snippet $CLUSTER) \
    bats-verify \
    --pretty /workdir/${CLUSTER}/bats ; then
    echogreen "✓ PASS"
  else
    echored "⨯ FAIL"
    res=1
  fi

  return $res
}

function capture_logs {
  local LOG_DIR="workdir/logs/${CASE_DIR}/${ENVOY_VERSION}"

  init_vars

  echo "Capturing Logs"
  mkdir -p "$LOG_DIR"
  services="$REQUIRED_SERVICES consul-primary"
  if is_set $REQUIRE_SECONDARY
  then
    services="$services consul-secondary"
  fi
  if is_set $REQUIRE_PARTITIONS
  then
    services="$services consul-ap1"
  fi
  if is_set $REQUIRE_PEERS
  then
      services="$services consul-alpha"
  fi

  if [ -f "${CASE_DIR}/capture.sh" ]
  then
    echo "Executing ${CASE_DIR}/capture.sh"
    source ${CASE_DIR}/capture.sh || true
  fi

  for cont in $services; do
    echo "Capturing log for $cont"
    docker logs "envoy_${cont}_1" &> "${LOG_DIR}/${cont}.log" || {
        echo "EXIT CODE $?" > "${LOG_DIR}/${cont}.log"
    }
  done
}

function stop_services {
  # Teardown
  docker_kill_rm $REQUIRED_SERVICES

  docker_kill_rm consul-primary consul-secondary consul-ap1 consul-alpha
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

function wipe_volumes {
  docker run --rm -i \
    $WORKDIR_SNIPPET \
    --net=none \
    "${HASHICORP_DOCKER_PROXY}/alpine" \
    sh -c 'rm -rf /workdir/*'
}

function run_tests {
  CASE_DIR="${CASE_DIR?CASE_DIR must be set to the path of the test case}"
  CASE_NAME=$( basename $CASE_DIR | cut -c6- )
  export CASE_NAME
  export SKIP_CASE=""

  init_vars

  # Initialize the workdir
  init_workdir primary

  if is_set $REQUIRE_SECONDARY
  then
    init_workdir secondary
  fi
  if is_set $REQUIRE_PARTITIONS
  then
    init_workdir ap1
  fi
  if is_set $REQUIRE_PEERS
  then
    init_workdir alpha
  fi

  global_setup

  # Allow vars.sh to set a reason to skip this test case based on the ENV
  if [ "$SKIP_CASE" != "" ] ; then
    echoyellow "SKIPPING CASE: $SKIP_CASE"
    return 0
  fi

  # Wipe state
  wipe_volumes

  # Push the state to the shared docker volume (note this is because CircleCI
  # can't use shared volumes)
  docker cp workdir/. envoy_workdir_1:/workdir

  start_consul primary

  if is_set $REQUIRE_SECONDARY; then
    start_consul secondary
  fi
  if is_set $REQUIRE_PARTITIONS; then
    docker_consul "primary" consul partition create -name ap1 > /dev/null
    start_partitioned_client ap1
  fi
  if is_set $REQUIRE_PEERS; then
    start_consul alpha
  fi

  echo "Setting up the primary datacenter"
  pre_service_setup primary

  if is_set $REQUIRE_SECONDARY; then
    echo "Setting up the secondary datacenter"
    pre_service_setup secondary
  fi
  if is_set $REQUIRE_PARTITIONS; then
    echo "Setting up the non-default partition"
    pre_service_setup ap1
  fi
  if is_set $REQUIRE_PEERS; then
    echo "Setting up the alpha peer"
    pre_service_setup alpha
  fi

  echo "Starting services"
  start_services

  # Run the verify container and report on the output
  echo "Verifying the primary datacenter"
  verify primary

  if is_set $REQUIRE_SECONDARY; then
    echo "Verifying the secondary datacenter"
    verify secondary
  fi
  if is_set $REQUIRE_PEERS; then
    echo "Verifying the alpha peer"
    verify alpha
  fi
}

function test_teardown {
    init_vars

    stop_services
}

function workdir_cleanup {
  docker_kill_rm workdir
  docker volume rm -f envoy_workdir &>/dev/null || true
}


function suite_setup {
    # Cleanup from any previous unclean runs.
    suite_teardown

    docker network create envoy-tests &>/dev/null

    # Start the volume container
    #
    # This is a dummy container that we use to create volume and keep it
    # accessible while other containers are down.
    docker volume create envoy_workdir &>/dev/null
    docker run -d --name envoy_workdir_1 \
        $WORKDIR_SNIPPET \
        --net=none \
        k8s.gcr.io/pause &>/dev/null
    # TODO(rb): switch back to "${HASHICORP_DOCKER_PROXY}/google/pause" once that is cached

    # pre-build the verify container
    echo "Rebuilding 'bats-verify' image..."
    docker build -t bats-verify -f Dockerfile-bats .

    # if this fails on CircleCI your first thing to try would be to upgrade
    # the machine image to the latest version using this listing:
    #
    # https://circleci.com/docs/2.0/configuration-reference/#available-linux-machine-images
    echo "Checking bats image..."
    docker run --rm -t bats-verify -v

    # pre-build the consul+envoy container
    echo "Rebuilding 'consul-dev-envoy:${ENVOY_VERSION}' image..."
    docker build -t consul-dev-envoy:${ENVOY_VERSION} \
        --build-arg ENVOY_VERSION=${ENVOY_VERSION} \
        -f Dockerfile-consul-envoy .

    # pre-build the test-sds-server container
    echo "Rebuilding 'test-sds-server' image..."
    docker build -t test-sds-server -f Dockerfile-test-sds-server test-sds-server
}

function suite_teardown {
    docker_kill_rm verify-primary verify-secondary verify-alpha

    # this is some hilarious magic
    docker_kill_rm $(grep "^function run_container_" $self_name | \
        sed 's/^function run_container_\(.*\) {/\1/g')

    docker_kill_rm consul-primary consul-secondary consul-ap1 consul-alpha

    if docker network inspect envoy-tests &>/dev/null ; then
        echo -n "Deleting network 'envoy-tests'..."
        docker network rm envoy-tests
        echo "done"
    fi

    workdir_cleanup
}

function run_containers {
 for name in $@ ; do
   run_container $name
 done
}

function run_container {
  docker_kill_rm "$1"
  "run_container_$1"
}

function common_run_container_service {
  local service="$1"
  local CLUSTER="$2"
  local httpPort="$3"
  local grpcPort="$4"

  docker run -d --name $(container_name_prev) \
    -e "FORTIO_NAME=${service}" \
    $(network_snippet $CLUSTER) \
    "${HASHICORP_DOCKER_PROXY}/fortio/fortio" \
    server \
    -http-port ":$httpPort" \
    -grpc-port ":$grpcPort" \
    -redirect-port disabled >/dev/null
}

function run_container_s1 {
  common_run_container_service s1 primary 8080 8079
}

function run_container_s1-ap1 {
  common_run_container_service s1 ap1 8080 8079
}

function run_container_s2 {
  common_run_container_service s2 primary 8181 8179
}
function run_container_s2-v1 {
  common_run_container_service s2-v1 primary 8182 8178
}
function run_container_s2-v2 {
  common_run_container_service s2-v2 primary 8183 8177
}

function run_container_s3 {
  common_run_container_service s3 primary 8282 8279
}
function run_container_s3-v1 {
  common_run_container_service s3-v1 primary 8283 8278
}
function run_container_s3-v2 {
  common_run_container_service s3-v2 primary 8284 8277
}
function run_container_s3-alt {
  common_run_container_service s3-alt primary 8286 8280
}

function run_container_s4 {
  common_run_container_service s4 primary 8382 8281
}

function run_container_s1-secondary {
  common_run_container_service s1-secondary secondary 8080 8079
}

function run_container_s2-secondary {
  common_run_container_service s2-secondary secondary 8181 8179
}

function run_container_s2-ap1 {
  common_run_container_service s2 ap1 8480 8479
}

function run_container_s3-ap1 {
  common_run_container_service s3 ap1 8580 8579
}

function run_container_s1-alpha {
  common_run_container_service s1-alpha alpha 8080 8079
}

function run_container_s2-alpha {
  common_run_container_service s2-alpha alpha 8181 8179
}

function common_run_container_sidecar_proxy {
  local service="$1"
  local CLUSTER="$2"

  # Hot restart breaks since both envoys seem to interact with each other
  # despite separate containers that don't share IPC namespace. Not quite
  # sure how this happens but may be due to unix socket being in some shared
  # location?
  docker run -d --name $(container_name_prev) \
    $WORKDIR_SNIPPET \
    $(network_snippet $CLUSTER) \
    "${HASHICORP_DOCKER_PROXY}/envoyproxy/envoy:v${ENVOY_VERSION}" \
    envoy \
    -c /workdir/${CLUSTER}/envoy/${service}-bootstrap.json \
    -l trace \
    --disable-hot-restart \
    --drain-time-s 1 >/dev/null
}

function run_container_s1-sidecar-proxy {
  common_run_container_sidecar_proxy s1 primary
}

function run_container_s1-ap1-sidecar-proxy {
  common_run_container_sidecar_proxy s1 ap1
}

function run_container_s1-sidecar-proxy-consul-exec {
  docker run -d --name $(container_name) \
    $(network_snippet primary) \
    consul-dev-envoy:${ENVOY_VERSION} \
    consul connect envoy -sidecar-for s1 \
    -envoy-version ${ENVOY_VERSION} \
    -- \
    -l trace >/dev/null
}

function run_container_s2-sidecar-proxy {
  common_run_container_sidecar_proxy s2 primary
}
function run_container_s2-v1-sidecar-proxy {
  common_run_container_sidecar_proxy s2-v1 primary
}
function run_container_s2-v2-sidecar-proxy {
  common_run_container_sidecar_proxy s2-v2 primary
}

function run_container_s3-sidecar-proxy {
  common_run_container_sidecar_proxy s3 primary
}
function run_container_s3-v1-sidecar-proxy {
  common_run_container_sidecar_proxy s3-v1 primary
}
function run_container_s3-v2-sidecar-proxy {
  common_run_container_sidecar_proxy s3-v2 primary
}

function run_container_s3-alt-sidecar-proxy {
  common_run_container_sidecar_proxy s3-alt primary
}

function run_container_s1-sidecar-proxy-secondary {
  common_run_container_sidecar_proxy s1 secondary
}
function run_container_s2-sidecar-proxy-secondary {
  common_run_container_sidecar_proxy s2 secondary
}

function run_container_s2-ap1-sidecar-proxy {
  common_run_container_sidecar_proxy s2 ap1
}

function run_container_s3-ap1-sidecar-proxy {
  common_run_container_sidecar_proxy s3 ap1
}

function run_container_s1-sidecar-proxy-alpha {
  common_run_container_sidecar_proxy s1 alpha
}
function run_container_s2-sidecar-proxy-alpha {
  common_run_container_sidecar_proxy s2 alpha
}

function common_run_container_gateway {
  local name="$1"
  local DC="$2"

  # Hot restart breaks since both envoys seem to interact with each other
  # despite separate containers that don't share IPC namespace. Not quite
  # sure how this happens but may be due to unix socket being in some shared
  # location?
  docker run -d --name $(container_name_prev) \
    $WORKDIR_SNIPPET \
    $(network_snippet $DC) \
    "${HASHICORP_DOCKER_PROXY}/envoyproxy/envoy:v${ENVOY_VERSION}" \
    envoy \
    -c /workdir/${DC}/envoy/${name}-bootstrap.json \
    -l trace \
    --disable-hot-restart \
    --drain-time-s 1 >/dev/null
}

function run_container_gateway-primary {
  common_run_container_gateway mesh-gateway primary
}
function run_container_gateway-secondary {
  common_run_container_gateway mesh-gateway secondary
}
function run_container_gateway-alpha {
  common_run_container_gateway mesh-gateway alpha
}

function run_container_ingress-gateway-primary {
  common_run_container_gateway ingress-gateway primary
}

function run_container_terminating-gateway-primary {
  common_run_container_gateway terminating-gateway primary
}

function run_container_fake-statsd {
  # This magic SYSTEM incantation is needed since Envoy doesn't add newlines and so
  # we need each packet to be passed to echo to add a new line before
  # appending.
  docker run -d --name $(container_name) \
    $WORKDIR_SNIPPET \
    $(network_snippet primary) \
    "${HASHICORP_DOCKER_PROXY}/alpine/socat:1.7.3.4-r1" \
    -u UDP-RECVFROM:8125,fork,reuseaddr \
    SYSTEM:'xargs -0 echo >> /workdir/primary/statsd/statsd.log'
}

function run_container_zipkin {
  docker run -d --name $(container_name) \
    $WORKDIR_SNIPPET \
    $(network_snippet primary) \
    "${HASHICORP_DOCKER_PROXY}/openzipkin/zipkin"
}

function run_container_jaeger {
  docker run -d --name $(container_name) \
    $WORKDIR_SNIPPET \
    $(network_snippet primary) \
    "${HASHICORP_DOCKER_PROXY}/jaegertracing/all-in-one:1.11" \
    --collector.zipkin.http-port=9411
}

function run_container_test-sds-server {
  docker run -d --name $(container_name) \
    $WORKDIR_SNIPPET \
    $(network_snippet primary) \
    "test-sds-server"
}

function container_name {
  echo "envoy_${FUNCNAME[1]/#run_container_/}_1"
}
function container_name_prev {
  echo "envoy_${FUNCNAME[2]/#run_container_/}_1"
}

# This is a debugging tool. Run via './run-tests.sh debug_dump_volumes'
function debug_dump_volumes {
  docker run --rm -it \
    $WORKDIR_SNIPPET \
    -v ./:/cwd \
    --net=none \
    "${HASHICORP_DOCKER_PROXY}/alpine" \
    cp -r /workdir/. /cwd/workdir/
}

function run_container_tcpdump-primary {
    # To use add "tcpdump-primary" to REQUIRED_SERVICES
    common_run_container_tcpdump primary
}
function run_container_tcpdump-secondary {
    # To use add "tcpdump-secondary" to REQUIRED_SERVICES
    common_run_container_tcpdump secondary
}
function run_container_tcpdump-alpha {
    # To use add "tcpdump-alpha" to REQUIRED_SERVICES
    common_run_container_tcpdump alpha
}

function common_run_container_tcpdump {
    local DC="$1"

    # we cant run this in circle but its only here to temporarily enable.

    docker build -t envoy-tcpdump -f Dockerfile-tcpdump .

    docker run -d --name $(container_name_prev) \
        $(network_snippet $DC) \
        -v $(pwd)/workdir/${DC}/envoy/:/data \
        --privileged \
        envoy-tcpdump \
        -v -i any \
        -w "/data/${DC}.pcap"
}

case "${1-}" in
  "")
    echo "command required"
    exit 1 ;;
  *)
    "$@" ;;
esac


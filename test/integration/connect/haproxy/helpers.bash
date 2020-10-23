#!/bin/bash

# retry based on
# https://github.com/fernandoacorreia/azure-docker-registry/blob/master/tools/scripts/create-registry-server
# under MIT license.
function retry {
  local n=1
  local max=$1
  shift
  local delay=$1
  shift

  local errtrace=0
  if grep -q "errtrace" <<< "$SHELLOPTS"
  then
    errtrace=1
    set +E
  fi

  for ((i=1;i<=$max;i++))
  do
    if "$@"
    then
      if test $errtrace -eq 1
      then
        set -E
      fi
      return 0
    else
      echo "Command failed. Attempt $i/$max:"
      sleep $delay
    fi
  done

  if test $errtrace -eq 1
  then
    set -E
  fi
  return 1
}

function retry_default {
  set +E
  ret=0
  retry 5 1 "$@" || ret=1
  set -E
  return $ret
}

function retry_long {
  retry 30 1 "$@"
}

function echored {
  tput setaf 1
  tput bold
  echo $@
  tput sgr0
}

function echogreen {
  tput setaf 2
  tput bold
  echo $@
  tput sgr0
}

function echoyellow {
  tput setaf 3
  tput bold
  echo $@
  tput sgr0
}

function echoblue {
  tput setaf 4
  tput bold
  echo $@
  tput sgr0
}

function is_set {
   # Arguments:
   #   $1 - string value to check its truthiness
   #
   # Return:
   #   0 - is truthy (backwards I know but allows syntax like `if is_set <var>` to work)
   #   1 - is not truthy

   local val=$(tr '[:upper:]' '[:lower:]' <<< "$1")
   case $val in
      1 | t | true | y | yes)
         return 0
         ;;
      *)
         return 1
         ;;
   esac
}

function get_cert {
  local HOSTPORT=$1
  CERT=$(openssl s_client -connect $HOSTPORT -showcerts </dev/null)
  openssl x509 -noout -text <<< "$CERT"
}

function assert_proxy_presents_cert_uri {
  local HOSTPORT=$1
  local SERVICENAME=$2
  local DC=${3:-primary}
  local NS=${4:-default}


  CERT=$(retry_default get_cert $HOSTPORT)

  echo "WANT SERVICE: ${NS}/${SERVICENAME}"
  echo "GOT CERT:"
  echo "$CERT"

  echo "$CERT" | grep -Eo "URI:spiffe://([a-zA-Z0-9-]+).consul/ns/${NS}/dc/${DC}/svc/$SERVICENAME"
}

function assert_dnssan_in_cert {
  local HOSTPORT=$1
  local DNSSAN=$2

  CERT=$(retry_default get_cert $HOSTPORT)

  echo "WANT DNSSAN: ${DNSSAN}"
  echo "GOT CERT:"
  echo "$CERT"

  echo "$CERT" | grep -Eo "DNS:${DNSSAN}"
}

function assert_envoy_version {
  local ADMINPORT=$1
  run retry_default curl -f -s localhost:$ADMINPORT/server_info
  [ "$status" -eq 0 ]
  # Envoy 1.8.0 returns a plain text line like
  # envoy 5d25f466c3410c0dfa735d7d4358beb76b2da507/1.8.0/Clean/DEBUG live 3 3 0
  # Later versions return JSON.
  if (echo $output | grep '^envoy') ; then
    VERSION=$(echo $output | cut -d ' ' -f 2)
  else
    VERSION=$(echo $output | jq -r '.version')
  fi
  echo "Status=$status"
  echo "Output=$output"
  echo "---"
  echo "Got version=$VERSION"
  echo "Want version=$HAPROXY_CONSUL_CONNECT_VERSION"
  echo $VERSION | grep "/$HAPROXY_CONSUL_CONNECT_VERSION/"
}

function extract_haproxy_connect_version {
  local SERVICE=$1
  local version=$(docker-compose exec -T $SERVICE haproxy-connect -version || true)
  echo $version > "case-${CASE_NAME}/version.txt"
}

function assert_haproxy_connect_version {
  local CASE=$1
  ls
  VERSION=$(cat case-${CASE}/version.txt | grep -o '[0-9].*.[0-9]')

  echo "$output" >&3
  # [ "$status" -eq 0 ]

  # VERSION=$(echo $output | grep -o '[0-9].*.[0-9]')
  # echo "Status=$status"
  # echo "Output=$output"
  # echo "---"
  echo "Got version=$VERSION"
  echo "Want version=$HAPROXY_CONSUL_CONNECT_VERSION"
  echo $VERSION | grep "$HAPROXY_CONSUL_CONNECT_VERSION"
}

function get_envoy_listener_filters {
  local HOSTPORT=$1
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  local HAPROXY_CONSUL_CONNECT_VERSION=$(echo $output | jq --raw-output '.configs[0].bootstrap.node.metadata.envoy_version')
  local QUERY=''
  # from 1.13.0 on the config json looks slightly different
  # 1.10.x, 1.11.x, 1.12.x are not affected
  if [[ "$HAPROXY_CONSUL_CONNECT_VERSION" =~ ^1\.1[012]\. ]]; then
    QUERY='.configs[2].dynamic_active_listeners[].listener | "\(.name) \( .filter_chains[0].filters | map(.name) | join(","))"'
  else
    QUERY='.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters | map(.name) | join(","))"'
  fi
  echo "$output" | jq --raw-output "$QUERY"
}

function get_envoy_http_filters {
  local HOSTPORT=$1
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  local HAPROXY_CONSUL_CONNECT_VERSION=$(echo $output | jq --raw-output '.configs[0].bootstrap.node.metadata.envoy_version')
  local QUERY=''
  # from 1.13.0 on the config json looks slightly different
  # 1.10.x, 1.11.x, 1.12.x are not affected
  if [[ "$HAPROXY_CONSUL_CONNECT_VERSION" =~ ^1\.1[012]\. ]]; then
      QUERY='.configs[2].dynamic_active_listeners[].listener | "\(.name) \( .filter_chains[0].filters[] | select(.name == "envoy.http_connection_manager") | .config.http_filters | map(.name) | join(","))"'
  else
      QUERY='.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters[] | select(.name == "envoy.http_connection_manager") | .config.http_filters | map(.name) | join(","))"'
  fi
  echo "$output" | jq --raw-output "$QUERY"
}

function get_envoy_cluster_config {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output "
    .configs[1].dynamic_active_clusters[]
    | select(.cluster.name|startswith(\"${CLUSTER_NAME}\"))
    | .cluster
  "
}

function get_envoy_stats_flush_interval {
  local HOSTPORT=$1
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  #echo "$output" > /workdir/s1_envoy_dump.json
  echo "$output" | jq --raw-output '.configs[0].bootstrap.stats_flush_interval'
}

# snapshot_envoy_admin is meant to be used from a teardown scriptlet from the host.
function snapshot_envoy_admin {
  local HOSTPORT=$1
  local ENVOY_NAME=$2
  local DC=${3:-primary}
  local OUTDIR="${LOG_DIR}/envoy-snapshots/${DC}/${ENVOY_NAME}"

  mkdir -p "${OUTDIR}"
  docker_wget "$DC" "http://${HOSTPORT}/config_dump" -q -O - > "${OUTDIR}/config_dump.json"
  docker_wget "$DC" "http://${HOSTPORT}/clusters?format=json" -q -O - > "${OUTDIR}/clusters.json"
  docker_wget "$DC" "http://${HOSTPORT}/stats" -q -O - > "${OUTDIR}/stats.txt"
}

function reset_envoy_metrics {
  local HOSTPORT=$1
  curl -s -f -XPOST $HOSTPORT/reset_counters
  return $?
}

function get_all_envoy_metrics {
  local HOSTPORT=$1
  curl -s -f $HOSTPORT/stats
  return $?
}

function get_envoy_metrics {
  local HOSTPORT=$1
  local METRICS=$2

  get_all_envoy_metrics $HOSTPORT | grep "$METRICS"
}

function get_upstream_endpoint_in_status_count {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local HEALTH_STATUS=$3
  run retry_default curl -s -f "http://${HOSTPORT}/clusters?format=json"
  [ "$status" -eq 0 ]
  # echo "$output" >&3
  echo "$output" | jq --raw-output "
.cluster_statuses[]
| select(.name|startswith(\"${CLUSTER_NAME}\"))
| [.host_statuses[].health_status.eds_health_status]
| [select(.[] == \"${HEALTH_STATUS}\")]
| length"
}

function assert_upstream_has_endpoints_in_status_once {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local HEALTH_STATUS=$3
  local EXPECT_COUNT=$4

  GOT_COUNT=$(get_upstream_endpoint_in_status_count $HOSTPORT $CLUSTER_NAME $HEALTH_STATUS)

  [ "$GOT_COUNT" -eq $EXPECT_COUNT ]
}

function assert_upstream_has_endpoints_in_status {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local HEALTH_STATUS=$3
  local EXPECT_COUNT=$4
  run retry_long assert_upstream_has_endpoints_in_status_once $HOSTPORT $CLUSTER_NAME $HEALTH_STATUS $EXPECT_COUNT
  [ "$status" -eq 0 ]
}

function assert_envoy_metric {
  set -eEuo pipefail
  local HOSTPORT=$1
  local METRIC=$2
  local EXPECT_COUNT=$3

  METRICS=$(get_envoy_metrics $HOSTPORT "$METRIC")

  if [ -z "${METRICS}" ]
  then
    echo "Metric not found" 1>&2
    return 1
  fi

  GOT_COUNT=$(awk -F: '{print $2}' <<< "$METRICS" | head -n 1 | tr -d ' ')

  if [ -z "$GOT_COUNT" ]
  then
    echo "Couldn't parse metric count" 1>&2
    return 1
  fi

  if [ $EXPECT_COUNT -ne $GOT_COUNT ]
  then
    echo "$METRIC - expected count: $EXPECT_COUNT, actual count: $GOT_COUNT" 1>&2
    return 1
  fi
}

function assert_envoy_metric_at_least {
  set -eEuo pipefail
  local HOSTPORT=$1
  local METRIC=$2
  local EXPECT_COUNT=$3

  METRICS=$(get_envoy_metrics $HOSTPORT "$METRIC")

  if [ -z "${METRICS}" ]
  then
    echo "Metric not found" 1>&2
    return 1
  fi

  GOT_COUNT=$(awk -F: '{print $2}' <<< "$METRICS" | head -n 1 | tr -d ' ')

  if [ -z "$GOT_COUNT" ]
  then
    echo "Couldn't parse metric count" 1>&2
    return 1
  fi

  if [ $EXPECT_COUNT -gt $GOT_COUNT ]
  then
    echo "$METRIC - expected >= count: $EXPECT_COUNT, actual count: $GOT_COUNT" 1>&2
    return 1
  fi
}

function assert_envoy_aggregate_metric_at_least {
  set -eEuo pipefail
  local HOSTPORT=$1
  local METRIC=$2
  local EXPECT_COUNT=$3

  METRICS=$(get_envoy_metrics $HOSTPORT "$METRIC")

  if [ -z "${METRICS}" ]
  then
    echo "Metric not found" 1>&2
    return 1
  fi

  GOT_COUNT=$(awk '{ sum += $2 } END { print sum }' <<< "$METRICS")

  if [ -z "$GOT_COUNT" ]
  then
    echo "Couldn't parse metric count" 1>&2
    return 1
  fi

  if [ $EXPECT_COUNT -gt $GOT_COUNT ]
  then
    echo "$METRIC - expected >= count: $EXPECT_COUNT, actual count: $GOT_COUNT" 1>&2
    return 1
  fi
}

function get_healthy_service_count {
  local SERVICE_NAME=$1
  local DC=$2
  local NS=$3

  run retry_default curl -s -f ${HEADERS} "127.0.0.1:8500/v1/health/connect/${SERVICE_NAME}?dc=${DC}&passing&ns=${NS}"
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '. | length'
}

function assert_alive_wan_member_count {
  local EXPECT_COUNT=$1
  run retry_long assert_alive_wan_member_count_once $EXPECT_COUNT
  [ "$status" -eq 0 ]
}

function assert_alive_wan_member_count_once {
  local EXPECT_COUNT=$1

  GOT_COUNT=$(get_alive_wan_member_count)

  [ "$GOT_COUNT" -eq "$EXPECT_COUNT" ]
}

function get_alive_wan_member_count {
  run retry_default curl -sL -f "127.0.0.1:8500/v1/agent/members?wan=1"
  [ "$status" -eq 0 ]
  # echo "$output" >&3
  echo "$output" | jq '.[] | select(.Status == 1) | .Name' | wc -l
}

function assert_service_has_healthy_instances_once {
  local SERVICE_NAME=$1
  local EXPECT_COUNT=$2
  local DC=${3:-primary}
  local NS=$4

  GOT_COUNT=$(get_healthy_service_count "$SERVICE_NAME" "$DC" "$NS")

  [ "$GOT_COUNT" -eq $EXPECT_COUNT ]
}

function assert_service_has_healthy_instances {
  local SERVICE_NAME=$1
  local EXPECT_COUNT=$2
  local DC=${3:-primary}
  local NS=$4

  run retry_long assert_service_has_healthy_instances_once "$SERVICE_NAME" "$EXPECT_COUNT" "$DC" "$NS"
  [ "$status" -eq 0 ]
}

function check_intention {
  local SOURCE=$1
  local DESTINATION=$2

  curl -s -f "localhost:8500/v1/connect/intentions/check?source=${SOURCE}&destination=${DESTINATION}" | jq ".Allowed"
}

function assert_intention_allowed {
  local SOURCE=$1
  local DESTINATION=$2

  [ "$(check_intention "${SOURCE}" "${DESTINATION}")" == "true" ]
}

function assert_intention_denied {
  local SOURCE=$1
  local DESTINATION=$2

  [ "$(check_intention "${SOURCE}" "${DESTINATION}")" == "false" ]
}

function docker_consul {
  local DC=$1
  shift 1
  docker run -i --rm --network container:haproxy_consul-${DC}_1 consul-dev "$@"
}

function docker_wget {
  local DC=$1
  shift 1
  docker run --rm --network container:haproxy_consul-${DC}_1 alpine:3.9 wget "$@"
}

function docker_curl {
  local DC=$1
  shift 1
  docker run --rm --network container:haproxy_consul-${DC}_1 --entrypoint curl consul-dev "$@"
}

function docker_exec {
  if ! docker exec -i "$@"
  then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul_exec {
  local DC=$1
  shift 1
  docker_exec haproxy_consul-${DC}_1 "$@"
}

function get_envoy_pid {
  local BOOTSTRAP_NAME=$1
  local DC=${2:-primary}
  run ps aux
  [ "$status" == 0 ]
  echo "$output" 1>&2
  PID="$(echo "$output" | grep "envoy -c /workdir/$DC/envoy/${BOOTSTRAP_NAME}-bootstrap.json" | awk '{print $1}')"
  [ -n "$PID" ]

  echo "$PID"
}

function kill_envoy {
  local BOOTSTRAP_NAME=$1
  local DC=${2:-primary}

  PID="$(get_envoy_pid $BOOTSTRAP_NAME "$DC")"
  echo "PID = $PID"

  kill -TERM $PID
}

function must_match_in_statsd_logs {
  local DC=${2:-primary}

  run cat /workdir/${DC}/statsd/statsd.log
  echo "$output"
  COUNT=$( echo "$output" | grep -Ec $1 )

  echo "COUNT of '$1' matches: $COUNT"

  [ "$status" == 0 ]
  [ "$COUNT" -gt "0" ]
}

function must_match_in_prometheus_response {
  run curl -f -s $1/metrics
  COUNT=$( echo "$output" | grep -Ec $2 )

  echo "OUTPUT head -n 10"
  echo "$output" | head -n 10
  echo "COUNT of '$2' matches: $COUNT"

  [ "$status" == 0 ]
  [ "$COUNT" -gt "0" ]
}

function must_match_in_stats_proxy_response {
  run curl -f -s $1/$2
  COUNT=$( echo "$output" | grep -Ec $3 )

  echo "OUTPUT head -n 10"
  echo "$output" | head -n 10
  echo "COUNT of '$3' matches: $COUNT"

  [ "$status" == 0 ]
  [ "$COUNT" -gt "0" ]
}

# must_fail_tcp_connection checks that a request made through an upstream fails,
# probably due to authz being denied if all other tests passed already. Although
# we are using curl, this only works as expected for TCP upstreams as we are
# checking TCP-level errors. HTTP upstreams will return a valid 503 generated by
# Envoy rather than a connection-level error.
function must_fail_tcp_connection {
  # Attempt to curl through upstream
  run curl -s -v -f -d hello $1

  echo "OUTPUT $output"

  # Should fail during handshake and return "got nothing" error
  [ "$status" == "52" ]

  # Verbose output should enclude empty reply
  echo "$output" | grep 'Empty reply from server'
}

# must_fail_http_connection see must_fail_tcp_connection but this expects Envoy
# to generate a 503 response since the upstreams have refused connection.
function must_fail_http_connection {
  # Attempt to curl through upstream
  run curl -s -i -d hello "$1"

  echo "OUTPUT $output"

  local expect_response="${2:-503 Service Unavailable}"
  # Should fail request with 503
  echo "$output" | grep "${expect_response}"
}

# must_fail_http_connection_with_502 see must_fail_tcp_connection but this expects HAProxy
# to generate a 502 response since the upstreams have refused connection.
function must_fail_http_connection_with_502 {
  # Attempt to curl through upstream
  run curl -s -i -d hello "$1"

  echo "OUTPUT $output"

  local expect_response="${2:-502 Bad Gateway}"
  # Should fail request with 502
  echo "$output" | grep "${expect_response}"
}

# must_pass_http_request allows you to craft a specific http request to assert
# that envoy will NOT reject the request. Primarily of use for testing L7
# intentions.
function must_pass_http_request {
  local METHOD=$1
  local URL=$2
  local DEBUG_HEADER_VALUE="${3:-""}"

  local extra_args
  if [[ -n "${DEBUG_HEADER_VALUE}" ]]; then
    extra_args="-H x-test-debug:${DEBUG_HEADER_VALUE}"
  fi
  case "$METHOD" in
    GET)
      ;;
    DELETE)
      extra_args="$extra_args -X${METHOD}"
      ;;
    PUT|POST)
      extra_args="$extra_args -d'{}' -X${METHOD}"
      ;;
    *)
      return 1
      ;;
  esac

  run retry_default curl -v -s -f $extra_args "$URL"
  [ "$status" == 0 ]
}

# must_fail_http_request allows you to craft a specific http request to assert
# that envoy will reject the request. Primarily of use for testing L7
# intentions.
function must_fail_http_request {
  local METHOD=$1
  local URL=$2
  local DEBUG_HEADER_VALUE="${3:-""}"

  local extra_args
  if [[ -n "${DEBUG_HEADER_VALUE}" ]]; then
      extra_args="-H x-test-debug:${DEBUG_HEADER_VALUE}"
  fi
  case "$METHOD" in
    HEAD)
      extra_args="$extra_args -I"
      ;;
    GET)
      ;;
    DELETE)
      extra_args="$extra_args -X${METHOD}"
      ;;
    PUT|POST)
      extra_args="$extra_args -d'{}' -X${METHOD}"
      ;;
    *)
      return 1
      ;;
  esac

  # Attempt to curl through upstream
  run retry_default curl -s -i $extra_args "$URL"

  echo "OUTPUT $output"

  echo "$output" | grep "403 Forbidden"
}

function gen_envoy_bootstrap {
  SERVICE=$1
  ADMIN_PORT=$2
  DC=${3:-primary}
  IS_GW=${4:-0}
  EXTRA_ENVOY_BS_ARGS="${5-}"

  PROXY_ID="$SERVICE"
  if ! is_set "$IS_GW"
  then
    PROXY_ID="$SERVICE-sidecar-proxy"
  fi

  if output=$(docker_consul "$DC" connect envoy -bootstrap \
    -proxy-id $PROXY_ID \
    -envoy-version "$HAPROXY_CONSUL_CONNECT_VERSION" \
    -admin-bind 0.0.0.0:$ADMIN_PORT ${EXTRA_ENVOY_BS_ARGS} 2>&1); then

    # All OK, write config to file
    echo "$output" > workdir/${DC}/envoy/$SERVICE-bootstrap.json
  else
    status=$?
    # Command failed, instead of swallowing error (printed on stdout by docker
    # it seems) by writing it to file, echo it
    echo "$output"
    return $status
  fi
}

function read_config_entry {
  local KIND=$1
  local NAME=$2
  local DC=${3:-primary}

  docker_consul "$DC" config read -kind $KIND -name $NAME
}

function wait_for_config_entry {
  retry_default read_config_entry "$@" >/dev/null
}

function delete_config_entry {
  local KIND=$1
  local NAME=$2
  retry_default curl -sL -XDELETE "http://127.0.0.1:8500/v1/config/${KIND}/${NAME}"
}

function list_intentions {
  curl -s -f "http://localhost:8500/v1/connect/intentions"
}

function get_intention_target_name {
  awk -F / '{ if ( NF == 1 ) { print $0 } else { print $2 }}'
}

function get_intention_target_namespace {
  awk -F / '{ if ( NF != 1 ) { print $1 } }'
}

function get_intention_by_targets {
  local SOURCE=$1
  local DESTINATION=$2

  local SOURCE_NS=$(get_intention_target_namespace <<< "${SOURCE}")
  local SOURCE_NAME=$(get_intention_target_name <<< "${SOURCE}")
  local DESTINATION_NS=$(get_intention_target_namespace <<< "${DESTINATION}")
  local DESTINATION_NAME=$(get_intention_target_name <<< "${DESTINATION}")

  existing=$(list_intentions | jq ".[] | select(.SourceNS == \"$SOURCE_NS\" and .SourceName == \"$SOURCE_NAME\" and .DestinationNS == \"$DESTINATION_NS\" and .DestinationName == \"$DESTINATION_NAME\")")
  if test -z "$existing"
  then
    return 1
  fi
  echo "$existing"
  return 0
}

function update_intention {
  local SOURCE=$1
  local DESTINATION=$2
  local ACTION=$3

  intention=$(get_intention_by_targets "${SOURCE}" "${DESTINATION}")
  if test $? -ne 0
  then
    return 1
  fi

  id=$(jq -r .ID <<< "${intention}")
  updated=$(jq ".Action = \"$ACTION\"" <<< "${intention}")

  curl -s -X PUT "http://localhost:8500/v1/connect/intentions/${id}" -d "${updated}"
  return $?
}

function get_ca_root {
  curl -s -f "http://localhost:8500/v1/connect/ca/roots" | jq -r ".Roots[0].RootCert"
}

function wait_for_agent_service_register {
  local SERVICE_ID=$1
  local DC=${2:-primary}
  retry_default docker_curl "$DC" -sLf "http://127.0.0.1:8500/v1/agent/service/${SERVICE_ID}" >/dev/null
}

function wait_for_catalog_service_register {
  local SERVICE_ID=$1
  local DC=$2
  local MAX_RETRY=${3:-15}

  local counter=0

  while [ "$(docker_curl "$DC" -sLf "http://127.0.0.1:8500/v1/catalog/service/${SERVICE_ID}")" = "[]" ]; do
    echo "Waiting $SERVICE_ID service to be registered in $DC datacenter..."
    sleep 1

    if [ $MAX_RETRY -eq $counter ]; then
      echo "Registering service ${SERVICE_ID} timeout(${MAX_RETRY}s) reached."
      return
    fi
    counter=$((counter+1))
  done
  echo "$SERVICE_ID service registered in $DC datacenter."
}

function wait_for_health_check_passing_state {
  local SERVICE_ID=$1
  local DC=$2
  local MAX_RETRY=${3:-15}

  local counter=0

  while [ "$(docker_curl "$DC" -sLf "http://localhost:8500/v1/health/connect/${SERVICE_ID}?passing")" = "[]" ]; do
    echo "Waiting $SERVICE_ID service health check..."
    sleep 1

    if [ $MAX_RETRY -eq $counter ]; then
      echo "Health check for service ${SERVICE_ID} timeout(${MAX_RETRY}s) reached."
      return
    fi
    counter=$((counter+1))
  done
  echo "$SERVICE_ID service health status: passing."
}

function set_ttl_check_state {
  local CHECK_ID=$1
  local CHECK_STATE=$2
  local DC=${3:-primary}

  case "$CHECK_STATE" in
    pass)
      ;;
    warn)
      ;;
    fail)
      ;;
    *)
      echo "invalid ttl check state '${CHECK_STATE}'" >&2
      return 1
  esac

  retry_default docker_curl "${DC}" -sL -XPUT "http://localhost:8500/v1/agent/check/warn/${CHECK_ID}"
}

function get_upstream_fortio_name {
  local HOST=$1
  local PORT=$2
  local PREFIX=$3
  local DEBUG_HEADER_VALUE="${4:-""}"
  local extra_args
  if [[ -n "${DEBUG_HEADER_VALUE}" ]]; then
      extra_args="-H x-test-debug:${DEBUG_HEADER_VALUE}"
  fi
  run retry_default curl -v -s -f -H"Host: ${HOST}" $extra_args \
      "localhost:${PORT}${PREFIX}/debug?env=dump"
  [ "$status" == 0 ]
  echo "$output" | grep -E "^FORTIO_NAME="
}

function assert_expected_fortio_name {
  local EXPECT_NAME=$1
  local HOST=${2:-"localhost"}
  local PORT=${3:-5000}
  local URL_PREFIX=${4:-""}
  local DEBUG_HEADER_VALUE="${5:-""}"

  GOT=$(get_upstream_fortio_name ${HOST} ${PORT} "${URL_PREFIX}" "${DEBUG_HEADER_VALUE}")

  if [ "$GOT" != "FORTIO_NAME=${EXPECT_NAME}" ]; then
    echo "expected name: $EXPECT_NAME, actual name: $GOT" 1>&2
    return 1
  fi
}

function assert_expected_fortio_name_pattern {
  local EXPECT_NAME_PATTERN=$1
  local HOST=${2:-"localhost"}
  local PORT=${3:-5000}
  local URL_PREFIX=${4:-""}
  local DEBUG_HEADER_VALUE="${5:-""}"

  GOT=$(get_upstream_fortio_name ${HOST} ${PORT} "${URL_PREFIX}" "${DEBUG_HEADER_VALUE}")

  if [[ "$GOT" =~ $EXPECT_NAME_PATTERN ]]; then
      :
  else
    echo "expected name pattern: $EXPECT_NAME_PATTERN, actual name: $GOT" 1>&2
    return 1
  fi
}

#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


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
  if grep -q "errtrace" <<<"$SHELLOPTS"; then
    errtrace=1
    set +E
  fi

  for ((i = 1; i <= $max; i++)); do
    if "$@"; then
      if test $errtrace -eq 1; then
        set -E
      fi
      return 0
    else
      echo "Command failed. Attempt $i/$max:"
      sleep $delay
    fi
  done

  if test $errtrace -eq 1; then
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

# assert_upstream_message asserts both the returned code
# and message from upstream service
function assert_upstream_message {
  local HOSTPORT=$1
  run curl -s -d hello localhost:$HOSTPORT

  if [ "$status" -ne 0 ]; then
    echo "Command failed"
    return 1
  fi

  if (echo $output | grep 'hello'); then
    return 0
  fi

  echo "expected message not found in $output"
  return 1
}

function is_set {
  # Arguments:
  #   $1 - string value to check its truthiness
  #
  # Return:
  #   0 - is truthy (backwards I know but allows syntax like `if is_set <var>` to work)
  #   1 - is not truthy

  local val=$(tr '[:upper:]' '[:lower:]' <<<"$1")
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
  local SERVER_NAME=$2
  local CA_FILE=$3
  local SNI_FLAG=""
  if [ -n "$SERVER_NAME" ]; then
    SNI_FLAG="-servername $SERVER_NAME"
  fi
  CERT=$(openssl s_client -connect $HOSTPORT $SNI_FLAG -showcerts </dev/null)
  openssl x509 -noout -text <<<"$CERT"
}

function assert_proxy_presents_cert_uri {
  local HOSTPORT=$1
  local SERVICENAME=$2
  local DC=${3:-primary}
  local NS=${4:-default}
  local PARTITION=${5:default}

  CERT=$(retry_default get_cert $HOSTPORT)

  echo "WANT SERVICE: ${PARTITION}/${NS}/${SERVICENAME}"
  echo "GOT CERT:"
  echo "$CERT"

  if [[ -z $PARTITION ]] || [[ $PARTITION = "default" ]]; then
    echo "$CERT" | grep -Eo "URI:spiffe://([a-zA-Z0-9-]+).consul/ns/${NS}/dc/${DC}/svc/$SERVICENAME"
  else
    echo "$CERT" | grep -Eo "URI:spiffe://([a-zA-Z0-9-]+).consul/ap/${PARTITION}/ns/${NS}/dc/${DC}/svc/$SERVICENAME"
  fi
}

function assert_dnssan_in_cert {
  local HOSTPORT=$1
  local DNSSAN=$2
  local SERVER_NAME=${3:-$DNSSAN}

  CERT=$(retry_default get_cert $HOSTPORT $SERVER_NAME)

  echo "WANT DNSSAN: ${DNSSAN} (SNI: ${SERVER_NAME})"
  echo "GOT CERT:"
  echo "$CERT"

  echo "$CERT" | grep -Eo "DNS:${DNSSAN}"
}

function assert_cert_signed_by_ca {
  local CA_FILE=$1
  local HOSTPORT=$2
  local DNSSAN=$3
  local SERVER_NAME=${4:-$DNSSAN}
  local SNI_FLAG=""
  if [ -n "$SERVER_NAME" ]; then
    SNI_FLAG="-servername $SERVER_NAME"
  fi
  CERT=$(openssl s_client -connect $HOSTPORT $SNI_FLAG -CAfile $CA_FILE -showcerts </dev/null)

  echo "GOT CERT:"
  echo "$CERT"

  echo "$CERT" | grep 'Verify return code: 0 (ok)'
}

function assert_cert_has_cn {
  local HOSTPORT=$1
  local CN=$2
  local SERVER_NAME=${3:-$CN}

  CERT=$(openssl s_client -connect $HOSTPORT -servername $SERVER_NAME -showcerts </dev/null 2>/dev/null)

  echo "WANT CN: ${CN} (SNI: ${SERVER_NAME})"
  echo "GOT CERT:"
  echo "$CERT"

  echo "$CERT" | grep "CN = ${CN}"
}

function assert_envoy_version {
  local ADMINPORT=$1
  run retry_default curl -f -s localhost:$ADMINPORT/server_info
  [ "$status" -eq 0 ]
  # Envoy 1.8.0 returns a plain text line like
  # envoy 5d25f466c3410c0dfa735d7d4358beb76b2da507/1.8.0/Clean/DEBUG live 3 3 0
  # Later versions return JSON.
  if (echo $output | grep '^envoy'); then
    VERSION=$(echo $output | cut -d ' ' -f 2)
  else
    VERSION=$(echo $output | jq -r '.version')
  fi
  echo "Status=$status"
  echo "Output=$output"
  echo "---"
  echo "Got version=$VERSION"
  echo "Want version=$ENVOY_VERSION"

  # 1.20.2, 1.19.3 and 1.18.6 are special snowflakes in that the version for
  # the release is reported with a '-dev' suffix (eg 1.20.2-dev).
  if [ "$ENVOY_VERSION" = "1.20.2" ]; then
    ENVOY_VERSION="1.20.2-dev"
  elif [ "$ENVOY_VERSION" = "1.19.3" ]; then
    ENVOY_VERSION="1.19.3-dev"
  elif [ "$ENVOY_VERSION" = "1.18.6" ]; then
    ENVOY_VERSION="1.18.6-dev"
  fi

  echo $VERSION | grep "/$ENVOY_VERSION/"
}

function assert_envoy_expose_checks_listener_count {
  local HOSTPORT=$1
  local EXPECT_PATH=$2

  # scrape this once
  BODY=$(get_envoy_expose_checks_listener_once $HOSTPORT)
  echo "BODY = $BODY"

  CHAINS=$(echo "$BODY" | jq '.active_state.listener.filter_chains | length')
  echo "CHAINS = $CHAINS (expect 1)"
  [ "${CHAINS:-0}" -eq 1 ]

  RANGES=$(echo "$BODY" | jq '.active_state.listener.filter_chains[0].filter_chain_match.source_prefix_ranges | length')
  echo "RANGES = $RANGES (expect 3)"
  # note: if IPv6 is not supported in the kernel per
  # agent/xds:kernelSupportsIPv6() then this will only be 2
  [ "${RANGES:-0}" -eq 3 ]

  HCM=$(echo "$BODY" | jq '.active_state.listener.filter_chains[0].filters[0]')
  HCM_NAME=$(echo "$HCM" | jq -r '.name')
  HCM_PATH=$(echo "$HCM" | jq -r '.typed_config.route_config.virtual_hosts[0].routes[0].match.path')
  echo "HCM = $HCM"
  [ "${HCM_NAME:-}" == "envoy.filters.network.http_connection_manager" ]
  [ "${HCM_PATH:-}" == "${EXPECT_PATH}" ]
}

function get_envoy_expose_checks_listener_once {
  local HOSTPORT=$1
  run curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[] | select(.["@type"] == "type.googleapis.com/envoy.admin.v3.ListenersConfigDump") | .dynamic_listeners[] | select(.name | startswith("exposed_path_"))'
}

function get_envoy_public_listener_once {
  local HOSTPORT=$1
  run curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[] | select(.["@type"] == "type.googleapis.com/envoy.admin.v3.ListenersConfigDump") | .dynamic_listeners[] | select(.name | startswith("public_listener:"))'
}

function assert_envoy_http_rbac_policy_count {
  local HOSTPORT=$1
  local EXPECT_COUNT=$2

  GOT_COUNT=$(get_envoy_http_rbac_once $HOSTPORT | jq '.rules.policies | length')
  echo "GOT_COUNT = $GOT_COUNT"
  [ "${GOT_COUNT:-0}" -eq $EXPECT_COUNT ]
}

function get_envoy_http_rbac_once {
  local HOSTPORT=$1
  run curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[2].dynamic_listeners[].active_state.listener.filter_chains[0].filters[0].typed_config.http_filters[] | select(.name == "envoy.filters.http.rbac") | .typed_config'
}

function assert_envoy_network_rbac_policy_count {
  local HOSTPORT=$1
  local EXPECT_COUNT=$2

  GOT_COUNT=$(get_envoy_network_rbac_once $HOSTPORT | jq '.rules.policies | length')
  echo "GOT_COUNT = $GOT_COUNT"
  [ "${GOT_COUNT:-0}" -eq $EXPECT_COUNT ]
}

function get_envoy_network_rbac_once {
  local HOSTPORT=$1
  run curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[2].dynamic_listeners[].active_state.listener.filter_chains[0].filters[] | select(.name == "envoy.filters.network.rbac") | .typed_config'
}

function get_envoy_http_filter {
  local HOSTPORT=$1
  local FILTER_NAME=$2
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output ".configs[2].dynamic_listeners[] | .active_state.listener.filter_chains[].filters[] | select(.name == \"envoy.filters.network.http_connection_manager\") | .typed_config.http_filters[] | select(.name == \"${FILTER_NAME}\")"
}

function get_envoy_listener_filters {
  local HOSTPORT=$1
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters | map(.name) | join(","))"'
}

function get_envoy_http_filters {
  local HOSTPORT=$1
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[2].dynamic_listeners[].active_state.listener | "\(.name) \( .filter_chains[0].filters[] | select(.name == "envoy.filters.network.http_connection_manager") | .typed_config.http_filters | map(.name) | join(","))"'
}

function get_envoy_dynamic_cluster_once {
  local HOSTPORT=$1
  local NAME_PREFIX=$2
  run curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output ".configs[] | select (.[\"@type\"] == \"type.googleapis.com/envoy.admin.v3.ClustersConfigDump\") | .dynamic_active_clusters[] | select(.cluster.name | startswith(\"${NAME_PREFIX}\"))"
}

function assert_envoy_dynamic_cluster_exists_once {
  local HOSTPORT=$1
  local NAME_PREFIX=$2
  local EXPECT_SNI=$3
  BODY="$(get_envoy_dynamic_cluster_once $HOSTPORT $NAME_PREFIX)"
  [ -n "$BODY" ]

  SNI="$(echo "$BODY" | jq --raw-output ".cluster.transport_socket.typed_config.sni | select(. | startswith(\"${EXPECT_SNI}\"))")"
  [ -n "$SNI" ]
}

function assert_envoy_dynamic_cluster_exists {
  local HOSTPORT=$1
  local NAME_PREFIX=$2
  local EXPECT_SNI=$3
  run retry_long assert_envoy_dynamic_cluster_exists_once $HOSTPORT $NAME_PREFIX $EXPECT_SNI
  [ "$status" -eq 0 ]
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
  docker_wget "$DC" "http://${HOSTPORT}/config_dump" -q -O - >"${OUTDIR}/config_dump.json"
  docker_wget "$DC" "http://${HOSTPORT}/clusters?format=json" -q -O - >"${OUTDIR}/clusters.json"
  docker_wget "$DC" "http://${HOSTPORT}/stats" -q -O - >"${OUTDIR}/stats.txt"
  docker_wget "$DC" "http://${HOSTPORT}/stats/prometheus" -q -O - >"${OUTDIR}/stats_prometheus.txt"
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

function get_upstream_endpoint {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  run curl -s -f "http://${HOSTPORT}/clusters?format=json"
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output "
.cluster_statuses[]
| select(.name|startswith(\"${CLUSTER_NAME}\"))"
}

function get_upstream_endpoint_port {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local PORT_VALUE=$3
  run curl -s -f "http://${HOSTPORT}/clusters?format=json"
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output "
.cluster_statuses[]
| select(.name|startswith(\"${CLUSTER_NAME}\"))
| [.host_statuses[].address.socket_address.port_value]
| [select(.[] == ${PORT_VALUE})]
| length"
}

function assert_upstream_has_endpoint_port_once {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local PORT_VALUE=$3

  GOT_COUNT=$(get_upstream_endpoint_port $HOSTPORT $CLUSTER_NAME $PORT_VALUE)

  [ "$GOT_COUNT" -eq 1 ]
}

function assert_upstream_has_endpoint_port {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local PORT_VALUE=$3

  run retry_long assert_upstream_has_endpoint_port_once $HOSTPORT $CLUSTER_NAME $PORT_VALUE
  [ "$status" -eq 0 ]
}

function get_upstream_endpoint_in_status_count {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local HEALTH_STATUS=$3
  run curl -s -f "http://${HOSTPORT}/clusters?format=json"
  [ "$status" -eq 0 ]
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

  echo "GOT: $GOT_COUNT"
  [ "$GOT_COUNT" -eq $EXPECT_COUNT ]
}

function assert_upstream_missing_once {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  
  run get_upstream_endpoint $HOSTPORT $CLUSTER_NAME
  [ "$status" -eq 0 ]
  echo "$output"
  [ "" == "$output" ]
}

function assert_upstream_missing {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  run retry_long assert_upstream_missing_once $HOSTPORT $CLUSTER_NAME
  echo "OUTPUT: $output $status"

  [ "$status" -eq 0 ]
}

function assert_upstream_has_endpoints_in_status {
  local HOSTPORT=$1
  local CLUSTER_NAME=$2
  local HEALTH_STATUS=$3
  local EXPECT_COUNT=$4
  run retry_long assert_upstream_has_endpoints_in_status_once $HOSTPORT $CLUSTER_NAME $HEALTH_STATUS $EXPECT_COUNT
  echo "$output"

  [ "$status" -eq 0 ]
}

function assert_envoy_metric {
  set -eEuo pipefail
  local HOSTPORT=$1
  local METRIC=$2
  local EXPECT_COUNT=$3

  METRICS=$(get_envoy_metrics $HOSTPORT "$METRIC")

  if [ -z "${METRICS}" ]; then
    echo "Metric not found" 1>&2
    return 1
  fi

  GOT_COUNT=$(awk -F: '{print $2}' <<<"$METRICS" | head -n 1 | tr -d ' ')

  if [ -z "$GOT_COUNT" ]; then
    echo "Couldn't parse metric count" 1>&2
    return 1
  fi

  if [ $EXPECT_COUNT -ne $GOT_COUNT ]; then
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

  if [ -z "${METRICS}" ]; then
    echo "Metric not found" 1>&2
    return 1
  fi

  GOT_COUNT=$(awk -F: '{print $2}' <<<"$METRICS" | head -n 1 | tr -d ' ')

  if [ -z "$GOT_COUNT" ]; then
    echo "Couldn't parse metric count" 1>&2
    return 1
  fi

  if [ $EXPECT_COUNT -gt $GOT_COUNT ]; then
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

  if [ -z "${METRICS}" ]; then
    echo "Metric not found" 1>&2
    return 1
  fi

  GOT_COUNT=$(awk '{ sum += $2 } END { print sum }' <<<"$METRICS")

  if [ -z "$GOT_COUNT" ]; then
    echo "Couldn't parse metric count" 1>&2
    return 1
  fi

  if [ $EXPECT_COUNT -gt $GOT_COUNT ]; then
    echo "$METRIC - expected >= count: $EXPECT_COUNT, actual count: $GOT_COUNT" 1>&2
    return 1
  fi
}

function get_healthy_service_count {
  local SERVICE_NAME=$1
  local DC=$2
  local NS=$3
  local AP=$4
  local PEER_NAME=$5

  run curl -s -f ${HEADERS} "consul-${DC}-client:8500/v1/health/connect/${SERVICE_NAME}?passing&ns=${NS}&partition=${AP}&peer=${PEER_NAME}"

  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '. | length'
}

function assert_alive_wan_member_count {
  local DC=$1
  local EXPECT_COUNT=$2
  run retry_long assert_alive_wan_member_count_once $DC $EXPECT_COUNT
  [ "$status" -eq 0 ]
}

function assert_alive_wan_member_count_once {
  local DC=$1
  local EXPECT_COUNT=$2

  GOT_COUNT=$(get_alive_wan_member_count $DC)

  [ "$GOT_COUNT" -eq "$EXPECT_COUNT" ]
}

function get_alive_wan_member_count {
  local DC=$1
  run retry_default curl -sL -f "consul-${DC}-server:8500/v1/agent/members?wan=1"
  [ "$status" -eq 0 ]
  # echo "$output" >&3
  echo "$output" | jq '.[] | select(.Status == 1) | .Name' | wc -l
}

function assert_service_has_healthy_instances_once {
  local SERVICE_NAME=$1
  local EXPECT_COUNT=$2
  local DC=${3:-primary}
  local NS=${4:-}
  local AP=${5:-}
  local PEER_NAME=${6:-}

  GOT_COUNT=$(get_healthy_service_count "$SERVICE_NAME" "$DC" "$NS" "$AP" "$PEER_NAME")

  [ "$GOT_COUNT" -eq $EXPECT_COUNT ]
}

function assert_service_has_healthy_instances {
  local SERVICE_NAME=$1
  local EXPECT_COUNT=$2
  local DC=${3:-primary}
  local NS=${4:-}
  local AP=${5:-}
  local PEER_NAME=${6:-}

  run retry_long assert_service_has_healthy_instances_once "$SERVICE_NAME" "$EXPECT_COUNT" "$DC" "$NS" "$AP" "$PEER_NAME"
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

  run check_intention "${SOURCE}" "${DESTINATION}"
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

function assert_intention_denied {
  local SOURCE=$1
  local DESTINATION=$2

  run check_intention "${SOURCE}" "${DESTINATION}"
  [ "$status" -eq 0 ]
  [ "$output" = "false" ]
}

function docker_consul {
  local DC=$1
  shift 1
  docker run -i --rm --network container:envoy_consul-${DC}_1 consul:local "$@"
}

function docker_consul_for_proxy_bootstrap {
  local DC=$1
  shift 1

  docker run -i --rm --network container:envoy_consul-${DC}_1 consul:local "$@" 2>/dev/null
}

function docker_wget {
  local DC=$1
  shift 1
  docker run --rm --network container:envoy_consul-${DC}_1 docker.mirror.hashicorp.services/alpine:3.17 wget "$@"
}

function docker_curl {
  local DC=$1
  shift 1
  docker run --rm --network container:envoy_consul-${DC}_1 --entrypoint curl consul:local "$@"
}

function docker_exec {
  if ! docker exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul_exec {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}

function kill_envoy {
  local BOOTSTRAP_NAME=$1
  local DC=${2:-primary}

  pkill -TERM -f "envoy -c /workdir/$DC/envoy/${BOOTSTRAP_NAME}-bootstrap.json"
}

function must_match_in_statsd_logs {
  local DC=${2:-primary}

  run cat /workdir/${DC}/statsd/statsd.log
  echo "$output"
  COUNT=$(echo "$output" | grep -Ec $1)

  echo "COUNT of '$1' matches: $COUNT"

  [ "$status" == 0 ]
  [ "$COUNT" -gt "0" ]
}

function must_match_in_prometheus_response {
  run curl -f -s $1/metrics
  COUNT=$(echo "$output" | grep -Ec $2)

  echo "OUTPUT head -n 10"
  echo "$output" | head -n 10
  echo "COUNT of '$2' matches: $COUNT"

  [ "$status" == 0 ]
  [ "$COUNT" -gt "0" ]
}

function must_match_in_stats_proxy_response {
  run curl -f -s $1/$2
  COUNT=$(echo "$output" | grep -Ec $3)

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
  run curl --no-keepalive -s -v -f -d hello $1

  echo "OUTPUT $output"

  # Should fail during handshake and return "got nothing" error
  [ "$status" == "52" ]

  # Verbose output should enclude empty reply
  echo "$output" | grep 'Empty reply from server'
}

function must_pass_tcp_connection {
  run curl --no-keepalive -s -f -d hello $1

  echo "OUTPUT $output"

  [ "$status" == "0" ]
  [[ "$output" == *"hello"* ]]
}

# must_fail_http_connection see must_fail_tcp_connection but this expects Envoy
# to generate a 503 response since the upstreams have refused connection.
function must_fail_http_connection {
  # Attempt to curl through upstream
  run curl --no-keepalive -s -i -d hello "$1"

  echo "OUTPUT $output"

  [ "$status" == "0" ]

  local expect_response="${2:-403 Forbidden}"
  # Should fail request with 503
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
  GET) ;;

  DELETE)
    extra_args="$extra_args -X${METHOD}"
    ;;
  PUT | POST)
    extra_args="$extra_args -d'{}' -X${METHOD}"
    ;;
  *)
    return 1
    ;;
  esac

  run curl --no-keepalive -v -s -f $extra_args "$URL"
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
  GET) ;;

  DELETE)
    extra_args="$extra_args -X${METHOD}"
    ;;
  PUT | POST)
    extra_args="$extra_args -d'{}' -X${METHOD}"
    ;;
  *)
    return 1
    ;;
  esac

  # Attempt to curl through upstream
  run curl --no-keepalive -s -i $extra_args "$URL"

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
  if ! is_set "$IS_GW"; then
    PROXY_ID="$SERVICE-sidecar-proxy"
  fi

  if output=$(docker_consul_for_proxy_bootstrap "$DC" connect envoy -bootstrap \
    -proxy-id $PROXY_ID \
    -envoy-version "$ENVOY_VERSION" \
    -admin-bind 0.0.0.0:$ADMIN_PORT ${EXTRA_ENVOY_BS_ARGS} 2>&1); then

    # All OK, write config to file
    echo "$output" >workdir/${DC}/envoy/$SERVICE-bootstrap.json
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

function wait_for_namespace {
  local NS="${1}"
  local DC=${2:-primary}
  retry_default docker_curl "$DC" -sLf "http://127.0.0.1:8500/v1/namespace/${NS}" >/dev/null
}

function wait_for_config_entry {
  retry_default read_config_entry "$@" >/dev/null
}

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}

function docker_exec {
  if ! docker.exe exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}

function assert_config_entry_status {
  local TYPE="$1"
  local STATUS="$2"
  local REASON="$3"
  local DC="$4"
  local KIND="$5"
  local NAME="$6"
  local NS=${7:-}
  local AP=${8:-}
  local PEER=${9:-}

  status=$(curl -s -f "consul-${DC}-client:8500/v1/config/${KIND}/${NAME}?passing&ns=${NS}&partition=${AP}&peer=${PEER}" | jq ".Status.Conditions[] | select(.Type == \"$TYPE\" and .Status == \"$STATUS\" and .Reason == \"$REASON\")")
  [ -n "$status" ]
}

function delete_config_entry {
  local KIND=$1
  local NAME=$2
  retry_default curl -sL -XDELETE "http://127.0.0.1:8500/v1/config/${KIND}/${NAME}"
}

function register_services {
  local DC=${1:-primary}
  wait_for_leader "$DC"
  docker_consul_exec ${DC} sh -c "consul services register /workdir/${DC}/register/service_*.hcl"
}

# wait_for_leader waits until a leader is elected.
# Its first argument must be the datacenter name.
function wait_for_leader {
  retry_default docker_consul_exec "$1" sh -c '[[ $(curl --fail -sS http://127.0.0.1:8500/v1/status/leader) ]]'
}

function setup_upsert_l4_intention {
  local SOURCE=$1
  local DESTINATION=$2
  local ACTION=$3

  retry_default docker_curl primary -sL -XPUT "http://127.0.0.1:8500/v1/connect/intentions/exact?source=${SOURCE}&destination=${DESTINATION}" \
    -d"{\"Action\": \"${ACTION}\"}" >/dev/null
}

function upsert_l4_intention {
  local SOURCE=$1
  local DESTINATION=$2
  local ACTION=$3

  retry_default curl -sL -XPUT "http://127.0.0.1:8500/v1/connect/intentions/exact?source=${SOURCE}&destination=${DESTINATION}" \
    -d"{\"Action\": \"${ACTION}\"}" >/dev/null
}

function get_ca_root {
  curl -s -f "http://localhost:8500/v1/connect/ca/roots" | jq -r ".Roots[0].RootCert"
}

function cacert_curl {
  local RESOLVE_ADDR=$1
  local ADDR=$2  
  
  run retry_default curl --cacert <(get_ca_root) -s -f -d hello --resolve $RESOLVE_ADDR $ADDR

  [ "$status" -eq 0 ]
  [ "$output" == *"hello"* ]
}

function wait_for_agent_service_register {
  local SERVICE_ID=$1
  local DC=${2:-primary}
  retry_default docker_curl "$DC" -sLf "http://127.0.0.1:8500/v1/agent/service/${SERVICE_ID}" >/dev/null
}

function set_ttl_check_state {
  local CHECK_ID=$1
  local CHECK_STATE=$2
  local DC=${3:-primary}

  case "$CHECK_STATE" in
  pass) ;;

  warn) ;;

  fail) ;;

  *)
    echo "invalid ttl check state '${CHECK_STATE}'" >&2
    return 1
    ;;
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
  # split proto if https:// is at the front of the host since the --resolve
  # string needs just a bare host.
  local PROTO=""
  local CA_FILE=""
  if [ "${HOST:0:8}" = "https://" ]; then
    HOST="${HOST:8}"
    PROTO="https://"
    extra_args="${extra_args} --cacert /workdir/test-sds-server/certs/ca-root.crt"
  fi
  # We use --resolve instead of setting a Host header since we need the right
  # name to be sent for SNI in some cases too.
  run retry_default curl -v -s -f --resolve "${HOST}:${PORT}:127.0.0.1" $extra_args \
    "${PROTO}${HOST}:${PORT}${PREFIX}/debug?env=dump"

  # Useful Debugging but breaks the expectation that the value output is just
  # the grep output when things don't fail
  if [ "$status" != 0 ]; then
    echo "GOT FORTIO OUTPUT: $output"
  fi
  [ "$status" == 0 ]
  echo "$output" | grep -E "^FORTIO_NAME="
}

function assert_expected_fortio_name {
  local EXPECT_NAME=$1
  local HOST=${2:-"localhost"}
  local PORT=${3:-5000}
  local URL_PREFIX=${4:-""}
  local DEBUG_HEADER_VALUE="${5:-""}"

  run get_upstream_fortio_name ${HOST} ${PORT} "${URL_PREFIX}" "${DEBUG_HEADER_VALUE}"

  echo "GOT: $output"

  [ "$status" == 0 ]
  [ "$output" == "FORTIO_NAME=${EXPECT_NAME}" ]
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

function get_upstream_fortio_host_header {
  local HOST=$1
  local PORT=$2
  local PREFIX=$3
  local DEBUG_HEADER_VALUE="${4:-""}"
  local extra_args
  if [[ -n "${DEBUG_HEADER_VALUE}" ]]; then
    extra_args="-H x-test-debug:${DEBUG_HEADER_VALUE}"
  fi
  run retry_default curl -v -s -f -H"Host: ${HOST}" $extra_args \
    "localhost:${PORT}${PREFIX}/debug"
  [ "$status" == 0 ]
  echo "$output" | grep -E "^Host: "
}

function assert_expected_fortio_host_header {
  local EXPECT_HOST=$1
  local HOST=${2:-"localhost"}
  local PORT=${3:-5000}
  local URL_PREFIX=${4:-""}
  local DEBUG_HEADER_VALUE="${5:-""}"

  GOT=$(get_upstream_fortio_host_header ${HOST} ${PORT} "${URL_PREFIX}" "${DEBUG_HEADER_VALUE}")

  if [ "$GOT" != "Host: ${EXPECT_HOST}" ]; then
    echo "expected Host header: $EXPECT_HOST, actual Host header: $GOT" 1>&2
    return 1
  fi
}

function create_peering {
  local GENERATE_PEER=$1
  local ESTABLISH_PEER=$2
  run curl -sL -XPOST "http://consul-${GENERATE_PEER}-client:8500/v1/peering/token" -d"{ \"PeerName\" : \"${GENERATE_PEER}-to-${ESTABLISH_PEER}\" }"
  # echo "$output" >&3
  [ "$status" == 0 ]

  local token
  token="$(echo "$output" | jq -r .PeeringToken)"
  [ -n "$token" ]

  run curl -sLv -XPOST "http://consul-${ESTABLISH_PEER}-client:8500/v1/peering/establish" -d"{ \"PeerName\" : \"${ESTABLISH_PEER}-to-${GENERATE_PEER}\", \"PeeringToken\" : \"${token}\" }"
  # echo "$output" >&3
  [ "$status" == 0 ]

  sleep 1
  run curl -s -f "http://consul-${GENERATE_PEER}-client:8500/v1/peering/${GENERATE_PEER}-to-${ESTABLISH_PEER}"
  state="$(echo "$output" | jq --raw-output .State)"

  if [ "$state" != "ACTIVE" ]; then
    echo "fail to peering: $output"
    return 1
  fi
}

function assert_service_has_imported {
  local DC=${1:-primary}
  local SERVICE_NAME=$2
  local PEER_NAME=$3

  run curl -s -f "http://consul-${DC}-client:8500/v1/peering/${PEER_NAME}"
  [ "$status" == 0 ]

  echo "$output" | jq --raw-output '.StreamStatus.ImportedServices' | grep -e "${SERVICE_NAME}"
  if [ $? -ne 0 ]; then
    echo "Error finding service: ${SERVICE_NAME}"
    return 1
  fi
}

function register_lambdas {
  local DC=${1:-primary}
  # register lambdas to the catalog
  for f in $(find workdir/${DC}/register -type f -name 'lambda_*.json'); do
    retry_default curl -sL -XPUT -d @${f} "http://localhost:8500/v1/catalog/register" >/dev/null &&
      echo "Registered Lambda: $(jq -r .Service.Service $f)"
  done
  # write service-defaults config entries for lambdas
  for f in $(find workdir/${DC}/register -type f -name 'service_defaults_*.json'); do
    varsub ${f} AWS_LAMBDA_REGION AWS_LAMBDA_ARN
    retry_default curl -sL -XPUT -d @${f} "http://localhost:8500/v1/config" >/dev/null &&
      echo "Wrote config: $(jq -r '.Kind + " / " + .Name' $f)"
  done
}

function assert_lambda_envoy_dynamic_cluster_exists {
  local HOSTPORT=$1
  local NAME_PREFIX=$2

  local BODY=$(get_envoy_dynamic_cluster_once $HOSTPORT $NAME_PREFIX)
  [ -n "$BODY" ]

  [ "$(echo $BODY | jq -r '.cluster.transport_socket.typed_config.sni')" == '*.amazonaws.com' ]
}

function assert_lambda_envoy_dynamic_http_filter_exists {
  local HOSTPORT=$1
  local ARN=$2

  local FILTER=$(get_envoy_http_filter $HOSTPORT 'envoy.filters.http.aws_lambda')
  [ -n "$FILTER" ]

  [ "$(echo $FILTER | jq -r '.typed_config | .arn')" == "$ARN" ]
}

function varsub {
  local file=$1
  shift
  for v in "$@"; do
    sed -i "s/\${$v}/${!v}/g" $file
  done
}

function get_url_header {
  local URL=$1
  local HEADER=$2
  run curl -s -f -X GET -I "${URL}"
  [ "$status" == 0 ]
  RESP=$(echo "$output" | tr -d '\r')
  RESP=$(echo "$RESP" | grep -E "^${HEADER}: ")
  RESP=$(echo "$RESP" | sed "s/^${HEADER}: //g")
  echo "$RESP"
}

function assert_url_header {
  local URL=$1
  local HEADER=$2
  local VALUE=$3
  run get_url_header "$URL" "$HEADER"
  [ "$status" == 0 ]
  [ "$VALUE" = "$output" ]
}

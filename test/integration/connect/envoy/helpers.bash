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
  while true; do
    "$@" && break || {
      exit=$?
      if [[ $n -lt $max ]]; then
        ((n++))
        echo "Command failed. Attempt $n/$max:"
        sleep $delay;
      else
        echo "The command has failed after $n attempts." >&2
        return $exit
      fi
    }
  done
}

function retry_default {
  retry 5 1 $@
}

function retry_long {
  retry 30 1 $@
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

function get_cert {
  local HOSTPORT=$1
  openssl s_client -connect $HOSTPORT \
    -showcerts 2>/dev/null \
    | openssl x509 -noout -text
}

function assert_proxy_presents_cert_uri {
  local HOSTPORT=$1
  local SERVICENAME=$2

  CERT=$(retry_default get_cert $HOSTPORT)

  echo "WANT SERVICE: $SERVICENAME"
  echo "GOT CERT:"
  echo "$CERT"

  echo "$CERT" | grep -Eo "URI:spiffe://([a-zA-Z0-9-]+).consul/ns/default/dc/dc1/svc/$SERVICENAME"
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
  echo "Want version=$ENVOY_VERSION"
  echo $VERSION | grep "/$ENVOY_VERSION/"
}

function get_envoy_listener_filters {
  local HOSTPORT=$1
  run retry_default curl -s -f $HOSTPORT/config_dump
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '.configs[2].dynamic_active_listeners[].listener | "\(.name) \( .filter_chains[0].filters | map(.name) | join(","))"'
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

  docker_wget "http://${HOSTPORT}/config_dump" -q -O - > "./workdir/envoy/${ENVOY_NAME}-config_dump.json"
  docker_wget "http://${HOSTPORT}/clusters?format=json" -q -O - > "./workdir/envoy/${ENVOY_NAME}-clusters.json"
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
| select(.name|startswith(\"${CLUSTER_NAME}.default.dc1.internal.\"))
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

function get_healthy_service_count {
  local SERVICE_NAME=$1
  run retry_default curl -s -f 127.0.0.1:8500/v1/health/connect/${SERVICE_NAME}?passing
  [ "$status" -eq 0 ]
  echo "$output" | jq --raw-output '. | length'
}

function assert_service_has_healthy_instances_once {
  local SERVICE_NAME=$1
  local EXPECT_COUNT=$2

  GOT_COUNT=$(get_healthy_service_count $SERVICE_NAME)

  [ "$GOT_COUNT" -eq $EXPECT_COUNT ]
}

function assert_service_has_healthy_instances {
  local SERVICE_NAME=$1
  local EXPECT_COUNT=$2

  run retry_long assert_service_has_healthy_instances_once $SERVICE_NAME $EXPECT_COUNT
  [ "$status" -eq 0 ]
}

function docker_consul {
  docker run -i --rm --network container:envoy_consul_1 consul-dev $@
}

function docker_wget {
  docker run -ti --rm --network container:envoy_consul_1 alpine:3.9 wget $@
}

function get_envoy_pid {
  local BOOTSTRAP_NAME=$1
  run ps aux
  [ "$status" == 0 ]
  PID="$(echo "$output" | grep "envoy -c /workdir/envoy/${BOOTSTRAP_NAME}-bootstrap.json" | awk '{print $1}')"
  [ -n "$PID" ]

  echo "$PID"
}

function kill_envoy {
  local BOOTSTRAP_NAME=$1

  PID="$(get_envoy_pid $BOOTSTRAP_NAME)"
  echo "PID = $PID"

  kill -TERM $PID
}

function must_match_in_statsd_logs {
  run cat /workdir/statsd/statsd.log
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
  run curl -s -i -d hello $1

  echo "OUTPUT $output"

  # Should fail request with 503
  echo "$output" | grep '503 Service Unavailable'
}

function gen_envoy_bootstrap {
  SERVICE=$1
  ADMIN_PORT=$2

  if output=$(docker_consul connect envoy -bootstrap \
    -proxy-id $SERVICE-sidecar-proxy \
    -admin-bind 0.0.0.0:$ADMIN_PORT 2>&1); then

    # All OK, write config to file
    echo "$output" > workdir/envoy/$SERVICE-bootstrap.json
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
  docker_consul config read -kind $KIND -name $NAME
}

function wait_for_config_entry {
  local KIND=$1
  local NAME=$2
  retry_default read_config_entry $KIND $NAME >/dev/null
}

function get_upstream_fortio_name {
  run retry_default curl -v -s -f localhost:5000/debug?env=dump
  [ "$status" == 0 ]
  echo "$output" | grep -E "^FORTIO_NAME="
}

function assert_expected_fortio_name {
  local EXPECT_NAME=$1

  GOT=$(get_upstream_fortio_name)
  echo "GOT $GOT"

  [ "$GOT" == "FORTIO_NAME=${EXPECT_NAME}" ]
}

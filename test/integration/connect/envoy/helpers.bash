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
  retry 2 10 $@
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

function docker_consul {
  docker run -ti --network container:envoy_consul_1 consul-dev $@
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
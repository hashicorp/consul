#!/usr/bin/env bats

load helpers

@test "ingress proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "ingress-gateway should have healthy endpoints for s1" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "s1 proxy should have been configured with connection threshold from defaults and service" {
  CLUSTER_THRESHOLD=$(get_envoy_cluster_config 127.0.0.1:20000 s1.default.primary | jq '.circuit_breakers.thresholds[0]')
  echo $CLUSTER_THRESHOLD

  MAX_CONNS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_connections')
  MAX_PENDING_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_pending_requests')
  MAX_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_requests')

  echo "MAX_CONNS = $MAX_CONNS"
  echo "MAX_PENDING_REQS = $MAX_PENDING_REQS"
  echo "MAX_REQS = $MAX_REQS"

  [ "$MAX_CONNS" = "100" ]
  [ "$MAX_PENDING_REQS" = "200" ]
  [ "$MAX_REQS" = "30" ]
}

@test "ingress should be able to connect to s1 via configured port" {
  run retry_default curl -s -f -d hello localhost:9999
  [ "$status" -eq 0 ]
  [ "$output" = "hello" ]
}

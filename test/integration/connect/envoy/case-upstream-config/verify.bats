#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s2 proxy should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

@test "s1 proxy should have been configured with max_connections on the cluster" {
  CLUSTER_THRESHOLD=$(get_envoy_cluster_config localhost:19000 s2.default.primary | jq '.circuit_breakers.thresholds[0]')
  echo $CLUSTER_THRESHOLD

  MAX_CONNS=$(echo $CLUSTER_THRESHOLD | jq  --raw-output '.max_connections')
  MAX_PENDING_REQS=$(echo $CLUSTER_THRESHOLD | jq  --raw-output '.max_pending_requests')
  MAX_REQS=$(echo $CLUSTER_THRESHOLD | jq  --raw-output '.max_requests')

  echo "MAX_CONNS = $MAX_CONNS"
  echo "MAX_PENDING_REQS = $MAX_PENDING_REQS"
  echo "MAX_REQS = $MAX_REQS"

  [ "$MAX_CONNS" = "3" ]
  [ "$MAX_PENDING_REQS" = "4" ]
  [ "$MAX_REQS" = "5" ]
}

@test "s1 proxy should have been configured with passive_health_check" {
  CLUSTER_CONFIG=$(get_envoy_cluster_config localhost:19000 s2.default.primary)
  echo $CLUSTER_CONFIG

  [ "$(echo $CLUSTER_CONFIG | jq --raw-output '.outlier_detection.consecutive_5xx')" = "4" ]
  [ "$(echo $CLUSTER_CONFIG | jq --raw-output '.outlier_detection.interval')" = "22s" ]
  [ "$(echo $CLUSTER_CONFIG | jq --raw-output '.outlier_detection.enforcing_consecutive_5xx')" = "99" ]
  [ "$(echo $CLUSTER_CONFIG | jq --raw-output '.outlier_detection.max_ejection_percent')" = "50" ]
  [ "$(echo $CLUSTER_CONFIG | jq --raw-output '.outlier_detection.base_ejection_time')" = "60s" ]
}

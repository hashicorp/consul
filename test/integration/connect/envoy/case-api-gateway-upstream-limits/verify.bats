#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have been accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway routes should be bound" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-default
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-override
}

@test "api gateway should have healthy endpoints for s1 and s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "default route cluster should inherit limits from gateway defaults" {
  CLUSTER_THRESHOLD=$(get_envoy_cluster_config 127.0.0.1:20000 s1.default.primary | jq '.circuit_breakers.thresholds[0]')
  echo $CLUSTER_THRESHOLD

  MAX_CONNS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_connections')
  MAX_PENDING_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_pending_requests')
  MAX_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_requests')

  [ "$MAX_CONNS" = "5" ]
  [ "$MAX_PENDING_REQS" = "3" ]
  [ "$MAX_REQS" = "4" ]
}

@test "override route cluster should use service-level limits" {
  CLUSTER_THRESHOLD=$(get_envoy_cluster_config 127.0.0.1:20000 s2.default.primary | jq '.circuit_breakers.thresholds[0]')
  echo $CLUSTER_THRESHOLD

  MAX_CONNS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_connections')
  MAX_PENDING_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_pending_requests')
  MAX_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_requests')

  [ "$MAX_CONNS" = "2" ]
  [ "$MAX_PENDING_REQS" = "1" ]
  [ "$MAX_REQS" = "2" ]
}

@test "api gateway should route default host to s1" {
  assert_expected_fortio_name s1 default.consul.example 9999
}

@test "api gateway should route override host to s2" {
  assert_expected_fortio_name s2 override.consul.example 9999
}

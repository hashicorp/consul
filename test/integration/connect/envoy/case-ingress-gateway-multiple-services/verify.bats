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

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s1 proxies should be healthy" {
  assert_service_has_healthy_instances s1 1
}

@test "s2 proxies should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "ingress-gateway should have healthy endpoints for s1" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "ingress-gateway should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "s2 proxy should have been configured with connection threshold from defaults" {
  CLUSTER_THRESHOLD=$(get_envoy_cluster_config 127.0.0.1:20000 s2.default.primary | jq '.circuit_breakers.thresholds[0]')
  echo $CLUSTER_THRESHOLD

  MAX_CONNS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_connections')
  MAX_PENDING_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_pending_requests')
  MAX_REQS=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.max_requests')

  echo "MAX_CONNS = $MAX_CONNS"
  echo "MAX_PENDING_REQS = $MAX_PENDING_REQS"
  echo "MAX_REQS = $MAX_REQS"

  [ "$MAX_CONNS" = "10" ]
  [ "$MAX_PENDING_REQS" = "20" ]
  [ "$MAX_REQS" = "30" ]
}

@test "s2 proxy should have been configured with outlier detection in ingress gateway" {
  CLUSTER_THRESHOLD=$(get_envoy_cluster_config 127.0.0.1:20000 s2.default.primary | jq '.outlier_detection')
  echo $CLUSTER_THRESHOLD

  INTERVAL=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.interval')
  CONSECTIVE5xx=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.consecutive_5xx')
  ENFORCING_CONSECTIVE5xx=$(echo $CLUSTER_THRESHOLD | jq --raw-output '.enforcing_consecutive_5xx')

  echo "INTERVAL = $INTERVAL"
  echo "CONSECTIVE5xx = $CONSECTIVE5xx"
  echo "ENFORCING_CONSECTIVE5xx = $ENFORCING_CONSECTIVE5xx"

  [ "$INTERVAL" = "5s" ]
  [ "$CONSECTIVE5xx" = "10" ]
  [ "$ENFORCING_CONSECTIVE5xx" = null ]
}

@test "ingress should be able to connect to s1 using Host header" {
  assert_expected_fortio_name s1 s1.ingress.consul 9999
}

@test "ingress should be able to connect to s2 using Host header" {
  assert_expected_fortio_name s2 s2.ingress.consul 9999
}

@test "ingress should be able to connect to s1 using a user-specified Host" {
  assert_expected_fortio_name s1 test.example.com 9998
}

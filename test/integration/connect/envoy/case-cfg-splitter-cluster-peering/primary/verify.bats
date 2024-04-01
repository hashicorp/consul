#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}

@test "s1 proxy admin is up" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary
}

@test "s2 proxies should be healthy in alpha" {
  assert_service_has_healthy_instances s2 1 alpha
}

@test "gateway-alpha should be up and listening" {
  retry_long nc -z consul-alpha-client:4432
}

@test "peer the two clusters together" {
  retry_default create_peering primary alpha
}

@test "s2 alpha proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary "" "" primary-to-alpha
}

@test "s1 upstream should have healthy endpoints for s2 primary and alpha" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary.internal HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary-to-alpha.external HEALTHY 1
}

@test "s1 upstream should be split between peer and local dc" {
  retry_long assert_url_header "http://127.0.0.1:5000/" "x-test-split" "primary"
  [ "$status" -eq 0 ]
  retry_long assert_url_header "http://127.0.0.1:5000/" "x-test-split" "alpha"
  [ "$status" -eq 0 ]
  retry_long assert_expected_fortio_name s2 127.0.0.1 5000
  retry_long assert_expected_fortio_name s2-alpha 127.0.0.1 5000
}

@test "s1 upstream made 2 connections to primary s2 split" {
  retry_long assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.s2.default.primary.internal.*upstream_rq_total" 1
}

@test "s1 upstream made 2 connections to alpha s2 split" {
  retry_long assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.s2.default.primary-to-alpha.external.*upstream_rq_total" 1
}

@test "reset envoy statistics" {
  reset_envoy_metrics 127.0.0.1:19000
  retry_long assert_envoy_metric 127.0.0.1:19000 "cluster.s2.default.primary-to-alpha.external.*upstream_rq_total" 0
}

@test "s1 upstream should be able to connect to s2 via peer-s2" {
  assert_expected_fortio_name s2-alpha 127.0.0.1 5001
}

@test "s1 upstream made 1 connection to s2 via peer-s2" {
  retry_long assert_envoy_metric_at_least 127.0.0.1:19000 "http.upstream.peer-s2.default.default.primary.rq_total" 1
}

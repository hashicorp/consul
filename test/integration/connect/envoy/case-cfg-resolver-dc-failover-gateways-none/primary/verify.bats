#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
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

@test "s2 proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary
}

@test "s2 proxies should be healthy in secondary" {
  assert_service_has_healthy_instances s2 1 secondary
}

################
# PHASE 1: we show that by default requests are served from the primary

# Note: when failover is configured the cluster is named for the original
# service not any destination related to failover.
@test "s1 upstream should have healthy endpoints for s2 in both primary and failover" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 2
}

@test "s1 upstream should be able to connect to s2 via upstream s2 to start" {
  assert_expected_fortio_name s2
}

@test "s1 upstream made 1 connection" {
  assert_envoy_metric 127.0.0.1:19000 "cluster.s2.default.primary.*cx_total" 1
}

################
# PHASE 2: we show that in failover requests are served from the secondary
#
@test "terminate instance of s2 primary envoy which should trigger failover to s2 secondary when tcp check fails" {
  kill_envoy s2 primary
}

@test "s2 proxies should be unhealthy in primary" {
  assert_service_has_healthy_instances s2 0 primary
}

@test "s1 upstream should have healthy endpoints for s2 secondary and unhealthy endpoints for s2 primary" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary UNHEALTHY 1
}

@test "s1 upstream should be able to connect to s2 in secondary now" {
  assert_expected_fortio_name s2-secondary
}

@test "s1 upstream made 2 connections" {
  assert_envoy_metric 127.0.0.1:19000 "cluster.s2.default.primary.*cx_total" 2
}

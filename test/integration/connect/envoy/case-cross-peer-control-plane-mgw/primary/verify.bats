#!/usr/bin/env bats

load helpers

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "gateway-primary proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxies should be healthy in alpha" {
  assert_service_has_healthy_instances s2 1 alpha
}

@test "gateway-primary should be up and listening" {
  retry_long nc -z consul-primary-client:4431
}

@test "gateway-alpha should be up and listening" {
  retry_long nc -z consul-alpha-client:4432
}

@test "peer the two clusters together" {
  retry_default create_peering primary alpha
}

@test "s2 alpha proxies should be imported in primary" {
  retry_long assert_service_has_imported primary s2 primary-to-alpha
  [ "$status" -eq 0 ]
}

@test "acceptor gateway-primary should have healthy endpoints for primary servers" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19001 server.primary.peering HEALTHY 1
}

@test "alpha servers made connection to primary servers via primary gateway" {
  assert_envoy_metric_at_least 127.0.0.1:19001 "cluster.server.primary.peering.*cx_total" 1
}

@test "s2 alpha proxies should be healthy in primary" {
  assert_service_has_healthy_instances s2 1 primary "" "" primary-to-alpha
}

@test "gateway-alpha should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status consul-alpha-client:19003 exported~s2.default.alpha HEALTHY 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary-to-alpha.external HEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  run retry_default curl -s -f -d hello localhost:5000
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "s1 upstream made 1 connection to s2" {
  assert_envoy_metric_at_least 127.0.0.1:19000 "cluster.s2.default.primary-to-alpha.external.*cx_total" 1
}

@test "s1 upstream made 1 connection to s2 through the primary mesh gateway" {
  assert_envoy_metric_at_least 127.0.0.1:19001 "cluster.s2.default.default.alpha-to-primary.external.*cx_total" 1
}

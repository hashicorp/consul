#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "terminating proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "terminating-gateway-primary listener is up on :8443" {
  retry_default nc -z localhost:8443
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s1 upstream should have healthy endpoints for v1.s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 v1.s2 HEALTHY 1
}

@test "terminating-gateway should have healthy endpoints for v1.s2" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 v1.s2 HEALTHY 1
}

@test "terminating-gateway should have healthy endpoints for v2.s2" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 v2.s2 HEALTHY 1
}

@test "s1 upstream should be able to connect to s2-v1 via terminating-gateway" {
  assert_expected_fortio_name s2-v1
}

@test "terminating-gateway is used for the upstream connection" {
  assert_envoy_metric_at_least 127.0.0.1:20000 "v1.s2.default.primary.*cx_total" 1
}

@test "terminating-gateway is used for the upstream connection of the proxy" {
  # make sure we resolve the terminating gateway as endpoint for the upstream
  assert_upstream_has_endpoint_port 127.0.0.1:19001 "v1.s2" 8443
}

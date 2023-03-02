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

@test "deleting the service-resolver should be possible" {
  delete_config_entry service-resolver s2
}

@test "terminating gateway should no longer have v1.s2 endpoints" {
  assert_upstream_missing 127.0.0.1:20000 v1.s2
}

@test "terminating gateway should no longer have v2.s2 endpoints" {
  assert_upstream_missing 127.0.0.1:20000 v2.s2
}

@test "terminating gateway should still have s2 endpoints" {
  # expected 3 nodes here due to s1, s2, s2-v1, and s2-v2, the latter
  # all starting with "s2"
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 3
}


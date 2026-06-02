#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway and single route should be accepted/bound" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route
}

@test "api gateway should have healthy endpoints for both services in single route" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "api gateway should serve foo.example.com cert for single-route multi-service" {
  assert_cert_signed_by_ca /workdir/test-sds-server/certs/ca-root.crt \
    localhost:9999 foo.example.com
}

@test "api gateway single-route multi-service should return a response for foo.example.com" {
  run retry_long curl -k -s -f -H "Host: foo.example.com" -d hello https://localhost:9999
  [ "$status" -eq 0 ]
  [[ ! -z "$output" ]]
}


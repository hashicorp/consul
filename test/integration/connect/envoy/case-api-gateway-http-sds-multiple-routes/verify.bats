#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway and routes should be accepted/bound" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-one
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-two
}

@test "api gateway should have healthy endpoints for route-one and route-two upstreams" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "api gateway should route foo.example.com to s1" {
  assert_expected_fortio_name s1 https://foo.example.com 9999
}

@test "api gateway should route www.example.com to s2" {
  assert_expected_fortio_name s2 https://www.example.com 9999
}

@test "api gateway should serve foo.example.com cert from SDS" {
  assert_cert_signed_by_ca /workdir/test-sds-server/certs/ca-root.crt \
    localhost:9999 foo.example.com
}

@test "api gateway should serve www.example.com cert from SDS" {
  assert_cert_signed_by_ca /workdir/test-sds-server/certs/ca-root.crt \
    localhost:9999 www.example.com
}


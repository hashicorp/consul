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

@test "ingress-gateway should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "ingress-gateway should have healthy endpoints for s2" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
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

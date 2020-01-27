#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2-v1 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2-v2 proxy admin is up on :19002" {
  retry_default curl -f -s localhost:19002/stats -o /dev/null
}

@test "s2 proxy admin is up on :19003" {
  retry_default curl -f -s localhost:19003/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2-v1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s2-v2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21002 s2
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21003 s2
}

@test "s2 proxies should be healthy" {
  assert_service_has_healthy_instances s2 3
}

@test "s1 upstream should have healthy endpoints for v2.s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 v2.s2.default.primary HEALTHY 1
}

@test "s1 upstream should be able to connect to s2-v2 via upstream s2" {
  assert_expected_fortio_name s2-v2
}


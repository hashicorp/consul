#!/usr/bin/env bats

load helpers

@test "proxies are running correct version" {
  assert_envoy_version 19001
  assert_envoy_version 19002
}

@test "s1 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2 proxy admin is up on :19002" {
  retry_default curl -f -s localhost:19002/stats -o /dev/null
}

@test "gateway-alpha proxy admin is up on :19003" {
  retry_default curl -f -s localhost:19003/stats -o /dev/null
}

@test "proxy listeners should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1 alpha
  assert_proxy_presents_cert_uri localhost:21001 s2 alpha
}

@test "s1 and s2 proxies should be healthy" {
  assert_service_has_healthy_instances s1 1 alpha
  assert_service_has_healthy_instances s2 1 alpha
}

@test "gateway-alpha should be up and listening" {
  retry_long nc -z consul-alpha-client:4432
}

#!/usr/bin/env bats

load helpers

# This test case really only validates the exec mechanism worked and brought
# envoy up.

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s1 proxy is running correct version" {
  assert_envoy_version 19000
}


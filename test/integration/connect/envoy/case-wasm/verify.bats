#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxy is configured with a wasm http filter" {
  run get_envoy_http_filter localhost:19001 envoy.filters.http.wasm
  [ "$status" == 0 ]

  [ "$(echo "$output" | jq -r '.typed_config.config.vm_config.runtime')" == "envoy.wasm.runtime.v8" ]
  [ "$(echo "$output" | jq -r '.typed_config.config.vm_config.code.local.filename')" == "/workdir/primary/data/dummy.wasm" ]
  [ "$(echo "$output" | jq -r '.typed_config.config.configuration.value')" == "plugin configuration" ]
  [ "$(echo "$output" | jq -r '.typed_config.config.fail_open')" == "true" ]
}

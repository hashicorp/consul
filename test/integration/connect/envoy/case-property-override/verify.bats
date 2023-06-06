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

@test "s1 proxy is configured with the expected envoy patches" {
  run get_envoy_cluster_config localhost:19000 s2
  [ "$status" == 0 ]

  [ "$(echo "$output" | jq -r '.upstream_connection_options.tcp_keepalive.keepalive_probes')" == "1234" ]
  [ "$(echo "$output" | jq -r '.outlier_detection')" == "null" ]

  run get_envoy_cluster_config localhost:19000 s3
  [ "$status" == 0 ]

  [ "$(echo "$output" | jq -r '.upstream_connection_options.tcp_keepalive.keepalive_probes')" == "1234" ]
  [ "$(echo "$output" | jq -r '.outlier_detection')" == "{}" ]
}

@test "s2 proxy is configured with the expected envoy patches" {
  run get_envoy_public_listener_once localhost:19001
  [ "$status" == 0 ]
  
  [ "$(echo "$output" | jq -r '.active_state.listener.stat_prefix')" == "custom.stats.s2" ]
}

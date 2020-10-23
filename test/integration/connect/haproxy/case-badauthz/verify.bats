#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :8080" {
 retry_default curl -f -s localhost:8080 -o /dev/null
}

@test "s2 proxy admin is up on :8181" {
 retry_default curl -f -s localhost:8181 -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

#@test "s2 proxy should be healthy" {
#  assert_service_has_healthy_instances s2 1
#}
#
#  This is specific to Envoy, test something else if possible
#  https://www.envoyproxy.io/docs/envoy/latest/api-v3/admin/v3/clusters.proto.html?highlight=cluster_statuses
# @test "s1 upstream should have healthy endpoints for s2" {
#   assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
# }

@test "s1 upstream should NOT be able to connect to s2" {
  run retry_default must_fail_http_connection_with_502 localhost:5000

  echo "OUTPUT $output"

  [ "$status" == "0" ]
}

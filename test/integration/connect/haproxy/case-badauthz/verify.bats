#!/usr/bin/env bats

load helpers

#@test "s1 proxy admin is up on :19000" {
#  retry_default curl -f -s localhost:19000/stats -o /dev/null
#}
#
#@test "s2 proxy admin is up on :19001" {
#  retry_default curl -f -s localhost:19001/stats -o /dev/null
#}

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
#@test "s1 upstream should have healthy endpoints for s2" {
#  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
#}

@test "s1 upstream should NOT be able to connect to s2" {
  #run retry_default must_fail_tcp_connection localhost:5000
  run retry_default must_fail_http_connection localhost:5000

  echo "OUTPUT $output"

  [ "$status" == "0" ]
}

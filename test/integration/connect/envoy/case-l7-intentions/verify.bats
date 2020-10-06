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

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s2 proxies should be healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have healthy endpoints for s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2.default.primary HEALTHY 1
}

# these all use the same context: "s1 upstream should be able to connect to s2 via upstream s2"

@test "test exact path" {
  must_pass_http_request GET localhost:5000/exact
  must_fail_http_request GET localhost:5000/exact-nope
}

@test "test prefix path" {
  must_pass_http_request GET localhost:5000/prefix
  must_fail_http_request GET localhost:5000/nope-prefix
}

@test "test regex path" {
  must_pass_http_request GET localhost:5000/regex
  must_fail_http_request GET localhost:5000/reggex
}

@test "test present header" {
  must_pass_http_request GET localhost:5000/hdr-present anything
  must_fail_http_request GET localhost:5000/hdr-present ""
}

@test "test exact header" {
  must_pass_http_request GET localhost:5000/hdr-exact exact
  must_fail_http_request GET localhost:5000/hdr-exact exact-nope
}

@test "test prefix header" {
  must_pass_http_request GET localhost:5000/hdr-prefix prefix
  must_fail_http_request GET localhost:5000/hdr-prefix nope-prefix
}

@test "test suffix header" {
  must_pass_http_request GET localhost:5000/hdr-suffix suffix
  must_fail_http_request GET localhost:5000/hdr-suffix suffix-nope
}

@test "test regex header" {
  must_pass_http_request GET localhost:5000/hdr-regex regex
  must_fail_http_request GET localhost:5000/hdr-regex reggex
}

@test "test method match" {
  must_pass_http_request GET localhost:5000/method-match
  must_pass_http_request PUT localhost:5000/method-match
  must_fail_http_request POST localhost:5000/method-match
  must_fail_http_request HEAD localhost:5000/method-match
}

# @test "s1 upstream should NOT be able to connect to s2" {
#   run retry_default must_fail_tcp_connection localhost:5000

#   echo "OUTPUT $output"

#   [ "$status" == "0" ]
# }

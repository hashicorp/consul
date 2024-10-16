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

@test "s2 should have http rbac rules loaded from xDS" {
  retry_default assert_envoy_http_rbac_policy_count localhost:19001 1
}

# The following tests exercise the same cases in "case-l7-intentions-request-normalization"
# but with all normalization disabled, including default path normalization. Note that
# disabling normalization is not recommended in production environments unless specifically
# required.

@test "test allowed path" {
  retry_default must_pass_http_request GET localhost:5000/foo
  retry_default must_pass_http_request GET localhost:5000/value/foo
  retry_default must_pass_http_request GET localhost:5000/foo/supersecret
}

@test "test disallowed path" {
  retry_default must_fail_http_request 403 GET 'localhost:5000/value/supersecret'
  retry_default must_fail_http_request 403 GET 'localhost:5000/value/supersecret#foo'
  retry_default must_fail_http_request 403 GET 'localhost:5000/value/supersecret?'
}

@test "test ignored disallowed path with repeat slashes" {
  retry_default must_pass_http_request GET 'localhost:5000/value//supersecret'
  get_echo_request_path | grep -Fx '/value//supersecret'
  retry_default must_pass_http_request GET 'localhost:5000/value///supersecret'
  get_echo_request_path | grep -Fx '/value///supersecret'
}

@test "test ignored disallowed path with escaped characters" {
  # escaped '/' (HTTP reserved)
  retry_default must_pass_http_request GET 'localhost:5000/value%2Fsupersecret'
  get_echo_request_path | grep -Fx '/value%2Fsupersecret'
  # escaped 'v' (not HTTP reserved)
  retry_default must_pass_http_request GET 'localhost:5000/value/%73upersecret'
  get_echo_request_path | grep -Fx '/value/%73upersecret'
}

@test "test ignored disallowed path with backward slashes" {
  # URLs must be quoted due to backslashes, otherwise shell erases them
  retry_default must_pass_http_request GET 'localhost:5000/value\supersecret'
  get_echo_request_path | grep -Fx '/value\supersecret'
  retry_default must_pass_http_request GET 'localhost:5000/value\\supersecret'
  get_echo_request_path | grep -Fx '/value\\supersecret'
  retry_default must_pass_http_request GET 'localhost:5000/value\/supersecret'
  get_echo_request_path | grep -Fx '/value\/supersecret'
  retry_default must_pass_http_request GET 'localhost:5000/value/\/supersecret'
  get_echo_request_path | grep -Fx '/value/\/supersecret'
}

@test "test ignored underscore in header key" {
  retry_default must_pass_http_request GET localhost:5000/foo x_poison:anything
  get_echo_request_header_value "x_poison" | grep -Fx 'anything'
  retry_default must_pass_http_request GET localhost:5000/foo x_check:bad
  get_echo_request_header_value "x_check" | grep -Fx 'bad'
  retry_default must_pass_http_request GET localhost:5000/foo x_check:good-sufbad
  get_echo_request_header_value "x_check" | grep -Fx 'good-sufbad'
  retry_default must_pass_http_request GET localhost:5000/foo x_check:prebad-good
  get_echo_request_header_value "x_check" | grep -Fx 'prebad-good'
}

# Header contains and ignoreCase are not expected to change behavior with normalization
# disabled, so those cases from "case-l7-intentions-request-normalization" are omitted here.


# @test "s1 upstream should NOT be able to connect to s2" {
#   run retry_default must_fail_tcp_connection localhost:5000

#   echo "OUTPUT $output"

#   [ "$status" == "0" ]
# }

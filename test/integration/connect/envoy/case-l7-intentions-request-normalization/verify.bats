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

# The following tests assert one of two things: that the request was
# rejected by L7 intentions as expected due to normalization, or that the
# request was allowed, and the request received by the upstream matched the
# expected normalized form.

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

@test "test disallowed path with repeat slashes" {
  retry_default must_fail_http_request 403 GET 'localhost:5000/value//supersecret'
  retry_default must_fail_http_request 403 GET 'localhost:5000/value///supersecret'
}

@test "test path with repeat slashes normalized" {
  # After each request, verify that the request path observed by fortio matches the expected normalized path.
  retry_default must_pass_http_request GET 'localhost:5000/value//foo'
  get_echo_request_path | grep -Fx '/value/foo'
  retry_default must_pass_http_request GET 'localhost:5000/value///foo'
  get_echo_request_path | grep -Fx '/value/foo'
}

@test "test disallowed path with escaped characters" {
  # escaped '/' (HTTP reserved)
  retry_default must_fail_http_request 403 GET 'localhost:5000/value%2Fsupersecret'
  # escaped 'v' (not HTTP reserved)
  retry_default must_fail_http_request 403 GET 'localhost:5000/value/%73upersecret'
}

@test "test path with escaped characters normalized" {
  # escaped '/' (HTTP reserved)
  retry_default must_pass_http_request GET 'localhost:5000/value%2Ffoo'
  get_echo_request_path | grep -Fx '/value/foo'
  # escaped 'v' (not HTTP reserved)
  retry_default must_pass_http_request GET 'localhost:5000/value/%66oo'
  get_echo_request_path | grep -Fx '/value/foo'
}

@test "test disallowed path with backward slashes" {
  # URLs must be quoted due to backslashes, otherwise shell erases them
  retry_default must_fail_http_request 403 GET 'localhost:5000/value\supersecret'
  retry_default must_fail_http_request 403 GET 'localhost:5000/value\\supersecret'
  retry_default must_fail_http_request 403 GET 'localhost:5000/value\/supersecret'
  retry_default must_fail_http_request 403 GET 'localhost:5000/value/\/supersecret'
}

@test "test path with backward slashes normalized" {
  retry_default must_pass_http_request GET 'localhost:5000/value\foo'
  get_echo_request_path | grep -Fx '/value/foo'
  retry_default must_pass_http_request GET 'localhost:5000/value\\foo'
  get_echo_request_path | grep -Fx '/value/foo'
  retry_default must_pass_http_request GET 'localhost:5000/value\/foo'
  get_echo_request_path | grep -Fx '/value/foo'
  retry_default must_pass_http_request GET 'localhost:5000/value/\/foo'
  get_echo_request_path | grep -Fx '/value/foo'
}

@test "test disallowed underscore in header key" {
  # Envoy responds with 400 when configured to reject underscore headers.
  retry_default must_fail_http_request 400 GET localhost:5000/foo x_poison:anything
  retry_default must_fail_http_request 400 GET localhost:5000/foo x_check:bad
  retry_default must_fail_http_request 400 GET localhost:5000/foo x_check:good-sufbad
  retry_default must_fail_http_request 400 GET localhost:5000/foo x_check:prebad-good
}

@test "test disallowed contains header" {
  retry_default must_fail_http_request 403 GET localhost:5000/foo x-check:thiscontainsbadinit
}

@test "test disallowed ignore case header" {
  retry_default must_fail_http_request 403 GET localhost:5000/foo x-check:exactBaD
  retry_default must_fail_http_request 403 GET localhost:5000/foo x-check:good-SuFBaD
  retry_default must_fail_http_request 403 GET localhost:5000/foo x-check:PrEBaD-good
  retry_default must_fail_http_request 403 GET localhost:5000/foo x-check:thiscontainsBaDinit
  retry_default must_fail_http_request 403 GET localhost:5000/foo Host:foo.BaD.com
}

@test "test case-insensitive disallowed header" {
  retry_default must_fail_http_request 403 GET localhost:5000/foo Host:foo.BAD.com
}


# @test "s1 upstream should NOT be able to connect to s2" {
#   run retry_default must_fail_tcp_connection localhost:5000

#   echo "OUTPUT $output"

#   [ "$status" == "0" ]
# }

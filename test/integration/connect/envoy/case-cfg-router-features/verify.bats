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

### the router fallthrough logic sends you to v1, otherwise you go to v2

# these all use the same context: "s1 upstream should be able to connect to s2-v2 via upstream s2"

@test "test exact path" {
  assert_expected_fortio_name s2-v2 localhost 5000 /exact
  assert_expected_fortio_name s2-v1 localhost 5000 /exact-alt
}

@test "test prefix path" {
  assert_expected_fortio_name s2-v2 localhost 5000 /prefix
  assert_expected_fortio_name s2-v1 localhost 5000 /prefix-alt
}

@test "test regex path" {
  assert_expected_fortio_name s2-v2 localhost 5000 "" regex-path
}

@test "test present header" {
  assert_expected_fortio_name s2-v2 localhost 5000 /hdr-present anything
  assert_expected_fortio_name s2-v1 localhost 5000 /hdr-present ""
}

@test "test exact header" {
  assert_expected_fortio_name s2-v2 localhost 5000 /hdr-exact exact
  assert_expected_fortio_name s2-v1 localhost 5000 /hdr-exact exact-alt
}

@test "test prefix header" {
  assert_expected_fortio_name s2-v2 localhost 5000 /hdr-prefix prefix
  assert_expected_fortio_name s2-v1 localhost 5000 /hdr-prefix alt-prefix
}

@test "test suffix header" {
  assert_expected_fortio_name s2-v2 localhost 5000 /hdr-suffix suffix
  assert_expected_fortio_name s2-v1 localhost 5000 /hdr-suffix suffix-alt
}

@test "test regex header" {
  assert_expected_fortio_name s2-v2 localhost 5000 /hdr-regex regex
  assert_expected_fortio_name s2-v1 localhost 5000 /hdr-regex regexx
}

@test "test inverted header" {
  assert_expected_fortio_name s2-v2 localhost 5000 /hdr-invert something-else
}

@test "test present query param" {
  assert_expected_fortio_name s2-v2 localhost 5000 /qp-present
}

@test "test exact query param" {
  assert_expected_fortio_name s2-v2 localhost 5000 /qp-exact
}

@test "test regex query param" {
  assert_expected_fortio_name s2-v2 localhost 5000 /qp-regex
}

@test "test method match" {
  assert_expected_fortio_name s2-v2 localhost 5000 /method-match
}

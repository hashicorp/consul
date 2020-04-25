#!/usr/bin/env bats

load helpers

@test "ingress proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

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

@test "ingress-gateway should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "ingress-gateway should have healthy endpoints for s2" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s2 HEALTHY 1
}

@test "ingress should be able to connect to s1 via configured path" {
  run retry_default curl -s -f localhost:9999/s1/debug?env=dump
  [ "$status" -eq 0 ]

  GOT=$(echo "$output" | grep -E "^FORTIO_NAME=")
  EXPECT_NAME="s1"

  if [ "$GOT" != "FORTIO_NAME=${EXPECT_NAME}" ]; then
    echo "expected name: $EXPECT_NAME, actual name: $GOT" 1>&2
    return 1
  fi
}

@test "ingress should be able to connect to s2 via configured path" {
  run retry_default curl -s -f localhost:9999/s2/debug?env=dump
  [ "$status" -eq 0 ]

  GOT=$(echo "$output" | grep -E "^FORTIO_NAME=")
  EXPECT_NAME="s2"

  if [ "$GOT" != "FORTIO_NAME=${EXPECT_NAME}" ]; then
    echo "expected name: $EXPECT_NAME, actual name: $GOT" 1>&2
    return 1
  fi
}


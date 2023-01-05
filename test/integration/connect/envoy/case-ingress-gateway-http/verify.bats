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
  assert_expected_fortio_name s1 router.ingress.consul 9999 /s1
}

@test "ingress should be able to connect to s2 via configured path" {
  assert_expected_fortio_name s2 router.ingress.consul 9999 /s2
}

@test "test request header manipulation" {
  run retry_default curl -s -f \
    -H "Host: router.ingress.consul" \
    -H "X-Existing-1: original" \
    -H "X-Existing-2: original" \
    -H "X-Bad-Req: true" \
    "localhost:9999/s2/debug?env=dump"

  echo "GOT: $output"

  [ "$status" == "0" ]

  # Should have been routed to the right server
  echo "$output" | grep -E "^FORTIO_NAME=s2"

  # Ingress should have added the new request header
  echo "$output" | grep -E "^X-Foo: bar-req"

  # Ingress should have appended the first existing header - both should be
  # present
  echo "$output" | grep -E "^X-Existing-1: original,appended-req"

  # Ingress should have replaced the second existing header
  echo "$output" | grep -E "^X-Existing-2: replaced-req"

  # Ingress should have set the client ip from dynamic Envoy variable
  echo "$output" | grep -E "^X-Client-Ip: 127.0.0.1"

  # Ingress should have removed the bad request header
  if echo "$output" | grep -E "^X-Bad-Req: true"; then
    echo "X-Bad-Req request header should have been stripped but was still present"
    exit 1
  fi
}

@test "test response header manipulation" {
  # Add a response header that should be stripped by the route.
  run retry_default curl -v -s -f -X PUT \
    -H "Host: router.ingress.consul" \
    "localhost:9999/s2/echo?header=x-bad-resp:true&header=x-existing-1:original&header=x-existing-2:original"

  echo "GOT: $output"

  [ "$status" == "0" ]

  # Ingress should have added the new response header
  echo "$output" | grep -E "^< x-foo: bar-resp"

  # Ingress should have appended the first existing header - both should be
  # present
  echo "$output" | grep -E "^< x-existing-1: original"
  echo "$output" | grep -E "^< x-existing-1: appended-resp"

  # Ingress should have replaced the second existing header
  echo "$output" | grep -E "^< x-existing-2: replaced-resp"
  if echo "$output" | grep -E "^< x-existing-2: original"; then
    echo "x-existing-2 response header should have been overridden, original still present"
    exit 1
  fi

  # Ingress should have removed the bad response header
  if echo "$output" | grep -E "^< x-bad-resp: true"; then
    echo "X-Bad-Resp response header should have been stripped but was still present"
    exit 1
  fi
}

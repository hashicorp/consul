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

@test "s1 upstream should have healthy endpoints for v1.s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 v1.s2.default.primary HEALTHY 1
}

### the splitter sends you to v1 or v2 but never the default
@test "s1 upstream should be able to connect to s2-v1 or s2-v2 via upstream s2" {
  assert_expected_fortio_name_pattern ^FORTIO_NAME=s2-v[12]$
}

@test "test request header manipulation" {
  run retry_default curl -s -f \
    -H "X-Bad-Req: true" \
    "localhost:5000/debug?env=dump"


  echo "GOT: $output"

  [ "$status" == "0" ]

  # Figure out which version we hit. This will fail the test if the grep can't
  # find a match while capturing the v1 or v2 from the server name in VERSION
  VERSION=$(echo "$output" | grep -o -E "^FORTIO_NAME=s2-v[12]" | grep -o 'v[12]$')

  # Route should have added the right request header
  GOT_HEADER=$(echo "$output" | grep -E "^X-Split-Leg: v[12]" | grep -o 'v[12]$')

  [ "$GOT_HEADER" == "$VERSION" ]

  # Route should have removed the bad request header
  if echo "$output" | grep -E "^X-Bad-Req: true"; then
    echo "X-Bad-Req request header should have been stripped but was still present"
    exit 1
  fi
}

@test "test response header manipulation" {
  # Add a response header that should be stripped by the route.
  run retry_default curl -v -f -X PUT \
    "localhost:5000/header-manip/echo?header=x-bad-resp:true"

  echo "GOT: $output"

  [ "$status" == "0" ]

  # Splitter should have added the right response header (this is output by curl -v)
  echo "$output" | grep -E "^< x-svc-version: v[12]"

  # Splitter should have removed the bad response header
  if echo "$output" | grep -E "^< x-bad-resp: true"; then
    echo "X-Bad-Resp response header should have been stripped but was still present"
    exit 1
  fi
}

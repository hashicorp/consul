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

@test "ingress should be able to connect to s1 using Host header" {
  assert_expected_fortio_name s1 https://s1.ingress.consul 9999
}

@test "ingress should be able to connect to s2 using Host header" {
  assert_expected_fortio_name s2 https://s2.ingress.consul 9999
}

@test "ingress should be able to connect to s1 using a user-specified Host" {
  assert_expected_fortio_name s1 https://foo.example.com 9998
}

@test "ingress should serve SDS-supplied cert for wildcard service" {
  # Make sure the Cert was the one SDS served and didn't just happen to have the
  # right domain from Connect.
  assert_cert_signed_by_ca /workdir/test-sds-server/certs/ca-root.crt \
    localhost:9999 *.ingress.consul
}

@test "ingress should serve SDS-supplied cert for specific service" {
  # Make sure the Cert was the one SDS served and didn't just happen to have the
  # right domain from Connect.
  assert_cert_signed_by_ca /workdir/test-sds-server/certs/ca-root.crt \
    localhost:9998 foo.example.com
}

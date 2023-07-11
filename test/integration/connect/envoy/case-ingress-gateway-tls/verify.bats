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

@test "ingress-gateway should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "should be able to connect to s1 through the TLS-enabled ingress port" {
  assert_dnssan_in_cert localhost:9998 '\*.ingress.consul'
  # Use the --resolve argument to fake dns resolution for now so we can use the
  # s1.ingress.consul domain to validate the cert
  cacert_curl
}

@test "should be able to connect to s1 through the TLS-enabled ingress port using the custom host" {
  assert_dnssan_in_cert localhost:9999 'test.example.com'
  cacert_curl_custom_host
}

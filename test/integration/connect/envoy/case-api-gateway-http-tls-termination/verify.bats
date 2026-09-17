#!/usr/bin/env bats

load helpers

@test "api gateway proxy admin is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "api gateway should have been accepted and not conflicted" {
  assert_config_entry_status Accepted True Accepted primary api-gateway api-gateway
  assert_config_entry_status Conflicted False NoConflict primary api-gateway api-gateway
}

@test "api gateway should have healthy endpoints for s1" {
  assert_config_entry_status Bound True Bound primary http-route api-gateway-route-one
  assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "api gateway terminates downstream HTTPS with no custom certificate configured" {
  run retry_long curl -sk -f -d hello https://localhost:9999
  [ "$status" -eq 0 ]
  [[ "$output" == *"hello"* ]]
}

@test "api gateway leaf certificate carries the *.api-gateway.consul DNS SAN" {
  # The auto-issued Connect leaf must advertise the canonical wildcard SAN so
  # the "<svc>.api-gateway.consul" name is verifiable without any manual cert.
  retry_long assert_dnssan_in_cert localhost:9999 '[*][.]api-gateway[.]consul'
}

@test "api gateway leaf certificate also carries the datacenter-scoped DNS SAN" {
  retry_long assert_dnssan_in_cert localhost:9999 '[*][.]api-gateway[.]primary[.]consul'
}

@test "api gateway zero-touch leaf verifies against the Consul Connect CA" {
  # Fetch the active Connect CA root and confirm the presented leaf chains to it,
  # i.e. the canonical endpoint is CA-verifiable with zero certificate setup.
  get_ca_root >"${BATS_TMPDIR:-/tmp}/connect-ca-root.crt"
  retry_long assert_cert_signed_by_ca "${BATS_TMPDIR:-/tmp}/connect-ca-root.crt" localhost:9999 s1.api-gateway.consul
}


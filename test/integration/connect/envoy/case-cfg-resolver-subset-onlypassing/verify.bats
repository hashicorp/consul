#!/usr/bin/env bats

load helpers

@test "s1 proxy admin is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "s2 proxy admin is up on :19001" {
  retry_default curl -f -s localhost:19001/stats -o /dev/null
}

@test "s2-v1 proxy admin is up on :19002" {
  retry_default curl -f -s localhost:19002/stats -o /dev/null
}

@test "s1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21000 s1
}

@test "s2 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21001 s2
}

@test "s2-v1 proxy listener should be up and have right cert" {
  assert_proxy_presents_cert_uri localhost:21011 s2
}

###########################
## with onlypassing=true

@test "only one s2 proxy is healthy" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have 1 healthy endpoint for test.s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 test.s2 HEALTHY 1
}

@test "s1 upstream should have 1 unhealthy endpoints for test.s2" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 test.s2 UNHEALTHY 1
}

@test "s1 upstream should be able to connect to s2" {
  assert_expected_fortio_name s2
}

###########################
## with onlypassing=false

@test "switch back to OnlyPassing=false by deleting the config" {
  delete_config_entry service-resolver s2
}

@test "only one s2 proxy is healthy (OnlyPassing=false)" {
  assert_service_has_healthy_instances s2 1
}

@test "s1 upstream should have 2 healthy endpoints for test.s2 (OnlyPassing=false)" {
  # NOTE: the subset is erased, so we use the bare name now
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2 HEALTHY 2
}

@test "s1 upstream should have 0 unhealthy endpoints for test.s2 (OnlyPassing=false)" {
  assert_upstream_has_endpoints_in_status 127.0.0.1:19000 s2 UNHEALTHY 0
}

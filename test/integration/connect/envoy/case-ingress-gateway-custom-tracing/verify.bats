#!/usr/bin/env bats

load helpers

function get_trace_count {
  local HOSTPORT=$1
  local SERVICE_NAME=$2
  local OPERATION_NAME=$3
  local len_data
  run curl -s -f "http://${HOSTPORT}/api/traces?service=${SERVICE_NAME}"
  [ "$status" -eq 0 ]

  len_data=$(echo "$output" | jq --raw-output '.data | length')

  # data is empty when there are no spans
  if [ "$len_data" = "0" ]; then
    echo "0"
  else
    echo "$output" | jq --raw-output "[.data[].spans[] | select(.operationName==\"${OPERATION_NAME}\")] | length"
  fi
}

# assume the jaeger hostport is localhost:16886 (default) and that we are looking for traces
# that have  service=unknown-service-name since we can't override that anyways.
function assert_trace_count {
  local OPERATION_NAME=$1
  local EXPECT_COUNT=$2
  local GOT_COUNT

  GOT_COUNT=$(get_trace_count localhost:16686 unknown-service-name $OPERATION_NAME)
  echo "GOT_COUNT: $GOT_COUNT, EXPECT_COUNT=$EXPECT_COUNT"
  [ "$GOT_COUNT" -eq "$EXPECT_COUNT" ]
}

@test "proxy admin endpoint is up on :20000" {
  retry_default curl -f -s localhost:20000/stats -o /dev/null
}

@test "proxy admin endpoint is up on :20001" {
  retry_default curl -f -s localhost:20001/stats -o /dev/null
}

@test "proxy admin endpoint is up on :20002" {
  retry_default curl -f -s localhost:20002/stats -o /dev/null
}

@test "proxy admin endpoint is up on :20003" {
  retry_default curl -f -s localhost:20002/stats -o /dev/null
}

@test "proxy admin endpoint is up on :19000" {
  retry_default curl -f -s localhost:19000/stats -o /dev/null
}

@test "ingress-gateway-all-0 should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20000 s1 HEALTHY 1
}

@test "ingress-gateway-client-0 should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20001 s1 HEALTHY 1
}

@test "ingress-gateway-overall-0 should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20002 s1 HEALTHY 1
}

@test "ingress-gateway-overall-100 should have healthy endpoints for s1" {
   assert_upstream_has_endpoints_in_status 127.0.0.1:20003 s1 HEALTHY 1
}

@test "all sampling values set to 0% should not send traces to zipkin/jaeger" {
  assert_trace_count localhost:9990 0

  run curl -s -f localhost:9990
  [ "$status" -eq 0 ]

  assert_trace_count localhost:9990 0

  # send with trace header, should not create a trace
  # note: these trace ids that are manually provided must be unique from
  # previous requests otherwise a trace will not be recorded
  run curl -s -f -H "x-client-trace-id:aabbcc" localhost:9990
  [ "$status" -eq 0 ]

  assert_trace_count localhost:9990 0
}

@test "client sampling set to 100% should send traces to zipkin/jaeger conditionally" {
  #sleep 9999

  assert_trace_count localhost:9991 0

  run curl -s -f localhost:9991
  [ "$status" -eq 0 ]

  assert_trace_count localhost:9991 0

  # send with trace header, should create a trace
  # note: these trace ids that are manually provided must be unique from
  # previous requests otherwise a trace will not be recorded
  run curl -s -f -H "x-client-trace-id:bbccdd" localhost:9991
  [ "$status" -eq 0 ]

  retry_long assert_trace_count localhost:9991 1
}

@test "overall sampling set to 0% should send not traces to zipkin/jaeger" {
  assert_trace_count localhost:9992 0

  run curl -s -f localhost:9992
  [ "$status" -eq 0 ]

  retry_long assert_trace_count localhost:9992 0

  # send with trace header, should create not create a trace
  # note: these trace ids that are manually provided must be unique from
  # previous requests otherwise a trace will not be recorded
  run curl -s -f -H "x-client-trace-id:ccddee" localhost:9992
  [ "$status" -eq 0 ]

  assert_trace_count localhost:9992 0
}

@test "only overall sampling set to 100% should send not traces to zipkin/jaeger" {
  assert_trace_count localhost:9992 0

  run curl -s -f localhost:9993
  [ "$status" -eq 0 ]

  retry_long assert_trace_count localhost:9993 0

  # send with trace header, should create not create a trace
  # note: these trace ids that are manually provided must be unique from
  # previous requests otherwise a trace will not be recorded
  run curl -s -f -H "x-client-trace-id:ddeeff" localhost:9993
  [ "$status" -eq 0 ]

  assert_trace_count localhost:9993 0
}
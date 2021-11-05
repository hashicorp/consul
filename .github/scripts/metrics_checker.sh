#!/usr/bin/env bash

# search for any "new" or modified metric emissions
metrics_modified=$(git diff | grep -i "SetGauge\|EmitKey\|IncrCounter\|AddSample\|MeasureSince\|UpdateFilter")
if [ -n "$metrics_modified" ]; then
  # need to check if there are modifications to metrics_test
  modified_metrics_test_file=$(git diff-index --name-only --diff-filter=d HEAD | grep -i "metrics_test")
  if [ -n "$modified_metrics_test_file" ]; then
    # 1 happy path: metrics_test has been modified bc we modified metrics behavior
    echo "PR seems to modify metrics behavior. It seems it has modified agent/metrics_test.go as well."
    exit 0
  else
    echo "PR seems to modify metrics behavior. It seems no tests or test behavior has been modified in agent/metrics_test.go."
    echo "TODO: update PR or skip"
    exit 1
  fi

else
  # no metrics modified in code, all good
  echo "No metric behavior seems to be modified."
  exit 0
fi
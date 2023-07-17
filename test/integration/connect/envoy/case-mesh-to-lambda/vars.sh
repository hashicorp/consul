#!/bin/bash

# Ensure that the environment variables required to configure and invoke the Lambda function are present, otherwise skip.
# Note that `set | grep ...` is used here because we cannot check the vars directly. If they are unbound the test will
# fail instead of being skipped.
export SKIP_CASE=""
[ -n "$(set | grep '^AWS_LAMBDA_REGION=')" ] || export SKIP_CASE="AWS_LAMBDA_REGION is not present in the environment"
[ -n "$(set | grep '^AWS_LAMBDA_ARN=')" ] || export SKIP_CASE="AWS_LAMBDA_ARN is not present in the environment"
[ -n "$(set | grep '^AWS_SESSION_TOKEN=')" ] || export SKIP_CASE="AWS_SESSION_TOKEN is not present in the environment"
[ -n "$(set | grep '^AWS_SECRET_ACCESS_KEY=')" ] || export SKIP_CASE="AWS_SECRET_ACCESS_KEY is not present in the environment"
[ -n "$(set | grep '^AWS_ACCESS_KEY_ID=')" ] || export SKIP_CASE="AWS_ACCESS_KEY_ID is not present in the environment"

[ -n "$SKIP_CASE" ] && return 0

export REQUIRED_SERVICES="s1 s1-sidecar-proxy terminating-gateway-primary"

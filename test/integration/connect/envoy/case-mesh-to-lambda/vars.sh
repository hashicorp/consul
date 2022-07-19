#!/bin/bash

export SKIP_CASE=""

# Ensure that the environment variables required to configure the Lambda function are present, otherwise skip.
[ -n "$(set | grep '^AWS_LAMBDA_REGION=')" ] || export SKIP_CASE="Lambda region (AWS_LAMBDA_REGION) is not present in the environment"
[ -n "$(set | grep '^AWS_LAMBDA_ARN=')" ] || export SKIP_CASE="Lambda function ARN (AWS_LAMBDA_ARN) is not present in the environment"

# Envoy needs AWS credentials to invoke Lambda functions so skip this case if there are no AWS credentials in the environment.
[ -n "$(set | grep '^AWS_SECRET_ACCESS_KEY=')" ] || export SKIP_CASE="AWS credentials (AWS_SECRET_ACCESS_KEY) not present in the environment"
[ -n "$(set | grep '^AWS_ACCESS_KEY_ID=')" ] || export SKIP_CASE="AWS credentials (AWS_ACCESS_KEY_ID) not present in the environment"

[ -n "$SKIP_CASE" ] && return 0

export REQUIRED_SERVICES="s1 s1-sidecar-proxy terminating-gateway-primary"

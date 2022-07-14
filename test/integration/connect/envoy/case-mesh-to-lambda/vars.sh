#!/bin/bash

# Envoy needs AWS credentials to invoke Lambda functions so skip this case if there are no AWS credentials in the environment.
# Note: we can't check the vars directly because they will be unbound if not present and the case will fail instead of skipping.
[ -n "$(set | grep '^AWS_SECRET_ACCESS_KEY=')" ] || export SKIP_CASE="AWS credentials (AWS_SECRET_ACCESS_KEY) not present in the environment"
[ -n "$(set | grep '^AWS_ACCESS_KEY_ID=')" ] || export SKIP_CASE="AWS credentials (AWS_ACCESS_KEY_ID) not present in the environment"

export REQUIRED_SERVICES="s1 s1-sidecar-proxy terminating-gateway-primary"

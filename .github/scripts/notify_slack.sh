#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -uo pipefail

# This script is used in GitHub Actions pipelines to notify Slack of a job failure.
GITHUB_ENDPOINT="https://github.com/${GITHUB_REPOSITORY}/commit/${GITHUB_SHA}"
GITHUB_ACTIONS_ENDPOINT="https://github.com/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
COMMIT_MESSAGE=$(git log -1 --pretty=%B | head -n1)
SHORT_REF=$(git rev-parse --short "${GITHUB_SHA}")
curl -X POST -H 'Content-type: application/json' \
  --data \
  "{ \
\"attachments\": [ \
  { \
  \"fallback\": \"GitHub Actions workflow failed!\", \
  \"text\": \"❌ Failed: \`${GITHUB_ACTOR}\`'s <${GITHUB_ACTIONS_ENDPOINT}> workflow failed for commit <${GITHUB_ENDPOINT}|${SHORT_REF}> on \`${GITHUB_REF_NAME}\`!\n\n- <${COMMIT_MESSAGE}\", \
  \"footer\": \"${GITHUB_REPOSITORY}\", \
  \"ts\": \"$(date +%s)\", \
  \"color\": \"danger\" \
  } \
] \
}" "${SLACK_WEBHOOK_URL}"

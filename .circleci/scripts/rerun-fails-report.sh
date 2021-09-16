#!/usr/bin/env bash
#
# Add a comment on the github PR if there were any rerun tests.
#
set -eu -o pipefail

# Don't report rerun tests on non-PRs. CircleCI sets a CIRCLE_PULL_REQUEST variable when a PR
# is created. https://circleci.com/docs/2.0/env-vars/#built-in-environment-variables
if [ -z "${CIRCLE_PULL_REQUEST-}" ]; then
  echo "CIRCLE_PULL_REQUEST isn't set. Not posting rerun test report for non-PRs."
  exit 0
fi

report_filename="${1?report filename is required}"
if [ ! -s "$report_filename" ]; then
    echo "gotestsum rerun report file is empty or missing"
    exit 0
fi

function pr_id {
    resp=$(curl -f -s \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      "https://api.github.com/search/issues?q=repo:${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}+sha:${CIRCLE_SHA1}")
    pr_url=$(echo "$resp" | jq --raw-output '.items[].pull_request.html_url')
    basename "$pr_url"
}

function report {
    echo ":repeat: gotestsum re-ran some tests in $CIRCLE_BUILD_URL"
    echo
    echo '```'
    cat "$report_filename"
    echo '```'
}

cat "$report_filename"

pr_id="$(pr_id)"
body=$(jq --null-input --arg body "$(report)" '{body: $body}')

curl -f -s -S \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Content-Type: application/json" \
  -X POST \
  --data "$body" \
  "https://api.github.com/repos/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}/issues/${pr_id}/comments"

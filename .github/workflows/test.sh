#!/bin/bash

# Set these variables for your test
#GH_PAT="your_github_token"  # Must have repo:repo scope
REPO="hashicorp/consul"
PR_NUMBER="22705"
EVENT_TYPE="oss-test"

response=$(curl -s -H "Authorization: token $GH_PAT" "https://api.github.com/repos/$REPO/pulls/$PR_NUMBER/reviews")
echo "Raw response:"
echo "$response"
echo "$response" | jq -c '[.[] | select(.state=="APPROVED") | .user.login] | unique'

reviewers=$(curl -s -H "Authorization: token $GH_PAT" \
            "https://api.github.com/repos/$REPO/pulls/$PR_NUMBER/reviews" | \
            jq -c '[.[] | select(.state=="APPROVED") | .user.login] | unique')
          echo "approved_reviewers=$reviewers" >> $GITHUB_OUTPUT
          echo "Approved reviewers: $reviewers"

# Example payload values
GIT_REF="main"
GIT_SHA="deadbeef12345678"
GIT_ACTOR="your-username"
PR_TITLE="Test PR title"
PR_BODY="## Backport
  
  This PR is auto-generated from #22702 to be assessed for backporting due to the inclusion of the label backport/1.21.
  
  
  
  The below text is copied from the body of the original PR.
  
  ---
  
  _Original PR had no description content._
  
  ---
  
  <details>
  <summary> Overview of commits </summary>
  
   
    - ad1734f1ed3e7ac0bb4baade90880710a2601273
   
  
  </details>"
PR_NUMBER="12345"
PR_APPROVERS=$reviewers 

# Build the JSON payload
PAYLOAD=$(jq -n \
  --arg event_type "$EVENT_TYPE" \
  --arg git_ref "$GIT_REF" \
  --arg git_sha "$GIT_SHA" \
  --arg git_actor "$GIT_ACTOR" \
  --arg pr_title "$PR_TITLE" \
  --arg pr_body "$PR_BODY" \
  --argjson pr_approvers "$PR_APPROVERS" \
  --arg pr_number "$PR_NUMBER" \
  '{
    event_type: $event_type,
    client_payload: {
      "git-ref": $git_ref,
      "git-sha": $git_sha,
      "git-actor": $git_actor,
      "title": $pr_title,
      "description": $pr_body,
      "pr_approvers": $pr_approvers,
      "pr_number": $pr_number
    }
  }'
)

echo "Payload to be sent:"
echo "$PAYLOAD"

echo "--- CURL REQUEST/RESPONSE ---"
# Send the dispatch event
curl -v -X POST \
  -H "Authorization: token $GH_PAT" \
  -H "Accept: application/vnd.github+json" \
  -d "$PAYLOAD" \
  "https://api.github.com/repos/$REPO/dispatches"
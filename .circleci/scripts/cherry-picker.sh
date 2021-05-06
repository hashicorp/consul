#!/usr/bin/env bash
#
# This script is meant to run on every new commit to master in CircleCI. If the commit comes from a PR, it will
# check the PR associated with the commit for labels. If the label matches `docs*` it will be cherry-picked
# to stable-website. If the label matches `backport/*`, it will be cherry-picked to the appropriate `release/*`
# branch.

# Requires $CIRCLE_PROJECT_USERNAME, $CIRCLE_PROJECT_REPONAME, and $CIRCLE_SHA1 from CircleCI

set -e -o pipefail

# colorized status prompt
function status {
    tput setaf 4
    echo "$@"
    tput sgr0
}

# This function will do the cherry-picking of a commit on a branch
# Exit 1 if cherry-picking fails
function cherry_pick_with_slack_notification {
    # Arguments:
    #   $1 - branch to cherry-pick to
    #   $2 - commit to cherry-pick
    #   $3 - url to PR of commit

    local branch="$1"
    local commit="$2"
    local pr_url="$3"

    git checkout "$branch" || exit 1
    # If git cherry-pick fails, we send a failure notification
    if ! git cherry-pick --mainline 1 "$commit"; then
        status "🍒❌ Cherry pick of commit ${commit:0:7} from $pr_url onto $branch failed!"
        curl -X POST -H 'Content-type: application/json' \
        --data \
        "{ \
        \"attachments\": [ \
            { \
            \"fallback\": \"Cherry pick failed!\", \
            \"text\": \"🍒❌ Cherry picking of <$pr_url|${commit:0:7}> to \`$branch\` failed!\n\nBuild Log: ${CIRCLE_BUILD_URL}\", \
            \"footer\": \"${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}\", \
            \"ts\": \"$(date +%s)\", \
            \"color\": \"danger\" \
            } \
        ] \
        }" "${CONSUL_SLACK_WEBHOOK_URL}"
        git status
        exit 1
    # Else we send a success notification
    else
        status "🍒✅ Cherry picking of PR commit ${commit:0:7} from $pr_url succeeded!"
        # push changes to the specified branch
        git push origin "$branch"
        curl -X POST -H 'Content-type: application/json' \
        --data \
        "{ \
        \"attachments\": [ \
            { \
            \"fallback\": \"Cherry pick succeeded!\", \
            \"text\": \"🍒✅ Cherry picking of <$pr_url|${commit:0:7}> to \`$branch\` succeeded!\n\nBuild Log: ${CIRCLE_BUILD_URL}\", \
            \"footer\": \"${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}\", \
            \"ts\": \"$(date +%s)\", \
            \"color\": \"good\" \
            } \
        ] \
        }" "${CONSUL_SLACK_WEBHOOK_URL}"
    fi
}

# search for the PR labels applicable to the specified commit
resp=$(curl -f -s -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/search/issues?q=repo:$CIRCLE_PROJECT_USERNAME/$CIRCLE_PROJECT_REPONAME+sha:$CIRCLE_SHA1")
ret="$?"
if [[ "$ret" -ne 0 ]]; then
    status "The GitHub API returned $ret which means it was probably rate limited."
    exit $ret
fi

# get the count from the GitHub API to check if the commit matched a PR
count=$(echo "$resp" | jq '.total_count')
if [[ "$count" -eq 0 ]]; then
    status "This commit was not associated with a PR"
    exit 0
fi

# If the API returned a non-zero count, we have found a PR with that commit so we find
# the labels from the PR
labels=$(echo "$resp" | jq --raw-output '.items[].labels[] | .name')
ret="$?"
pr_url=$(echo "$resp" | jq --raw-output '.items[].pull_request.html_url')
if [[ "$ret" -ne 0 ]]; then
    status "jq exited with $ret when trying to find label names. Are there labels applied to the PR ($pr_url)?"
    # This can be a valid error but usually this means we do not have any labels so it doesn't signal
    # cherry-picking is possible. Exit 0 for now unless we run into cases where these failures are important.
    exit 0
fi

git config --local user.email "github-team-consul-core@hashicorp.com"
git config --local user.name "hc-github-team-consul-core"

# loop through all labels on the PR
for label in $labels; do
    status "checking label: $label"
    # TODO: enable this when replatform is merged into stable-website
    # if the label matches docs-cherrypick, it will attempt to cherry-pick to stable-website
    # if [[ $label == docs-cherrypick ]]; then
    #     status "backporting to stable-website"
    #     branch="stable-website"
    #     cherry_pick_with_slack_notification "$branch" "$CIRCLE_SHA1" "$pr_url"
    # else if the label matches backport/*, it will attempt to cherry-pick to the release branch
    if [[ $label =~ backport/* ]]; then
        status "backporting to $label"
        branch="${label/backport/release}.x"
        cherry_pick_with_slack_notification "$branch" "$CIRCLE_SHA1" "$pr_url"
    fi
done

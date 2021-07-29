#!/usr/bin/env bash
#
# This script is meant to run on every new commit to main in CircleCI. If the commit comes from a PR, it will
# check the PR associated with the commit for labels. If the label matches `docs*` it will be cherry-picked
# to stable-website. If the label matches `backport/*`, it will be cherry-picked to the appropriate `release/*`
# branch.

# Requires $CIRCLE_PROJECT_USERNAME, $CIRCLE_PROJECT_REPONAME, and $CIRCLE_SHA1 from CircleCI

set -o pipefail

# colorized status prompt
function status {
    tput setaf 4
    echo "$@"
    tput sgr0
}

# Returns the latest GitHub "backport/*" label
function get_latest_backport_label {
    local resp
    local ret
    local latest_backport_label

    resp=$(curl -f -s -H "Authorization: token ${GITHUB_TOKEN}" "https://api.github.com/repos/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}/labels?per_page=100")
    ret="$?"
    if [[ "$ret" -ne 0 ]]; then
        status "The GitHub API returned $ret which means it was probably rate limited."
        exit $ret
    fi

    latest_backport_label=$(echo "$resp" | jq -r '.[] | select(.name | startswith("backport/")) | .name' | sort -rV | head -n1)
    echo "$latest_backport_label"
    return 0
}

# This function will do the cherry-picking of a commit on a branch
# Exit 1 if cherry-picking fails
function cherry_pick_with_slack_notification {
    # Arguments:
    #   $1 - branch to cherry-pick to
    #   $2 - commit to cherry-pick
    #   $3 - url to PR of commit
    #
    # Return:
    #   0 for success
    #   1 for error

    local branch="$1"
    local commit="$2"
    local pr_url="$3"

    git checkout "$branch" || exit 1
    # If git cherry-pick fails or it fails to push, we send a failure notification
    if ! (git cherry-pick --mainline 1 "$commit" && git push origin "$branch"); then
        status "🍒❌ Cherry pick of commit ${commit:0:7} from $pr_url onto $branch failed!"

        # send slack notification
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

        # post PR comment to GitHub
        github_message=":cherries::x: Cherry pick of commit ${commit} onto \`$branch\` failed! [Build Log]($CIRCLE_BUILD_URL)"
        pr_id=$(basename ${pr_url})
        curl -f -s -H "Authorization: token ${GITHUB_TOKEN}" \
             -X POST \
             -d "{ \"body\": \"${github_message}\"}" \
             "https://api.github.com/repos/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}/issues/${pr_id}/comments"

        # run git status to leave error in CircleCI log
        git status
        return 1

    # Else we send a success notification
    else
        status "🍒✅ Cherry picking of PR commit ${commit:0:7} from ${pr_url} succeeded!"
        curl -X POST -H 'Content-type: application/json' \
        --data \
        "{ \
        \"attachments\": [ \
            { \
            \"fallback\": \"Cherry pick succeeded!\", \
            \"text\": \"🍒✅ Cherry picking of <$pr_url|${commit:0:7}> to \`$branch\` succeeded!\", \
            \"footer\": \"${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}\", \
            \"ts\": \"$(date +%s)\", \
            \"color\": \"good\" \
            } \
        ] \
        }" "${CONSUL_SLACK_WEBHOOK_URL}"

        # post PR comment to GitHub
        github_message=":cherries::white_check_mark: Cherry pick of commit ${commit} onto \`$branch\` succeeded!"
        pr_id=$(basename ${pr_url})
        curl -f -s -H "Authorization: token ${GITHUB_TOKEN}" \
             -X POST \
             -d "{ \"body\": \"${github_message}\"}" \
             "https://api.github.com/repos/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}/issues/${pr_id}/comments"
    fi

    return 0
}

# search for the PR labels applicable to the specified commit
resp=$(curl -f -s -H "Authorization: token ${GITHUB_TOKEN}" "https://api.github.com/search/issues?q=repo:${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}+sha:${CIRCLE_SHA1}")
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

# save PR number
pr_number=$(echo "$resp" | jq '.items[].number')

# comment on the PR with the build number to make it easy to re-run the job when
# cherry-pick labels are added in the future
github_message=":cherries: If backport labels were added before merging, cherry-picking will start automatically.\n\nTo retroactively trigger a backport after merging, add backport labels and re-run ${CIRCLE_BUILD_URL}."
curl -f -s -H "Authorization: token ${GITHUB_TOKEN}" \
    -X POST \
    -d "{ \"body\": \"${github_message}\"}" \
    "https://api.github.com/repos/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}/issues/${pr_number}/comments"



# If the API returned a non-zero count, we have found a PR with that commit so we find
# the labels from the PR

# Sorts the labels from a PR via version sort
labels=$(echo "$resp" | jq --raw-output '.items[].labels[] | .name' | sort -rV)
ret="$?"
pr_url=$(echo "$resp" | jq --raw-output '.items[].pull_request.html_url')
if [[ "$ret" -ne 0 ]]; then
    status "jq exited with $ret when trying to find label names. Are there labels applied to the PR ($pr_url)?"
    # This can be a valid error but usually this means we do not have any labels so it doesn't signal
    # cherry-picking is possible. Exit 0 for now unless we run into cases where these failures are important.
    exit 0
fi

# Attach label for latest release branch if 'docs-cherrypick' is present. Will noop if already applied.
latest_backport_label=$(get_latest_backport_label)
status "latest backport label is $latest_backport_label"
if echo "$resp" | jq -e '.items[].labels[] | select(.name | contains("docs-cherrypick"))'; then
    labels=$(curl -f -s -H "Authorization: token ${GITHUB_TOKEN}" -X POST -d "{\"labels\":[\"$latest_backport_label\"]}" "https://api.github.com/repos/${CIRCLE_PROJECT_USERNAME}/${CIRCLE_PROJECT_REPONAME}/issues/${pr_number}/labels" | jq --raw-output '.[].name' | sort -rV)
    ret="$?"
    if [[ "$ret" -ne 0 ]]; then
        status "Error applying $latest_backport_label to $pr_url"
        exit $ret
    fi
fi

git config --local user.email "github-team-consul-core@hashicorp.com"
git config --local user.name "hc-github-team-consul-core"

backport_failures=0
# loop through all labels on the PR
for label in $labels; do
    status "checking label: $label"
    # if the label matches docs-cherrypick, it will attempt to cherry-pick to stable-website
    if [[ $label =~ docs-cherrypick ]]; then
        status "backporting to stable-website"
        branch="stable-website"
        cherry_pick_with_slack_notification "$branch" "$CIRCLE_SHA1" "$pr_url"
        backport_failures=$((backport_failures + "$?"))
    # else if the label matches backport/*, it will attempt to cherry-pick to the release branch
    elif [[ $label =~ backport/* ]]; then
        status "backporting to $label"
        branch="${label/backport/release}.x"
        cherry_pick_with_slack_notification "$branch" "$CIRCLE_SHA1" "$pr_url"
        backport_failures=$((backport_failures + "$?"))
    fi
    # reset the working directory for the next label
    git reset --hard
done

if [ "$backport_failures" -ne 0 ]; then
    echo "$backport_failures backports failed"
    exit 1
fi

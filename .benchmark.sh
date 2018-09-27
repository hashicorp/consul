#!/usr/bin/env bash

set -e +o pipefail

if [ "$TRAVIS_PULL_REQUEST" != "false" ] ; then
    echo -e "NOTE: The CPU benchmarks are performed on Travis VMs and vary widly between runs," > .benchmark.body
    echo -e " you can't trust them. The memory benchmarks are OK\n\n" >> .benchmark.body
    awk '/^benchmark.*old/ { printf "%s\n%s\n", "```", $0 };
         /^$/ { print "```" };
         /^Bench/ { print $0 };
         END{ print "```" }' .benchmark.log >> .benchmark.body
    jq -n --arg body "$(cat .benchmark.body)" '{body: $body}' > .benchmark.json
    curl -H "Authorization: token ${GITHUB_TOKEN}" -X POST \
        --data-binary "@.benchmark.json" \
        "https://api.github.com/repos/${TRAVIS_REPO_SLUG}/issues/${TRAVIS_PULL_REQUEST}/comments"
fi

#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

unset CDPATH

cd "$(dirname "$0")" # build-support/scripts
cd ../..             # <ROOT>

if [[ ! -f GNUmakefile ]] || [[ ! -f go.mod ]]; then
    echo "not in root consul checkout: ${PWD}" >&2
    exit 1
fi

GIT_BRANCH="${GIT_BRANCH:-main}"

readonly oss_branch="oss/${GIT_BRANCH}"
readonly ent_branch="origin/${GIT_BRANCH}"

echo "=============="
echo "OSS ${GIT_BRANCH}: $(git show-ref "${oss_branch}")"
echo "ENT ${GIT_BRANCH}: $(git show-ref "${ent_branch}")"
echo "=============="

# compute files in oss
readonly oss_files=$(git ls-tree --name-only -r "${oss_branch}")

set +e

echo "Changelog differences (all changelog entries should be in oss and synced to enterprise):"
echo "=============="
git diff "${oss_branch}..${ent_branch}" --numstat -- ':.changelog'
echo "=============="

echo "Files that are different in ENT than in OSS:"
echo "  git diff ${oss_branch}..${ent_branch} --numstat -- [elided]"
echo "=============="
git diff "${oss_branch}..${ent_branch}" --numstat -- ${oss_files} \
    ':!.github' \
    ':!*/.gitignore' \
    ':!.gitignore' \
    ':!.release' \
    ':!build-support' \
    ':!Dockerfile' \
    ':!GNUmakefile' \
    ':!*/go.mod' \
    ':!*/go.sum' \
    ':!go.mod' \
    ':!go.sum'
echo "=============="

echo "Actual diff follows:"
echo "  git diff ${oss_branch}..${ent_branch} -- [elided]"
echo "=============="
git diff "${oss_branch}..${ent_branch}" -- ${oss_files} \
    ':!.github' \
    ':!*/.gitignore' \
    ':!.gitignore' \
    ':!.release' \
    ':!build-support' \
    ':!Dockerfile' \
    ':!GNUmakefile' \
    ':!*/go.mod' \
    ':!*/go.sum' \
    ':!go.mod' \
    ':!go.sum'
exit 0

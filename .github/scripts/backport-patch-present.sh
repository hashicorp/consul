#!/bin/sh
# Copyright IBM Corp. 2026
# SPDX-License-Identifier: BUSL-1.1

# Read-only probe: 0 = present, 1 = not proven present, 2 = unsupported source,
# 3 = error. Objects may be written, but no checkout, ref or remote is modified.
set -eu

if [ "$#" -ne 3 ]; then
  echo "Usage: $0 REPOSITORY SOURCE_COMMIT TARGET_COMMIT" >&2
  exit 3
fi

cd "$1"
source_commit=$(git rev-parse --verify "$2^{commit}") || exit 3
target_commit=$(git rev-parse --verify "$3^{commit}") || exit 3
parents=$(git show -s --format=%P "$source_commit") || exit 3
case "$parents" in
  ""|*" "*)
    echo "Root/merge commits require manual review."
    exit 2
    ;;
esac

scratch=$(mktemp -d) || exit 3
cleanup() {
  status=$?
  trap - EXIT
  rm -f "$scratch/merge" "$scratch/merge.log" || status=3
  rmdir "$scratch" || status=3
  exit "$status"
}
trap cleanup EXIT
trap 'exit 3' HUP INT TERM

source_tree=$(git rev-parse "$source_commit^{tree}") || exit 3
parent_tree=$(git rev-parse "$parents^{tree}") || exit 3
if [ "$source_tree" = "$parent_tree" ]; then
  echo "Source commit has no tree changes."
  exit 0
fi

target_tree=$(git rev-parse "$target_commit^{tree}") || exit 3
# An explicit base simulates a cherry-pick, even when the source is an ancestor
# of the target and was subsequently reverted. Requires Git 2.40 or newer.
# Reverse-apply alone is not proof: a hunk can match an unrelated repeated block.
merge_status=0
git merge-tree --write-tree --merge-base="$parents" "$target_commit" "$source_commit" \
  >"$scratch/merge" 2>"$scratch/merge.log" || merge_status=$?
case "$merge_status" in
  0)
    IFS= read -r merged_tree <"$scratch/merge"
    if [ "$merged_tree" = "$target_tree" ]; then
      echo "The cherry-pick produces no changes."
      exit 0
    fi
    ;;
  1) ;; # Content conflicts are inconclusive, not evidence of presence.
  *)
    cat "$scratch/merge.log" >&2
    echo "Unable to compare the source patch with the target." >&2
    exit 3
    ;;
esac
echo "The complete patch is not proven present; backport or manual review is required."
exit 1

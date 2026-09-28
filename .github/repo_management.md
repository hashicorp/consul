# Consul GitHub Configuration

## Overview

This file helps track the configuration of the `.github/` folder.

## Issue Templates

Issue templates are stored in `.github/ISSUE_TEMPLATE/` and follow the
[documentation](https://docs.github.com/en/github/building-a-strong-community/using-templates-to-encourage-useful-issues-and-pull-requests).
The `.github/ISSUE_TEMPLATE/config.yml` controls links out to other support
resources.

## GitHub Actions

GitHub Actions provides a pluggable architecture for creating simple automation.
An Action is made of at least two files, the `workflow` file and a config file.
All workflows are stored in `.github/workflows/`. Configuration files are stored
one directory higher, in `.github/`. The workflow and the configuration file
should be named the same when created. Create unique and clear names for these
files.

### Issue Labeler

Issues are labeled with
[RegEx Labeler](https://github.com/marketplace/actions/regex-issue-labeler).
This action supports simple regexes, and most string parsing.

### PR Labeler

PRs are labeled with [labeler](https://github.com/actions/labeler) action.
This supports glob parsing so that labels can be applied to changed files.

### CE release backports

`.github/workflows/backport-assistant.yml` runs the local
`.github/scripts/ce-backport` preflight before delegating PR creation to the
existing `hashicorpdev/backport-assistant:v0.5.8` image. It processes merged PRs
into the repository's default branch; it does not introduce release-to-release
backports. The separate Enterprise dispatch job is unchanged.

The guard fetches the latest source PR labels and the default branch's
`.release/versions.hcl`. Only exact `backport/<major>.<minor>` labels whose version
is CE-active are considered. `lts` or an Enterprise-only version does not enable
CE backports. Label events reconcile all current CE-active labels,
because per-source-PR workflow concurrency can coalesce queued events.
`backport/all` still expands CE and Enterprise labels through the pinned
assistant, using a label-only invocation; every subsequent CE PR creation is
gated separately.

For each requested target, the preflight skips creation when a same-repository
open backport with the pinned creator's source-reference body already exists,
or when the entire source change is proven present in the current target tree.
It uses `.github/scripts/backport-patch-present.sh` (also used in Enterprise):
an explicit-source-parent three-way merge must produce the unchanged target
tree. An ancestor SHA, historical patch ID, or reverse-applied hunk alone is
not proof, particularly after reverts or with repeated code blocks. Unrelated
target changes are allowed; partial or conflicting changes are not skipped.

Before content comparison or creation, the aggregate source PR diff must match
the source merge commit's patch. This guards against a multi-commit rebase merge
whose merge SHA represents only its final commit. Unsupported root/merge commits,
incomplete source coverage, and API/Git failures fail visibly rather than
silently skipping work or creating a partial backport. These cases need a
manual backport or a workflow retry after resolving the reported error.

The target is refreshed before each decision, with at most three attempts if it
moves. Labels, active status, and open backports are also rechecked before
creation. A target change after the last fetch, or a separate manual creator
racing the final check, cannot be excluded atomically by this wrapper. An open
manual PR without the creator's provenance body is not inferred to be the same
backport; once merged, its content can satisfy the tree check. Substantially
reworked equivalent changes may remain inconclusive and still need review.

PR assignment, reviewer selection, and conflict-draft behavior remain with the
pinned creator. Existing PRs are not closed or modified by the guard. Execution
requires Git 2.40+, the Go version in `go.mod`, Docker, and the existing elevated
token. Only trusted default-branch tooling is checked out; Git comparisons use
an isolated temporary repository with credentials supplied through environment
configuration rather than saved on disk.

Focused local coverage (mocked GitHub/creator and temporary Git repositories):

```shell
go test ./.github/scripts/ce-backport -count=1
```

## Considered Actions

- [super-labeler-action](https://github.com/IvanFon/super-labeler-action) is an action that holds all the configuration in a single file. In setting up a basic configuration with 60 labels, the JSON config became ~1200 lines. This solution may be feaseable in the future, but wouldn't seem as scaleable. This also creates a single point of failure for the entire labeling system.

- [actions-label-commenter](https://github.com/peaceiris/actions-label-commenter) is an action that just responds based on tags, rather than tagging them as they come in. This would be helpful for responses for reoccuring types of messages.

- [top-issues-labeler](https://github.com/marketplace/actions/top-issues-labeler) labels the top ten issues based on number of :+1: 's on an inssue.

**This is a temporary README. We'll restore the old README prior to PR upstream.**

# Consul Connect

This repository is the forked repository for Consul Connect work to happen
in private prior to public release. This README will explain how to safely
use this fork, how to bring in upstream changes, etc.

## Cloning

To use this repository, clone it into your GOPATH as usual but you must
**rename `consul-connect` to `consul`** so that Go imports continue working
as usual.

## Important: Never Modify Master

**NEVER MODIFY MASTER! NEVER MODIFY MASTER!**

We want to keep the "master" branch equivalent to OSS master. This will make
rebasing easy for master. Instead, we'll use the branch `f-connect`. All
feature branches should branch from `f-connect` and make PRs against
`f-connect`.

When we're ready to merge back to upstream, we can make a single mega PR
merging `f-connect` into OSS master. This way we don't have a sudden mega
push to master on OSS.

## Creating a Feature Branch

To create a feature branch, branch from `f-connect`:

```sh
git checkout f-connect
git checkout -b my-new-branch
```

All merged Connect features will be in `f-connect`, so you want to work
from that branch. When making a PR for your feature branch, target the
`f-connect` branch as the merge target. You can do this by using the dropdowns
in the GitHub UI when creating a PR.

## Syncing Upstream

First update our local master:

```sh
# This has to happen on forked master
git checkout master

# Add upstream to OSS Consul
git remote add upstream https://github.com/hashicorp/consul.git

# Fetch it
git fetch upstream

# Rebase forked master onto upstream. This should have no changes since
# we're never modifying master.
git rebase upstream master
```

Next, update the `f-connect` branch:

```sh
git checkout f-connect
git rebase origin master
```

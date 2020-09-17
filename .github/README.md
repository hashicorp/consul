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

## Considered Actions

- [super-labeler-action](https://github.com/IvanFon/super-labeler-action) is an action that holds all the configuration in a single file. In setting up a basic configuration with 60 labels, the JSON config became ~1200 lines. This solution may be feaseable in the future, but wouldn't seem as scaleable. This also creates a single point of failure for the entire labeling system.

- [actions-label-commenter](https://github.com/peaceiris/actions-label-commenter) is an action that just responds based on tags, rather than tagging them as they come in. This would be helpful for responses for reoccuring types of messages.

- [top-issues-labeler](https://github.com/marketplace/actions/top-issues-labeler) labels the top ten issues based on number of :+1: 's on an inssue.

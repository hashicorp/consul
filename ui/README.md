# Consul UI Monorepo

This monorepo contains multiple projects, the UI for Consul and addons and
packages used by the UI.

This top-level repository provides limited common tasks, such as installation
and commit assistance.  However, most tasks must be executed from within a
subproject, e.g. running or testing.

**If you are looking to work on the Consul UI you probably want to read
the README that is in `./packages/consul-ui/README.md`.**


<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Yarn Commands](#yarn-commands)
- [Contributing](#contributing)
  - [Building ToC](#building-toc)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites

You will need the following things properly installed on your computer.

* [Git][git]
* [Node.js][node]
* [Yarn][yarn] installed globally
* [Google Chrome][chrome]
* [Firefox][firefox]

[git]: https://git-scm.com/
[node]: https://nodejs.org/
[yarn]: https://classic.yarnpkg.com/lang/en/
[chrome]: https://google.com/chrome/
[firefox]: https://firefox.com/
[yarn-workspaces]: https://classic.yarnpkg.com/en/docs/workspaces/

## Installation

* `git clone https://github.com/hashicorp/consul.git` this repository
* `cd ui`
* `yarn`

## Yarn Commands

List of available project commands.  `yarn run <command-name>`

| Command             | Description                        |
|---------------------|------------------------------------|
| doc:toc             | Re-builds the ToC for this README. |
| compliance:licenses | Checks that all dependencies have OSS-compatible licenses. |

## Contributing

### Building ToC

To autogenerate a ToC (table of contents) for this README,
run `yarn doc:toc`.  Please update the ToC whenever editing the structure
of README.

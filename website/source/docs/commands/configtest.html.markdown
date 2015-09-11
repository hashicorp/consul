---
layout: "docs"
page_title: "Commands: ConfigTest"
sidebar_current: "docs-commands-configtest"
description: >
  The `consul configtest` command tests that config files are valid by
  attempting to parse them. Useful to ensure a configuration change will
  not cause consul to fail after a restart.
---

# Consul ConfigTest

The `consul configtest` command performs a basic sanity test on Consul
configuration files. For each file or directory given, the configtest command
will attempt to parse the contents just as the "consul agent" command would,
and catch any errors. This is useful to do a test of the configuration only,
without actually starting the agent.

For more information on the format of Consul's configuration files, read the
consul agent [Configuration Files](/docs/agent/options.html#configuration_files)
section.

## Usage

Usage: `consul configtest [options]`

At least one `-config-file` or `-config-dir` parameter must be given. Returns 0
if the configuration is valid, or 1 if there are problems. The list of
available flags are:

* `-config-file` - Path to a config file. May be specified multiple times.

* `-config-dir` - Path to a directory of config files. All files ending in
  `.json` in the directory will be included. May be specified multiple times.

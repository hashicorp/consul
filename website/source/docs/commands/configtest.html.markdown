---
layout: "docs"
page_title: "Commands: ConfigTest"
sidebar_current: "docs-commands-configtest"
description: |-
  The `consul configtest` command tests that config files are valid by attempting to parse them. Useful to ensure a configuration change will not cause consul to fail after a restart.
---

# Consul ConfigTest

The `consul configtest` command tests that config files are valid by attempting to parse them. Useful to ensure a configuration change will not cause consul to fail after a restart.

## Usage

Usage: `consul configtest [options]`

At least one `-config-file` or `-config-dir` paramater must be given. The list of available flags are:

- `-config-file` - config file. may be specified multiple times 
- `-config-dir` - config directory. all files ending in `.json` in the directory will be included. may be specified multiple times.

* <a name="config_file"></a> `-config-file` - A configuration file
  to load. For more information on
  the format of this file, read the [Configuration Files](/docs/agent/options.html#configuration_files) section in the agent option documentation.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier. During a config merge,
  single-value keys (string, int, bool) will simply have their values replaced
  while list types will be appended together.

* `-config-dir` - A directory of
  configuration files to load. Consul will
  load all files in this directory with the suffix ".json". The load order
  is alphabetical, and the the same merge routine is used as with the
  [`config-file`](#_config_file) option above. For more information
  on the format of the configuration files, see the [Configuration Files](/docs/agent/options.html#configuration_files) section in the agent option documentation.

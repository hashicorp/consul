---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-index"
description: |-
  The agent has various configuration options that can be specified via the command-line or via configuration files. All of the configuration options are completely optional. Defaults are specified with their descriptions.
---

# Configuration

The agent has various configuration options that can be specified via
the command-line or via configuration files. All of the configuration
options are completely optional. Defaults are specified with their
descriptions.

Configuration precedence is evaluated in the following order:

1. Command line arguments
2. Environment Variables
3. Configuration files

When loading configuration, Consul loads the configuration from files and
directories in lexical order. For example, configuration file
`basic_config.json` will be processed before `extra_config.json`. Configuration
can be in either [HCL](https://github.com/hashicorp/hcl#syntax) or JSON format.
Available in Consul 1.0 and later, the HCL support now requires an `.hcl` or
`.json` extension on all configuration files in order to specify their format.

Configuration specified later will be merged into configuration specified
earlier. In most cases, "merge" means that the later version will override the
earlier. In some cases, such as event handlers, merging appends the handlers to
the existing configuration. The exact merging behavior is specified for each
option below.

Consul also supports reloading configuration when it receives the
SIGHUP signal. Not all changes are respected, but those that are
documented below in the
[Reloadable Configuration](#reloadable-configuration) section. The
[reload command](/docs/commands/reload.html) can also be used to trigger a
configuration reload.

## Example Configuration File

```javascript
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true,
  "watches": [
    {
        "type": "checks",
        "handler": "/usr/bin/health-check-handler.sh"
    }
  ],
  "telemetry": {
     "statsite_address": "127.0.0.1:2180"
  }
}
```

## Example Configuration File, with TLS

```javascript
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true,
  "addresses": {
    "https": "0.0.0.0"
  },
  "ports": {
    "https": 8501
  },
  "key_file": "/etc/pki/tls/private/my.key",
  "cert_file": "/etc/pki/tls/certs/my.crt",
  "ca_file": "/etc/pki/tls/certs/ca-bundle.crt"
}
```

See, especially, the use of the `ports` setting:

```javascript
"ports": {
  "https": 8501
}
```
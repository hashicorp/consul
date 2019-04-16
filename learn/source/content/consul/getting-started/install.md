---
name: 'Install Consul'
content_length: 4
id: install
layout: content_layout
products_used:
  - Consul
description: |-
  A Consul agent must be installed on every node in the Consul cluster, in this guide we will install one Consul agent locally to explore the core set of capabilities.
level: Beginner
wistia_video_id: pufo7fm8eu
---

Consul must first be installed on your machine. Consul is distributed as a
[binary package](https://www.consul.io/downloads.html) for all supported platforms and architectures.
This page will not cover how to compile Consul from source, but compiling from
source is covered in the [documentation](https://www.consul.io/docs/install/index.html#compiling-from-source) for those who want to
be sure they're compiling source they trust into the final binary.

## Installing Consul

To install Consul, find the [appropriate package](https://www.consul.io/downloads.html) for
your system and download it. Consul is packaged as a zip archive.

After downloading Consul, unzip the package. Consul runs as a single binary
named `consul`. Any other files in the package can be safely removed and
Consul will still function.

The final step is to make sure that the `consul` binary is available on the `PATH`.
See [this page](https://stackoverflow.com/questions/14637979/how-to-permanently-set-path-on-linux)
for instructions on setting the PATH on Linux and Mac.
[This page](https://stackoverflow.com/questions/1618280/where-can-i-set-path-to-make-exe-on-windows)
contains instructions for setting the PATH on Windows.

## Verifying the Installation

After installing Consul, verify the installation worked by opening a new
terminal session and checking that `consul` is available. By executing
`consul` you should see help output similar to this:

```text
$ consul
usage: consul [--version] [--help] <command> [<args>]

Available commands are:
    agent          Runs a Consul agent
    event          Fire a new event

# ...
```

If you get an error that `consul` could not be found, your `PATH`
environment variable was not set up properly. Please go back and ensure
that your `PATH` variable contains the directory where Consul was
installed.

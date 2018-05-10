---
name: Bug Report
about: You're experiencing an issue with Consul that is different than the documented behavior.

---

When filing a bug, please include the following headings if
possible. Any example text in this template can be deleted.

#### Overview of the Issue

A paragraph or two about the issue you're experiencing.

#### Reproduction Steps

Steps to reproduce this issue, eg:

1. Create a cluster with n client nodes n and n server nodes
1. Run `curl ...`
1. View error

### Consul info for both Client and Server

The `consul info` command provides information for us
about the agents running both as clients and servers.

Client:

```
[Client `consul info` here]
```

Server:

```
[Server `consul info` here]
```

### Operating system and Environment details

OS, Architecture, and any other information you can provide
about the environment.

### Log Fragments

Include appropriate Client or Server log fragments.
If the log is longer than a few dozen lines, please
include the URL to the [gist](https://gist.github.com/).

Use `-log-level=TRACE` on the client and server to capture the maximum log detail.

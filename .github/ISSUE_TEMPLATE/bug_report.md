---
name: Bug Report
about: You're experiencing an issue with Consul that is different than the documented behavior.

---

<!-- When filing a bug, please include the following headings if possible. Any example text in this template can be deleted.
-->

#### Overview of the Issue

<!-- Please provide a paragraph or two about the issue you're experiencing. -->

---

#### Reproduction Steps

<!-- Please provide steps to reproduce the bug, without any details it would be hard to troubleshoot: 

Steps to reproduce this issue, eg:

1. Create a cluster with n client nodes n and n server nodes
1. Run `curl ...`
1. View error

-->

### Consul info for both Client and Server


<!---  Please provide both `consul info` and agent HCL config for both client and servers to help us better diagnose the issue. Take careful steps to remove any sensitive information from config files that include secrets such as Gossip keys. --->

<details>
  <summary>Client info</summary>

```
Output from client 'consul info' command here
```

```
Client agent HCL config
```

</details>

<details>
  <summary>Server info</summary>

```
Output from server 'consul info' command here
```

```
Server agent HCL config
```

</details>

### Operating system and Environment details

<!--  OS, Architecture, and any other information you can provide about the environment. -->

### Log Fragments

<!-- Include appropriate Client or Server log fragments. If the log is longer than a few dozen lines, please include the URL to the [gist](https://gist.github.com/) of the log instead of posting it in the issue. Use `-log-level=TRACE` on the client and server to capture the maximum log detail. -->

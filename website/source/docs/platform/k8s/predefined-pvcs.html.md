---
layout: "docs"
page_title: "Predefined PVCs"
sidebar_current: "docs-platform-k8s-run-pvcs"
description: |-
  Using predefined Persistent Volume Claims
---

# Predefined Persistent Volume Claims (PVCs)

The only way to use a pre-created PVC is to name them in the format Kubernetes expects:

```
data-<kubernetes namespace>-<helm release name>-consul-server-<ordinal>
```

The Kubernetes namespace you are installing into, Helm release name, and ordinal
must match between your Consul servers and your pre-created PVCs. You only
need as many PVCs as you have Consul servers. For example, given a Kubernetes
namespace of "vault," a release name of "consul," and 5 servers, you would need
to create PVCs with the following names:

```
data-vault-consul-consul-server-0
data-vault-consul-consul-server-1
data-vault-consul-consul-server-2
data-vault-consul-consul-server-3
data-vault-consul-consul-server-4
```

If you are using your own storage, you'll need to configure a storage class. See the
documentation for configuring storage classes [here](https://kubernetes.io/docs/concepts/storage/storage-classes/).

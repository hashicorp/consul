---
layout: "docs"
page_title: "Uninstalling"
sidebar_current: "docs-platform-k8s-ops-uninstalling"
description: |-
  Uninstalling Consul on Kubernetes
---

# Uninstalling Consul
Consul can be uninstalled via the `helm delete` command:

```bash
$ helm delete hashicorp
release "hashicorp" uninstalled
```

-> If using Helm 2, run `helm delete --purge hashicorp`

After deleting the Helm release, you need to delete the `PersistentVolumeClaim`'s
for the persistent volumes that store Consul's data. These are not deleted by Helm due to a [bug](https://github.com/helm/helm/issues/5156).
To delete, run:

```bash
$ kubectl get pvc -l chart=consul-helm
NAME                                   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data-default-hashicorp-consul-server-0   Bound    pvc-32cb296b-1213-11ea-b6f0-42010a8001db   10Gi       RWO            standard       17m
data-default-hashicorp-consul-server-1   Bound    pvc-32d79919-1213-11ea-b6f0-42010a8001db   10Gi       RWO            standard       17m
data-default-hashicorp-consul-server-2   Bound    pvc-331581ea-1213-11ea-b6f0-42010a8001db   10Gi       RWO            standard       17m

$ kubectl delete pvc -l chart=consul-helm
persistentvolumeclaim "data-default-hashicorp-consul-server-0" deleted
persistentvolumeclaim "data-default-hashicorp-consul-server-1" deleted
persistentvolumeclaim "data-default-hashicorp-consul-server-2" deleted
```

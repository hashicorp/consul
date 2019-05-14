---
layout: "docs"
page_title: "Consul DNS - Kubernetes"
sidebar_current: "docs-platform-k8s-dns"
description: |-
  One of the primary query interfaces to Consul is the DNS interface. The Consul DNS interface can be exposed for all pods in Kubernetes using a stub-domain configuration.
---

# Consul DNS on Kubernetes

One of the primary query interfaces to Consul is the
[DNS interface](/docs/agent/dns.html). The Consul DNS interface can be
exposed for all pods in Kubernetes using a
[stub-domain configuration](https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#configure-stub-domain-and-upstream-dns-servers).

The stub-domain configuration must point to a static IP of a DNS resolver.
The [Helm chart](/docs/platform/k8s/helm.html) creates a `consul-dns` service
by default that exports Consul DNS. The cluster IP of this service can be used
to configure a stub-domain with kube-dns. While the `kube-dns` configuration
lives in the `kube-system` namespace, the IP just has to be routable so the
service can live in a different namespace.

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    addonmanager.kubernetes.io/mode: EnsureExists
  name: kube-dns
  namespace: kube-system
data:
  stubDomains: |
    {"consul": ["$(kubectl get svc consul-dns -o jsonpath='{.spec.clusterIP}')"]}
EOF
```

-> **Note:** The `stubDomain` can only point to a static IP. If the cluster IP
of the `consul-dns` service changes, then it must be updated in the config map to 
match the new service IP for this to continue
working. This can happen if the service is deleted and recreated, such as
in full cluster rebuilds.

## CoreDNS Configuration

If you are using CoreDNS instead of kube-dns in your Kubernetes cluster, you will
need to update your existing `coredns` ConfigMap in the `kube-system` namespace to
include a proxy definition for `consul` that points to the cluster IP of the 
`consul-dns` service.

```
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    addonmanager.kubernetes.io/mode: EnsureExists
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        <Existing CoreDNS definition>
    }
    consul {
      errors
      cache 30
      proxy . <consul-dns service cluster ip>
    }
```

-> **Note:** The consul proxy can only point to a static IP. If the cluster IP
of the `consul-dns` service changes, then it must be updated to the new IP to continue
working. This can happen if the service is deleted and recreated, such as
in full cluster rebuilds.

## Verifying DNS Works

To verify DNS works, run a simple job to query DNS. Save the following
job to the file `job.yaml` and run it:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: dns
spec:
  template:
    spec:
      containers:
      - name: dns
        image: anubhavmishra/tiny-tools
        command: ["dig",  "consul.service.consul"]
      restartPolicy: Never
  backoffLimit: 4
```

```sh
$ kubectl apply -f job.yaml
```

Then query the pod name for the job and check the logs. You should see
output similar to the following showing a successful DNS query. If you see
any errors, then DNS is not configured properly.

```
$ kubectl get pods --show-all | grep dns
dns-lkgzl         0/1       Completed   0          6m

$ kubectl logs dns-lkgzl
; <<>> DiG 9.11.2-P1 <<>> consul.service.consul
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 4489
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 3, AUTHORITY: 0, ADDITIONAL: 4

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 4096
;; QUESTION SECTION:
;consul.service.consul.		IN	A

;; ANSWER SECTION:
consul.service.consul.	0	IN	A	10.36.2.23
consul.service.consul.	0	IN	A	10.36.4.12
consul.service.consul.	0	IN	A	10.36.0.11

;; ADDITIONAL SECTION:
consul.service.consul.	0	IN	TXT	"consul-network-segment="
consul.service.consul.	0	IN	TXT	"consul-network-segment="
consul.service.consul.	0	IN	TXT	"consul-network-segment="

;; Query time: 5 msec
;; SERVER: 10.39.240.10#53(10.39.240.10)
;; WHEN: Wed Sep 12 02:12:30 UTC 2018
;; MSG SIZE  rcvd: 206
```

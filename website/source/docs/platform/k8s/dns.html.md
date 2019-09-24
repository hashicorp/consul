---
layout: "docs"
page_title: "Consul DNS - Kubernetes"
sidebar_current: "docs-platform-k8s-dns"
description: |-
  One of the primary query interfaces to Consul is the DNS interface. The Consul DNS interface can be exposed for all pods in Kubernetes using a stub-domain configuration.
---

# Consul DNS on Kubernetes

One of the primary query interfaces to Consul is the
[DNS interface](/docs/agent/dns.html). You can configure Consul DNS in
Kubernetes using a
[stub-domain configuration](https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#configure-stub-domain-and-upstream-dns-servers)
if using KubeDNS or a [proxy configuration](https://coredns.io/plugins/proxy/) if using CoreDNS.

Once configured, DNS requests in the form `<consul-service-name>.service.consul` will
resolve for services in Consul. This will work from all Kubernetes namespaces.

-> **Note:** If you want requests to just `<consul-service-name>` (without the `.service.consul`) to resolve, then you'll need
to turn on [Consul to Kubernetes Service Sync](/docs/platform/k8s/service-sync.html#consul-to-kubernetes).

## Consul DNS Cluster IP
To configure KubeDNS or CoreDNS you'll first need the `ClusterIP` of the Consul
DNS service created by the [Helm chart](/docs/platform/k8s/helm.html).

The default name of the Consul DNS service will be `consul-consul-dns`. Use
that name to get the `ClusterIP`:

```bash
$ kubectl get svc consul-consul-dns -o jsonpath='{.spec.clusterIP}'
10.35.240.78%
```

For this installation the `ClusterIP` is `10.35.240.78`. 

-> **Note:** If you've installed Consul using a different helm release name than `consul`
then the DNS service name will be `<release-name>-consul-dns`.

## KubeDNS
If using KubeDNS, you need to create a `ConfigMap` that tells KubeDNS
to use the Consul DNS service to resolve all domains ending with `.consul`:

Export the Consul DNS IP as an environment variable:

```bash
export CONSUL_DNS_IP=10.35.240.78
```

And create the `ConfigMap`:

```bash
$ cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    addonmanager.kubernetes.io/mode: EnsureExists
  name: kube-dns
  namespace: kube-system
data:
  stubDomains: |
    {"consul": ["$CONSUL_DNS_IP"]}
EOF
Warning: kubectl apply should be used on resource created by either kubectl create --save-config or kubectl apply
configmap/kube-dns configured
```

Ensure that the `ConfigMap` was created successfully:

```bash
$ kubectl get configmap kube-dns -n kube-system -o yaml
apiVersion: v1
data:
  stubDomains: |
    {"consul": ["10.35.240.78"]}
kind: ConfigMap
...
```

-> **Note:** The `stubDomain` can only point to a static IP. If the cluster IP
of the Consul DNS service changes, then it must be updated in the config map to 
match the new service IP for this to continue
working. This can happen if the service is deleted and recreated, such as
in full cluster rebuilds.

-> **Note:** If using a different zone than `.consul`, change the stub domain to
that zone.

Now skip ahead to the [Verifying DNS Works](#verifying-dns-works) section.

## CoreDNS Configuration

If using CoreDNS instead of KubeDNS in your Kubernetes cluster, you will
need to update your existing `coredns` ConfigMap in the `kube-system` namespace to
include a `forward` definition for `consul` that points to the cluster IP of the
Consul DNS service.

Edit the `ConfigMap`:

```bash
$ kubectl edit configmap coredns -n kube-system
```

And add the `consul` block below the default `.:53` block and replace
`<consul-dns-service-cluster-ip>` with the DNS Service's IP address you
found previously.

```diff
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
+   consul {
+     errors
+     cache 30
+     forward . <consul-dns-service-cluster-ip>
+   }
```

-> **Note:** The consul proxy can only point to a static IP. If the cluster IP
of the `consul-dns` service changes, then it must be updated to the new IP to continue
working. This can happen if the service is deleted and recreated, such as
in full cluster rebuilds.

-> **Note:** If using a different zone than `.consul`, change the key accordingly.

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

```sh
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

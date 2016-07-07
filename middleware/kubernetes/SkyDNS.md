## DNS Schema

Notes about the SkyDNS record naming scheme. (Copied from SkyDNS project README for reference while
hacking on the k8s middleware.)

### Services

#### A Records

"Normal" (not headless) Services are assigned a DNS A record for a name of the form `my-svc.my-namespace.svc.cluster.local.`
This resolves to the cluster IP of the Service.

"Headless" (without a cluster IP) Services are also assigned a DNS A record for a name of the form `my-svc.my-namespace.svc.cluster.local.`
Unlike normal Services, this resolves to the set of IPs of the pods selected by the Service.
Clients are expected to consume the set or else use standard round-robin selection from the set.


### Pods

#### A Records

When enabled, pods are assigned a DNS A record in the form of `pod-ip-address.my-namespace.pod.cluster.local.`

For example, a pod with ip `1.2.3.4` in the namespace default with a dns name of `cluster.local` would have 
an entry: `1-2-3-4.default.pod.cluster.local.`

####A Records and hostname Based on Pod Annotations - A Beta Feature in Kubernetes v1.2
Currently when a pod is created, its hostname is the Pod's `metadata.name` value.
With v1.2, users can specify a Pod annotation, `pod.beta.kubernetes.io/hostname`, to specify what the Pod's hostname should be.
If the annotation is specified, the annotation value takes precendence over the Pod's name, to be the hostname of the pod.
For example, given a Pod with annotation `pod.beta.kubernetes.io/hostname: my-pod-name`, the Pod will have its hostname set to "my-pod-name".

v1.2 introduces a beta feature where the user can specify a Pod annotation, `pod.beta.kubernetes.io/subdomain`, to specify what the Pod's subdomain should be.
If the annotation is specified, the fully qualified Pod hostname will be "<hostname>.<subdomain>.<pod namespace>.svc.<cluster domain>".
For example, given a Pod with the hostname annotation set to "foo", and the subdomain annotation set to "bar", in namespace "my-namespace", the pod will set its own FQDN as "foo.bar.my-namespace.svc.cluster.local"

If there exists a headless service in the same namespace as the pod and with the same name as the subdomain, the cluster's KubeDNS Server will also return an A record for the Pod's fully qualified hostname.
Given a Pod with the hostname annotation set to "foo" and the subdomain annotation set to "bar", and a headless Service named "bar" in the same namespace, the pod will see it's own FQDN as "foo.bar.my-namespace.svc.cluster.local". DNS will serve an A record at that name, pointing to the Pod's IP.

With v1.2, the Endpoints object also has a new annotation `endpoints.beta.kubernetes.io/hostnames-map`. Its value is the json representation of map[string(IP)][endpoints.HostRecord], for example: '{"10.245.1.6":{HostName: "my-webserver"}}'.
If the Endpoints are for a headless service, then A records will be created with the format <hostname>.<service name>.<pod namespace>.svc.<cluster domain>
For the example json, if endpoints are for a headless service named "bar", and one of the endpoints has IP "10.245.1.6", then a A record will be created with the name "my-webserver.bar.my-namespace.svc.cluster.local" and the A record lookup would return "10.245.1.6".
This endpoints annotation generally does not need to be specified by end-users, but can used by the internal service controller to deliver the aforementioned feature.


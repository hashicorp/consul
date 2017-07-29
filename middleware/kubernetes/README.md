# kubernetes

The *kubernetes* middleware enables the reading zone data from a Kubernetes cluster.  It implements the [Kubernetes DNS-Based Service Discovery Specification](https://github.com/kubernetes/dns/blob/master/docs/specification.md). 

CoreDNS running the kubernetes middleware can be used as a replacement of kube-dns in a kubernetes cluster.  See the [deployment](https://github.com/coredns/deployment) repository for details on [how to deploy CoreDNS in Kubernetes](https://github.com/coredns/deployment/tree/master/kubernetes).

## Syntax

```
kubernetes ZONE [ZONE...] [{
	[resyncperiod DURATION]
	[endpoint URL
	[tls CERT KEY CACERT]]
	[namespaces NAMESPACE [NAMESPACE...]]
	[labels EXPRESSION]
	[pods POD-MODE]
	[cidrs CIDR [CIDR...]]
	[upstream ADDRESS [ADDRESS...]]
	[federation NAME DOMAIN]
	[autopath [NDOTS [RESPONSE [RESOLV-CONF]]]
	[fallthrough]
}]
```

* `resyncperiod` **DURATION**
	
  The Kubernetes data API resynchronization period. Default is 5m. Example values: 60s, 5m, 1h

  Example:

  ```
	kubernetes cluster.local. {
		resyncperiod 15m
	}
  ```

* `endpoint` **URL**
	
  Use **URL** for a remote k8s API endpoint.  If omitted, it will connect to k8s in-cluster using the cluster service account. 

  Example:

  ```
	kubernetes cluster.local. {
		endpoint http://k8s-endpoint:8080
	}
  ```

* `tls` **CERT** **KEY** **CACERT**

  The TLS cert, key and the CA cert file names for remote k8s connection. This option is ignored if connecting in-cluster (i.e. endpoint is not
specified).

  Example:
 
  ```
	kubernetes cluster.local. {
		endpoint https://k8s-endpoint:8443
		tls cert key cacert
	}
  ```

* `namespaces` **NAMESPACE [NAMESPACE...]**

  Only expose the k8s namespaces listed.  If this option is omitted all namespaces are exposed

  Example:
 
  ```
	kubernetes cluster.local. {
		namespaces demo default
	}
  ```

* `labels` **EXPRESSION**
	
  Only expose the records for Kubernetes objects that match this label selector. The label selector syntax is described in the  [Kubernetes User Guide - Labels](http://kubernetes.io/docs/user-guide/labels/).

  Example:

  The following example only exposes objects labeled as "application=nginx" in the "staging" or "qa" environments.

  ```
	kubernetes cluster.local. {
		labels environment in (staging, qa),application=nginx
	}
  ```
 
* `pods` **POD-MODE**

  Set the mode for handling IP-based pod A records, e.g. `1-2-3-4.ns.pod.cluster.local. in A 1.2.3.4`.  This option is provided to facilitate use of SSL certs when connecting directly to pods.

  Valid values for **POD-MODE**:

  * `disabled`: Default. Do not process pod requests, always returning `NXDOMAIN`

  * `insecure`: Always return an A record with IP from request (without checking k8s).  This option is is vulnerable to abuse if used maliciously in conjunction with wildcard SSL certs.  This option is provided for backward compatibility with kube-dns.
	            
  * `verified`: Return an A record if there exists a pod in same namespace with matching IP.  This option requires substantially more memory than in insecure mode, since it will maintain a watch on all pods.
	 
  Example:

  
  ```
	kubernetes cluster.local. {
		pods verified
	}
  ```

* `cidrs` **CIDR [CIDR...]**
	
  Expose cidr ranges to reverse lookups.  Include any number of space delimited cidrs, and/or multiple cidrs options on separate lines. The Kubernetes middleware will respond to PTR requests for ip addresses that fall within these ranges.

  Example:
 
 
  ```
	kubernetes cluster.local. {
		cidrs 10.0.0.0/24 10.0.10.0/25
	}
 
  ```

* `upstream` **ADDRESS [ADDRESS...]**

  Defines upstream resolvers used for resolving services that point to external hosts (External Services).  **ADDRESS** can be an ip, an ip:port, or a path to a file structured like resolv.conf.
	
  Example:
 
   ```
	kubernetes cluster.local. {
		upstream 12.34.56.78:5053
	}
 
   ```
 
* `federation` **NAME DOMAIN**

  Defines federation membership.  One line for each federation membership. Each line consists of the name of the federation, and the domain.

  Example:

  ```
 	kubernetes cluster.local. {
		federation myfed foo.example.com
	}
  ```

* `autopath` **[NDOTS [RESPONSE [RESOLV-CONF]]**

  Enables server side search path lookups for pods.  When enabled, the kubernetes middleware will identify search path queries from pods and perform the remaining lookups in the path on the pod's behalf.  The search path used mimics the resolv.conf search path deployed to pods by the "cluster-first" dns-policy. E.g.

  ```
  search ns1.svc.cluster.local svc.cluster.local cluster.local foo.com
  ```

  If no domains in the path produce an answer, a lookup on the bare question will be attempted.	

  A successful response will contain a question section with the original question, and an answer section containing the record for the question that actually had an answer.  This means that the question and answer will not match. To avoid potential client confusion, a dynamically generated CNAME entry is added to join the two. For example:

  ```
    # host -v -t a google.com
    Trying "google.com.default.svc.cluster.local"
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 50957
    ;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

    ;; QUESTION SECTION:
    ;google.com.default.svc.cluster.local. IN A

    ;; ANSWER SECTION:
    google.com.default.svc.cluster.local. 175 IN CNAME google.com.
    google.com.		175	IN	A	216.58.194.206
  ```

  **Fully Qualified Queries:** There is a known limitation of the autopath feature involving fully qualified queries. When the kubernetes middleware receives a query from a client, it cannot tell the difference between a query that was fully qualified by the user, and one that was expanded by the first search path on the client side.

  This means that the kubernetes middleware with autopath enabled will perform a server-side path search for the query `google.com.default.svc.cluster.local.` as if the client had queried just `google.com`.  In other words, a query for `google.com.default.svc.cluster.local.` will produce the IP address for `google.com` as seen below.

  ```
	# host -t a google.com
	google.com has address 216.58.194.206
	google.com.default.svc.cluster.local is an alias for google.com.
	
	# host -t a google.com.default.svc.cluster.local.
	google.com has address 216.58.194.206
	google.com.default.svc.cluster.local is an alias for google.com.
  ```
 
  **NDOTS** (default: `0`) This provides an adjustable threshold to prevent server side lookups from triggering. If the number of dots before the first search domain is less than this number, then the search path will not executed on the server side.  When autopath is enabled with default settings, the search path is always conducted when the query is in the first search domain `<pod-namespace>.svc.<zone>.`.
	
  **RESPONSE** (default: `NOERROR`) This option causes the kubernetes middleware to return the given response instead of NXDOMAIN when the all searches in the path produce no results. Valid values: `NXDOMAIN`, `SERVFAIL` or `NOERROR`. Setting this to `SERVFAIL` or `NOERROR` should prevent the client from fruitlessly continuing the client side searches in the path after the server already checked them.

  **RESOLV-CONF** (default: `/etc/resolv.conf`) If specified, the kubernetes middleware uses this file to get the host's search domains. The kubernetes middleware performs a lookup on these domains if the in-cluster search domains in the path fail to produce an answer. If not specified, the values will be read from the local resolv.conf file (i.e the resolv.conf file in the pod containing CoreDNS).  In practice, this option should only need to be used if running CoreDNS outside of the cluster and the search path in /etc/resolv.conf does not match the cluster's "default" dns-policiy.

  Enabling autopath requires more memory, since it needs to maintain a watch on all pods. If autopath and `pods verified` mode are both enabled, they will share the same watch. Enabling both options should have an equivalent memory impact of just one.

  Example:
 
  ```
	kubernetes cluster.local. {
		autopath 0 NXDOMAIN /etc/resolv.conf
	}
  ```

* `fallthrough`

  If a query for a record in the cluster zone results in NXDOMAIN, normally that is what the response will be. However, if you specify this option, the query will instead be passed on down the middleware chain, which can include another middleware to handle the query.


## Examples

**Example 1:** This is a minimal configuration with no options other than zone. It will handle all queries in the `cluster.local` zone and connect to Kubernetes in-cluster, but it will not provide PTR records for services, or A records for pods.

	kubernetes cluster.local

**Example 2:** Handle all queries in the `cluster.local` zone. Connect to Kubernetes in-cluster. Handle all `PTR` requests in the `10.0.0.0/16` cidr block. Verify the existence of pods when answering pod requests.  Resolve upstream records against `10.102.3.10`. Enable the autopath feature.

	kubernetes cluster.local {
		cidrs 10.0.0.0/16
		pods verified
		upstream 10.102.3.10:53
		autopath
	}
	
**Selective Exposure Example:** Handle all queries in the `cluster.local` zone. Connect to Kubernetes in-cluster. Only expose objects in the test and staging namespaces. Handle all `PTR` requests that fall between `10.0.0.100` and `10.0.0.255` (expressed as CIDR blocks in the example below). Resolve upstream records using the servers configured in `/etc/resolv.conf`.

	kubernetes cluster.local {
		namespaces test staging
		cidrs 10.0.0.100/30 10.0.0.104/29
		cidrs 10.0.0.112/28 10.0.0.128/25
		upstream /etc/resolv.conf
	}

**Federation Example:** Handle all queries in the `cluster.local` zone. Connect to Kubernetes in-cluster. Handle federated service requests in the `prod` and `stage` federations. Handle all `PTR` requests in the `10.0.0.0/24` cidr block. Resolve upstream records using the servers configured in `/etc/resolv.conf`.

	kubernetes cluster.local {
		federation prod prod.feddomain.com
		federation stage stage.feddomain.com
		cidrs 10.0.0.0/24
		upstream /etc/resolv.conf
	}
	
**Out-Of-Cluster Example:** Handle all queries in the `cluster.local` zone. Connect to Kubernetes from outside the cluster. Handle all `PTR` requests in the `10.0.0.0/24` cidr block. Verify the existence of pods when answering pod requests.  Resolve upstream records against `10.102.3.10`. Enable the autopath feature, using the `cluster.conf` file instead of `/etc/resolv.conf`.

	kubernetes cluster.local {
		endpoint https://k8s-endpoint:8443
		tls cert key cacert
		cidrs 10.0.0.0/24
		pods verified
		upstream 10.102.3.10:53
		autopath 0 NOERROR cluster.conf
	}



## Wildcards

Some query labels accept a wildcard value to match any value.  If a label is a valid wildcard (\*, or the word "any"), then that label will match all values.  The labels that accept wildcards are:

 * _service_ in an `A` record request: _service_.namespace.svc.zone.
   * e.g. `*.ns.svc.myzone.local`
 * _namespace_ in an `A` record request: service._namespace_.svc.zone.
   * e.g. `nginx.*.svc.myzone.local`
 * _port and/or protocol_ in an `SRV` request: __port_.__protocol_.service.namespace.svc.zone.
   * e.g. `_http.*.service.ns.svc.`
 * multiple wild cards are allowed in a single query.
   * e.g. `A` Request `*.*.svc.zone.` or `SRV` request `*.*.*.*.svc.zone.`

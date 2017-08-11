# kubernetes

The *kubernetes* middleware enables the reading zone data from a Kubernetes cluster.  It implements
the [Kubernetes DNS-Based Service Discovery
Specification](https://github.com/kubernetes/dns/blob/master/docs/specification.md).

CoreDNS running the kubernetes middleware can be used as a replacement of kube-dns in a kubernetes
cluster.  See the [deployment](https://github.com/coredns/deployment) repository for details on [how
to deploy CoreDNS in Kubernetes](https://github.com/coredns/deployment/tree/master/kubernetes).

## Syntax

~~~
kubernetes [ZONES...]
~~~

With only the directive specified, the *kubernetes* middleware will default to the zone specified in
the server's block. It will handle all queries in that zone and connect to Kubernetes in-cluster. It
will not provide PTR records for services, or A records for pods. If **ZONES** is used is specifies
all the zones the middleware should be authoritative for.

```
kubernetes [ZONES...] {
	resyncperiod DURATION
	endpoint URL
	tls CERT KEY CACERT
	namespaces NAMESPACE [NAMESPACE...]
	labels EXPRESSION
	pods POD-MODE
	upstream ADDRESS [ADDRESS...]
	federation NAME DOMAIN
	fallthrough
}
```
* `resyncperiod` specifies the Kubernetes data API **DURATION** period.
* `endpoint` specifies the **URL** for a remove k8s API endpoint.
   If omitted, it will connect to k8s in-cluster using the cluster service account.
   Multiple k8s API endpoints could be specified, separated by `,`s, e.g.
   `endpoint http://k8s-endpoint1:8080,http://k8s-endpoint2:8080`. CoreDNS
   will automatically perform a healthcheck and proxy to the healthy k8s API endpoint.
* `tls` **CERT** **KEY** **CACERT** are the TLS cert, key and the CA cert file names for remote k8s connection.
   This option is ignored if connecting in-cluster (i.e. endpoint is not specified).
* `namespaces` **NAMESPACE [NAMESPACE...]**, exposed only the k8s namespaces listed.
   If this option is omitted all namespaces are exposed
* `labels` **EXPRESSION** only exposes the records for Kubernetes objects that match this label selector.
   The label selector syntax is described in the
   [Kubernetes User Guide - Labels](http://kubernetes.io/docs/user-guide/labels/). An example that
   only exposes objects labeled as "application=nginx" in the "staging" or "qa" environments, would
   use: `labels environment in (staging, qa),application=nginx`.
* `pods` **POD-MODE** sets the mode for handling IP-based pod A records, e.g.
   `1-2-3-4.ns.pod.cluster.local. in A 1.2.3.4`.
   This option is provided to facilitate use of SSL certs when connecting directly to pods. Valid
   values for **POD-MODE**:

   * `disabled`: Default. Do not process pod requests, always returning `NXDOMAIN`
   * `insecure`: Always return an A record with IP from request (without checking k8s).  This option
     is is vulnerable to abuse if used maliciously in conjunction with wildcard SSL certs.  This
     option is provided for backward compatibility with kube-dns.
   * `verified`: Return an A record if there exists a pod in same namespace with matching IP.  This
     option requires substantially more memory than in insecure mode, since it will maintain a watch
     on all pods.

* `upstream` **ADDRESS [ADDRESS...]** defines the upstream resolvers used for resolving services
  that point to external hosts (External Services).  **ADDRESS** can be an ip, an ip:port, or a path
  to a file structured like resolv.conf.
* `federation` **NAME DOMAIN** defines federation membership.  One line for each federation
  membership. Each line consists of the name of the federation, and the domain.
* `fallthrough`  If a query for a record in the cluster zone results in NXDOMAIN, normally that is
  what the response will be. However, if you specify this option, the query will instead be passed
  on down the middleware chain, which can include another middleware to handle the query.

## Examples

Handle all queries in the `cluster.local` zone. Connect to Kubernetes in-cluster.
Also handle all `PTR` requests for `10.0.0.0/16` . Verify the existence of pods when answering pod
requests. Resolve upstream records against `10.102.3.10`. Note we show the entire server block
here:

    10.0.0.0/16 cluster.local {
        kubernetes {
            pods verified
            upstream 10.102.3.10:53
        }
    }

Or you can selectively expose some namespaces:

	kubernetes cluster.local {
		namespaces test staging
    }

If you want to use federation, just use the `federation` option. Here we handle all service requests
in the `prod` and `stage` federations. We resolve upstream records using the servers configured in
`/etc/resolv.conf`.

    . {
        kubernetes cluster.local {
		    federation prod prod.feddomain.com
		    federation stage stage.feddomain.com
		    upstream /etc/resolv.conf
    	}
    }

And finally we can connect to Kubernetes from outside the cluster:

	kubernetes cluster.local {
		endpoint https://k8s-endpoint:8443
		tls cert key cacert
	}

## Enabling server-side domain search path completion with *autopath*

The *kubernetes* middleware can be used in conjunction with the *autopath* middleware.  Using this feature enables server-side domain search path completion in kubernetes clusters.  Note: `pods` must be set to `verified` for this to function properly.

	autopath @kubernetes
	kubernetes cluster.local {
		pods verified
	}


## Wildcard

Some query labels accept a wildcard value to match any value.  If a label is a valid wildcard (\*,
or the word "any"), then that label will match all values.  The labels that accept wildcards are:

 * _service_ in an `A` record request: _service_.namespace.svc.zone.
   * e.g. `*.ns.svc.myzone.local`
 * _namespace_ in an `A` record request: service._namespace_.svc.zone.
   * e.g. `nginx.*.svc.myzone.local`
 * _port and/or protocol_ in an `SRV` request: __port_.__protocol_.service.namespace.svc.zone.
   * e.g. `_http.*.service.ns.svc.`
 * multiple wild cards are allowed in a single query.
   * e.g. `A` Request `*.*.svc.zone.` or `SRV` request `*.*.*.*.svc.zone.`

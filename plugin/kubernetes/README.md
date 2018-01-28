# kubernetes

## Name

*kubernetes* - enables the reading zone data from a Kubernetes cluster.

## Description

It implements the [Kubernetes DNS-Based Service Discovery
Specification](https://github.com/kubernetes/dns/blob/master/docs/specification.md).

CoreDNS running the kubernetes plugin can be used as a replacement of kube-dns in a kubernetes
cluster.  See the [deployment](https://github.com/coredns/deployment) repository for details on [how
to deploy CoreDNS in Kubernetes](https://github.com/coredns/deployment/tree/master/kubernetes).

[stubDomains](http://blog.kubernetes.io/2017/04/configuring-private-dns-zones-upstream-nameservers-kubernetes.html)
are implemented via the *proxy* plugin.

## Syntax

~~~
kubernetes [ZONES...]
~~~

With only the directive specified, the *kubernetes* plugin will default to the zone specified in
the server's block. It will handle all queries in that zone and connect to Kubernetes in-cluster. It
will not provide PTR records for services, or A records for pods. If **ZONES** is used it specifies
all the zones the plugin should be authoritative for.

```
kubernetes [ZONES...] {
    resyncperiod DURATION
    endpoint URL [URL...]
    tls CERT KEY CACERT
    namespaces NAMESPACE...
    labels EXPRESSION
    pods POD-MODE
    endpoint_pod_names
    upstream ADDRESS...
    ttl TTL
    fallthrough [ZONES...]
}
```

* `resyncperiod` specifies the Kubernetes data API **DURATION** period.
* `endpoint` specifies the **URL** for a remote k8s API endpoint.
   If omitted, it will connect to k8s in-cluster using the cluster service account.
   Multiple k8s API endpoints could be specified:
   `endpoint http://k8s-endpoint1:8080 http://k8s-endpoint2:8080`. CoreDNS
   will automatically perform a healthcheck and proxy to the healthy k8s API endpoint.
* `tls` **CERT** **KEY** **CACERT** are the TLS cert, key and the CA cert file names for remote k8s connection.
   This option is ignored if connecting in-cluster (i.e. endpoint is not specified).
* `namespaces` **NAMESPACE [NAMESPACE...]**, only exposes the k8s namespaces listed.
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

* `endpoint_pod_names` uses the pod name of the pod targeted by the endpoint as
   the endpoint name in A records, e.g.
   `endpoint-name.my-service.namespace.svc.cluster.local. in A 1.2.3.4`
   By default, the endpoint-name name selection is as follows: Use the hostname
   of the endpoint, or if hostname is not set, use the dashed form of the endpoint
   IP address (e.g. `1-2-3-4.my-service.namespace.svc.cluster.local.`)
   If this directive is included, then name selection for endpoints changes as
   follows: Use the hostname of the endpoint, or if hostname is not set, use the
   pod name of the pod targeted by the endpoint. If there is no pod targeted by
   the endpoint, use the dashed IP address form.
* `upstream` **ADDRESS [ADDRESS...]** defines the upstream resolvers used for resolving services
  that point to external hosts (External Services).  **ADDRESS** can be an IP, an IP:port, or a path
  to a file structured like resolv.conf.
* `ttl` allows you to set a custom TTL for responses. The default (and allowed minimum) is to use
  5 seconds, the maximum is capped at 3600 seconds.
* `fallthrough` **[ZONES...]** If a query for a record in the zones for which the plugin is authoritative
  results in NXDOMAIN, normally that is what the response will be. However, if you specify this option,
  the query will instead be passed on down the plugin chain, which can include another plugin to handle
  the query. If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.

## Health

This plugin implements dynamic health checking. Currently this is limited to reporting healthy when
the API has synced.

## Examples

Handle all queries in the `cluster.local` zone. Connect to Kubernetes in-cluster. Also handle all
`in-addr.arpa` `PTR` requests for `10.0.0.0/17` . Verify the existence of pods when answering pod
requests. Resolve upstream records against `10.102.3.10`. Note we show the entire server block here:

~~~ txt
10.0.0.0/17 cluster.local {
    kubernetes {
        pods verified
        upstream 10.102.3.10:53
    }
}
~~~

Or you can selectively expose some namespaces:

~~~ txt
kubernetes cluster.local {
    namespaces test staging
}
~~~

Connect to Kubernetes with CoreDNS running outside the cluster:

~~~ txt
kubernetes cluster.local {
    endpoint https://k8s-endpoint:8443
    tls cert key cacert
}
~~~

Here we use the *proxy* plugin to implement stubDomains that forwards `example.org` and
`example.com` to another nameserver.

~~~ txt
cluster.local {
    kubernetes {
        endpoint https://k8s-endpoint:8443
        tls cert key cacert
    }
}
example.org {
    proxy . 8.8.8.8:53
}
example.com {
    proxy . 8.8.8.8:53
}
~~~

## AutoPath

The *kubernetes* plugin can be used in conjunction with the *autopath* plugin.  Using this
feature enables server-side domain search path completion in kubernetes clusters.  Note: `pods` must
be set to `verified` for this to function properly.

    cluster.local {
        autopath @kubernetes
        kubernetes {
            pods verified
        }
    }

## Federation

The *kubernetes* plugin can be used in conjunction with the *federation* plugin.  Using this
feature enables serving federated domains from the kubernetes clusters.

    cluster.local {
        federation {
            fallthrough
            prod prod.example.org
            staging staging.example.org

        }
        kubernetes
    }


## Wildcards

Some query labels accept a wildcard value to match any value.  If a label is a valid wildcard (\*,
or the word "any"), then that label will match all values.  The labels that accept wildcards are:

 * _service_ in an `A` record request: _service_.namespace.svc.zone, e.g. `*.ns.svc.myzone.local`
 * _namespace_ in an `A` record request: service._namespace_.svc.zone, e.g. `nginx.*.svc.myzone.local`
 * _port and/or protocol_ in an `SRV` request: __port_.__protocol_.service.namespace.svc.zone.,
   e.g. `_http.*.service.ns.svc.`
 * multiple wild cards are allowed in a single query, e.g. `A` Request `*.*.svc.zone.` or `SRV` request `*.*.*.*.svc.zone.`

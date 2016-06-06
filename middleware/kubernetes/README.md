# kubernetes

`kubernetes` enables reading zone data from a kubernetes cluster. Record names
are constructed as "myservice.mynamespace.coredns.local" where:

* "myservice" is the name of the k8s service (this may include multiple DNS labels, such as "c1.myservice"),
* "mynamespace" is the k8s namespace for the service, and
* "coredns.local" is the zone configured for `kubernetes`.


## Syntax

~~~
kubernetes [zones...]
~~~

* `zones` zones kubernetes should be authorative for.


~~~
kubernetes [zones] {
    endpoint http://localhost:8080
}
~~~

* `endpoint` the kubernetes API endpoint, default to http://localhost:8080

## Examples

This is the default kubernetes setup, with everything specified in full:

~~~
# Serve on port 53
.:53 {
    # use kubernetes middleware for domain "coredns.local"
    kubernetes coredns.local {
        # Use url for k8s API endpoint
        endpoint http://localhost:8080
    }
#    cache 160 coredns.local
}
~~~

### Basic Setup

#### Launch Kubernetes

Kubernetes is launched using the commands in the following `run_k8s.sh` script:

~~~
#!/bin/bash

# Based on instructions at: http://kubernetes.io/docs/getting-started-guides/docker/

#K8S_VERSION=$(curl -sS https://storage.googleapis.com/kubernetes-release/release/latest.txt)
K8S_VERSION="v1.2.4"

ARCH="amd64"

export K8S_VERSION
export ARCH

#DNS_ARGUMENTS="--cluster-dns=10.0.0.10 --cluster-domain=cluster.local"
DNS_ARGUMENTS=""

docker run -d \
    --volume=/:/rootfs:ro \
    --volume=/sys:/sys:ro \
    --volume=/var/lib/docker/:/var/lib/docker:rw \
    --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
    --volume=/var/run:/var/run:rw \
    --net=host \
    --pid=host \
    --privileged \
    gcr.io/google_containers/hyperkube-${ARCH}:${K8S_VERSION} \
    /hyperkube kubelet \
    --containerized \
    --hostname-override=127.0.0.1 \
    --api-servers=http://localhost:8080 \
    --config=/etc/kubernetes/manifests \
    ${DNS_ARGUMENTS} \
    --allow-privileged --v=2
~~~

#### Configure kubectl and test

The kubernetes control client can be downloaded from the generic URL:
`http://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${GOOS}/${GOARCH}/${K8S_BINARY}`

For example, the kubectl client for Linux can be downloaded using the command:
`curl -sSL "http://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl"

The following `setup_kubectl.sh` script can be stored in the same directory as 
kubectl to setup
kubectl to communicate with kubernetes running on the localhost:

~~~
#!/bin/bash

BASEDIR=`realpath $(dirname ${0})`

${BASEDIR}/kubectl config set-cluster test-doc --server=http://localhost:8080
${BASEDIR}/kubectl config set-context test-doc --cluster=test-doc
${BASEDIR}/kubectl config use-context test-doc

alias kubctl="${BASEDIR}/kubectl"
~~~


Verify that kubectl is working by querying for the kubernetes namespaces:

~~~
$ ./kubectl get namespaces
NAME      STATUS    AGE
default   Active    8d
test      Active    7d
~~~


#### Launch a kubernetes service and expose the service

The following commands will create a kubernetes namespace "demo",
launch an nginx service in the namespace, and expose the service on port 80:

~~~
$ ./kubectl create namespace demo
$ ./kubectl get namespace

$ ./kubectl run mynginx --namespace=demo --image=nginx
$ /kubectl get deployment --namespace=demo

$ ./kubectl expose deployment mynginx --namespace=demo --port=80
$ ./kubectl get service --namespace=demo
~~~


#### Launch CoreDNS

Build CoreDNS and launch using the configuration file in `conf/k8sCorefile`.
This configuration file sets up CoreDNS to use the zone `coredns.local` for
the kubernetes services.

The command to launch CoreDNS is:

~~~
$ ./coredns -conf conf/k8sCoreFile
~~~

In a separate terminal a dns query can be issued using dig:

~~~
$ dig @localhost mynginx.demo.coredns.local

; <<>> DiG 9.9.4-RedHat-9.9.4-29.el7_2.3 <<>> @localhost mynginx.demo.coredns.local
; (2 servers found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 47614
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 4096
;; QUESTION SECTION:
;mynginx.demo.coredns.local.    IN  A

;; ANSWER SECTION:
mynginx.demo.coredns.local. 0   IN  A   10.0.0.10

;; Query time: 2 msec
;; SERVER: ::1#53(::1)
;; WHEN: Thu Jun 02 11:07:18 PDT 2016
;; MSG SIZE  rcvd: 71
~~~



## Implementation Notes/Ideas

### Basic Zone Mapping (implemented)
The middleware is configured with a "zone" string. For 
example: "zone = coredns.local".

The Kubernetes service "myservice" running in "mynamespace" would map 
to: "myservice.mynamespace.coredns.local".

The middleware should publish an A record for that service and a service record.

Initial implementation just performs the above simple mapping. Subsequent 
revisions should allow different namespaces to be published under different zones.

For example:

    # Serve on port 53
    .:53 {
        # use kubernetes middleware for domain "coredns.local"
        kubernetes coredns.local {
            # Use url for k8s API endpoint
            endpoint http://localhost:8080
        }
        # Perform DNS response caching for the coredns.local zone
        # Cache timeout is provided by the integer argument in seconds
        # This works for the kubernetes middleware.)
        #cache 20 coredns.local
        #cache 160 coredns.local
    }


### Internal IP or External IP?
* Should the Corefile configuration allow control over whether the internal IP or external IP is exposed?
* If the Corefile configuration allows control over internal IP or external IP, then the config should allow users to control the precidence.

For example a service "myservice" running in namespace "mynamespace" with internal IP "10.0.0.100" and external IP "1.2.3.4".

This example could be published as:

| Corefile directive           | Result              |
|------------------------------|---------------------|
| iporder = internal           | 10.0.0.100          |
| iporder = external           | 1.2.3.4             |
| iporder = external, internal | 10.0.0.100, 1.2.3.4 |
| iporder = internal, external | 1.2.3.4, 10.0.0.100 |
| _no directive_               | 10.0.0.100, 1.2.3.4 |


### Wildcards

Publishing DNS records for singleton services isn't very interesting. Service
names are unique within a k8s namespace therefore multiple services will be
commonly run with a structured naming scheme.

For example, running multiple nginx services under the names:

| Service name |
|--------------|
| c1.nginx     |
| c2.nginx     |

or:

| Service name |
|--------------|
| nginx.c3     |
| nginx.c4     |

A DNS query with wildcard support for "nginx" in these examples should
return the IP addresses for all services with "nginx" in the service name.

TBD:
* How does this relate the the k8s load-balancer configuration?
* Do wildcards search across namespaces?
* Initial implementation assumes that a namespace maps to the first DNS label below the zone managed by the kubernetes middleware. This assumption may need to be revised.


## TODO
* Implement namespace filtering to different zones.
* Implement IP selection and ordering (internal/external).
* Implement SRV-record queries using naive lookup.
* Flatten service and namespace names to valid DNS characters. (service names
  and namespace names in k8s may use uppercase and non-DNS characters. Implement
  flattening to lower case and mapping of non-DNS characters to DNS characters
  in a standard way.)
* Do we need to generate synthetic zone records for namespaces?
* Implement wildcard-based lookup.
* Improve lookup to reduce size of query result obtained from k8s API.
  (namespace-based?, other ideas?)
* How to support label specification in Corefile to allow use of labels to 
  indicate zone? (Is this even useful?) For example, the following configuration
  exposes all services labeled for the "staging" environment and tenant "customerB"
  in the zone "customerB.stage.local":

~~~
kubernetes customerB.stage.local {
    # Use url for k8s API endpoint
    endpoint http://localhost:8080
    label "environment" : "staging", "tenant" : "customerB"
}
~~~

* Test with CoreDNS caching. CoreDNS caching for DNS response is working using
  the `cache` directive. Tested working using 20s cache timeout and A-record queries.
* DNS response caching is good, but we should also cache at the http query 
  level as well. (Take a look at https://github.com/patrickmn/go-cache as 
  a potential expiring cache implementation for the http API queries.)


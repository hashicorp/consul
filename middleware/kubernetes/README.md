# kubernetes

*kubernetes* enables reading zone data from a kubernetes cluster. Record names
are constructed as "myservice.mynamespace.coredns.local" where:

* "myservice" is the name of the k8s service (this may include multiple DNS labels,
  such as "c1.myservice"),
* "mynamespace" is the k8s namespace for the service, and
* "coredns.local" is the zone configured for `kubernetes`.

The record name format can be changed by specifying a name template in the Corefile.

## Syntax

~~~
kubernetes [ZONES...]
~~~

* `ZONES` zones kubernetes should be authorative for. Overlapping zones are ignored.


Or if you want to specify an endpoint:

~~~
kubernetes [ZONES...] {
    endpoint ENDPOINT
}
~~~

* **ENDPOINT** the kubernetes API endpoint, defaults to http://localhost:8080

TODO(...): Add all the other options.

## Examples

This is the default kubernetes setup, with everything specified in full:

~~~
# Serve on port 53
.:53 {
    # use kubernetes middleware for domain "coredns.local"
    kubernetes coredns.local {
        # Kubernetes data API resync period
        # Example values: 60s, 5m, 1h
        resyncperiod 5m
        # Use url for k8s API endpoint
        endpoint https://k8sendpoint:8080
        # The tls cert, key and the CA cert filenames
        tls cert key cacert
        # Assemble k8s record names with the template
        template {service}.{namespace}.{type}.{zone}
        # Only expose the k8s namespace "demo"
        namespaces demo
        # Only expose the records for kubernetes objects
        # that match this label selector. The label
        # selector syntax is described in the kubernetes
        # API documentation: http://kubernetes.io/docs/user-guide/labels/
        # Example selector below only exposes objects tagged as
        # "application=nginx" in the staging or qa environments.
        labels environment in (staging, qa),application=nginx
    }
    # Perform DNS response caching for the coredns.local zone
    # Cache timeout is specified by an integer in seconds
    #cache 180 coredns.local
}
~~~

Defaults:
* If the `namespaces` keyword is omitted, all kubernetes namespaces are exposed.
* If the `template` keyword is omitted, the default template of "{service}.{namespace}.{type}.{zone}" is used.
* If the `resyncperiod` keyword is omitted, the default resync period is 5 minutes.
* The `labels` keyword is only used when filtering results based on kubernetes label selector syntax
  is required. The label selector syntax is described in the kubernetes API documentation at:
  http://kubernetes.io/docs/user-guide/labels/

### Template Syntax
Record name templates can be constructed using the symbolic elements:

| template symbol | description                                                         |
| `{service}`     | Kubernetes object/service name.                                     |
| `{namespace}`   | The kubernetes namespace.                                           |
| `{type}`        | The type of the kubernetes object. Supports values 'svc' and 'pod'. |
| `{zone}`        | The zone configured for the kubernetes middleware.                  |

### Basic Setup

#### Launch Kubernetes

Kubernetes is launched using the commands in the `.travis/kubernetes/00_run_k8s.sh` script.

#### Configure kubectl and Test

The kubernetes control client can be downloaded from the generic URL:
`http://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${GOOS}/${GOARCH}/${K8S_BINARY}`

For example, the kubectl client for Linux can be downloaded using the command:
`curl -sSL "http://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl"`

The `contrib/kubernetes/testscripts/10_setup_kubectl.sh` script can be stored in the same directory as
kubectl to setup kubectl to communicate with kubernetes running on the localhost.

#### Launch a kubernetes service and expose the service

The following commands will create a kubernetes namespace "demo",
launch an nginx service in the namespace, and expose the service on port 80:

~~~
$ ./kubectl create namespace demo
$ ./kubectl get namespace

$ ./kubectl run mynginx --namespace=demo --image=nginx
$ ./kubectl get deployment --namespace=demo

$ ./kubectl expose deployment mynginx --namespace=demo --port=80
$ ./kubectl get service --namespace=demo
~~~

The script `.travis/kubernetes/20_setup_k8s_services.sh` creates a couple of sample namespaces
with services running in those namespaces. The automated kubernetes integration tests in
`test/kubernetes_test.go` depend on these services and namespaces to exist in kubernetes.


#### Launch CoreDNS

Build CoreDNS and launch using this configuration file:

~~~ txt
# Serve on port 53
.:53 {
    kubernetes coredns.local {
        resyncperiod 5m
        endpoint http://localhost:8080
        template {service}.{namespace}.{type}.{zone}
        namespaces demo
        # Only expose the records for kubernetes objects
        # that matches this label selector. 
        # See http://kubernetes.io/docs/user-guide/labels/
        # Example selector below only exposes objects tagged as
        # "application=nginx" in the staging or qa environments.
        #labels environment in (staging, qa),application=nginx
    }
    #cache 180 coredns.local # optionally enable caching
}
~~~

Put it in `~/k8sCorefile` for instance. This configuration file sets up CoreDNS to use the zone
`coredns.local` for the kubernetes services.

The command to launch CoreDNS is:

~~~
$ ./coredns -conf ~/k8sCorefile
~~~

In a separate terminal a DNS query can be issued using dig:

~~~
$ dig @localhost mynginx.demo.coredns.local

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


TODO(miek|...): below this line file bugs or issues and cleanup:

## Implementation Notes/Ideas

### Basic Zone Mapping
The middleware is configured with a "zone" string. For
example: "zone = coredns.local".

The Kubernetes service "myservice" running in "mynamespace" would map
to: "myservice.mynamespace.coredns.local".

The middleware should publish an A record for that service and a service record.

If multiple zone names are specified, the records for kubernetes objects are
exposed in all listed zones.

For example:

    # Serve on port 53
    .:53 {
        # use kubernetes middleware for domain "coredns.local"
        kubernetes coredns.local {
            # Use url for k8s API endpoint
            endpoint http://localhost:8080
        }
        # Perform DNS response caching for the coredns.local zone
        # Cache timeout is specified by an integer argument in seconds
        # (This works for the kubernetes middleware.)
        #cache 20 coredns.local
        #cache 160 coredns.local
    }


### Internal IP or External IP?
* Should the Corefile configuration allow control over whether the internal IP or external IP is exposed?
* If the Corefile configuration allows control over internal IP or external IP, then the config should allow users to control the precedence.

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
names are unique within a k8s namespace, therefore multiple services will be
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

## TODO
* SkyDNS compatibility/equivalency:
	* Kubernetes packaging and execution
		* Automate packaging to allow executing in Kubernetes. That is, add Docker
		  container build as target in Makefile. Also include anything else needed
		  to simplify launch as the k8s DNS service.
		  Note: Dockerfile already exists in coredns repo to build the docker image.
		  This work item should identify how to pass configuration and run as a SkyDNS
		  replacement.
		* Identify any kubernetes changes necessary to use coredns as k8s DNS server. That is,
		  how do we consume the "--cluster-dns=" and "--cluster-domain=" arguments.
		* Work out how to pass CoreDNS configuration via kubectl command line and yaml
		  service definition file.
		* Ensure that resolver in each kubernetes container is configured to use
		  coredns instance.
		* Update kubernetes middleware documentation to describe running CoreDNS as a
		  SkyDNS replacement. (Include descriptions of different ways to pass CoreFile
		  to coredns command.)
		* Remove dependency on healthz for health checking in
		  `kubernetes-rc.yaml` file.
		* Expose load-balancer IP addresses.
		* Calculate SRV priority based on number of instances running.
		  (See SkyDNS README.md)
	* Functional work
		* (done. '?' not supported yet) ~~Implement wildcard-based lookup. Minimally support `*`, consider `?` as well.~~
        * (done) ~~Note from Miek on PR 181: "SkyDNS also supports the word `any`.~~
		* Implement SkyDNS-style synthetic zones such as "svc" to group k8s objects. (This
		  should be optional behavior.) Also look at "pod" synthetic zones.
		* Implement test cases for SkyDNS equivalent functionality.
	* SkyDNS functionality, as listed in SkyDNS README: https://github.com/kubernetes/kubernetes/blob/release-1.2/cluster/addons/dns/README.md
		* Expose pods and srv objects.
		* A records in form of `pod-ip-address.my-namespace.cluster.local`.
		  For example, a pod with ip `1.2.3.4` in the namespace `default`
		  with a dns name of `cluster.local` would have an entry:
		  `1-2-3-4.default.pod.cluster.local`.
		* SRV records in form of
		  `_my-port-name._my-port-protocol.my-namespace.svc.cluster.local`
		  CNAME records for both regular services and headless services.
		  See SkyDNS README.
		* A Records and hostname Based on Pod Annotations (k8s beta 1.2 feature).
		  See SkyDNS README.
		* Note: the embedded IP and embedded port record names are weird. I
		  would need to know the IP/port in order to create the query to lookup
		  the name. Presumably these are intended for wildcard queries.
	* Performance
		* Improve lookup to reduce size of query result obtained from k8s API.
		  (namespace-based?, other ideas?)
* Additional features:
	* Reverse IN-ADDR entries for services. (Is there any value in supporting
	  reverse lookup records?) (need tests, functionality should work based on @aledbf's code.)
	* (done) ~~How to support label specification in Corefile to allow use of labels to
	  indicate zone? For example, the following
	  configuration exposes all services labeled for the "staging" environment
	  and tenant "customerB" in the zone "customerB.stage.local":

			kubernetes customerB.stage.local {
				# Use url for k8s API endpoint
				endpoint http://localhost:8080
				labels environment in (staging),tenant=customerB
			}

	  Note: label specification/selection is a killer feature for segmenting
	  test vs staging vs prod environments.~~ Need label testing.
	* Implement IP selection and ordering (internal/external). Related to
	  wildcards and SkyDNS use of CNAMES.
	* Flatten service and namespace names to valid DNS characters. (service names
	  and namespace names in k8s may use uppercase and non-DNS characters. Implement
	  flattening to lower case and mapping of non-DNS characters to DNS characters
	  in a standard way.)
	* Expose arbitrary kubernetes repository data as TXT records?
* DNS Correctness
	* Do we need to generate synthetic zone records for namespaces?
	* Do we need to generate synthetic zone records for the skydns synthetic zones?
* Test cases
	* Test with CoreDNS caching. CoreDNS caching for DNS response is working
	  using the `cache` directive. Tested working using 20s cache timeout
	  and A-record queries. Automate testing with cache in place.
	* Automate CoreDNS performance tests. Initially for zone files, and for
	  pre-loaded k8s API cache. With and without CoreDNS response caching.
    * Try to get rid of kubernetes launch scripts by moving operations into
      .travis.yml file.
    * Find root cause of timing condition that results in no data returned to
      test client when running k8s integration tests. Current work-around is a
      nasty hack of waiting 5 seconds after setting up test server before performing
      client calls. (See hack in test/kubernetes_test.go)

# Basic Setup for Development and Testing

## Launch Kubernetes

Kubernetes is launched using the commands in the `.travis/kubernetes/00_run_k8s.sh` script.

## Configure kubectl and Test

The kubernetes control client can be downloaded from the generic URL:
`http://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${GOOS}/${GOARCH}/${K8S_BINARY}`

For example, the kubectl client for Linux can be downloaded using the command:
`curl -sSL "http://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl"`

The `contrib/kubernetes/testscripts/10_setup_kubectl.sh` script can be stored in the same directory as
kubectl to setup kubectl to communicate with kubernetes running on the localhost.

## Launch a kubernetes service and expose the service

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


## Launch CoreDNS

Build CoreDNS and launch using this configuration file:

~~~ txt
# Serve on port 53
.:53 {
    kubernetes coredns.local {
        resyncperiod 5m
        endpoint http://localhost:8080
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

# Implementation Notes/Ideas

## Internal IP or External IP?
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
	* Functional work
		* Calculate SRV priority based on number of instances running.
		  (See SkyDNS README.md)
	* Performance
		* Improve lookup to reduce size of query result obtained from k8s API.
		  (namespace-based?, other ideas?)
		* reduce cache size by caching data into custom structs, instead of caching whole API objects
		* add (and use) indexes on the caches that support indexing
* Additional features:
	* Implement IP selection and ordering (internal/external). Related to
	  wildcards and SkyDNS use of CNAMES.
	* Expose arbitrary kubernetes repository data as TXT records?
* DNS Correctness
	* Do we need to generate synthetic zone records for namespaces?
	* Do we need to generate synthetic zone records for the skydns synthetic zones?
* Test cases
	* Implement test cases for SkyDNS equivalent functionality.
	* Add test cases for lables based filtering
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

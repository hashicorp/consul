# Basic Setup for Development and Testing

## Launch Kubernetes

To run the tests, you'll need a private, live Kubernetes cluster. If you don't have one,
you can try out [minikube](https://github.com/kubernetes/minikube), which is
also available via Homebrew for OS X users.

## Configure Test Data

The test data is all in [this manifest](https://github.com/coredns/coredns/blob/master/.travis/kubernetes/dns-test.yaml)
and you can load it with `kubectl apply -f`. It will create a couple namespaces and some services.
For the tests to pass, you should not create anything else in the cluster.

## Proxy the API Server

Assuming your Kuberentes API server isn't running on http://localhost:8080, you will need to proxy from that
port to your cluster. You can do this with `kubectl proxy --port 8080`.

## Run CoreDNS Kubernetes Tests

Now you can run the tests locally, for example:

~~~
$ cd $GOPATH/src/github.com/coredns/coredns/test
$ go test -v -tags k8s
~~~

# Implementation Notes/Ideas

* Additional features:
	* Implement IP selection and ordering (internal/external). Related to
	  wildcards and SkyDNS use of CNAMES.
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

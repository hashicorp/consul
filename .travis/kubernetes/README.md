## Test scripts to automate kubernetes startup

Requirements:
	docker
	curl

The scripts in this directory startup kubernetes with docker as the container runtime.
After starting kubernetes, a couple of kubernetes services are started to allow automatic
testing of CoreDNS with kubernetes. The kubernetes integration tests in `test/kubernetes_test.go` depend on having some sample services running. The scripts in this folder
automate the launch of kubernetes and the creation of the expected sample services.

To start up kubernetes and launch some sample services,
run the script `start_k8s_with_services.sh`.

~~~
$ ./start_k8s_with_services.sh
~~~

Alternatively, the individual scripts may be run independently as needed:

~~~
$ ./00_run_k8s.sh && ./10_setup_kubectl.sh && ./20_setup_k8s_services.sh
~~~

After running the above scripts, kubernetes will be running on the localhost with the following services
exposed:

~~
NAMESPACE   NAME         CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
default     kubernetes   10.0.0.1     <none>        443/TCP   48m
demo        mynginx      10.0.0.168   <none>        80/TCP    9m
demo        webserver    10.0.0.28    <none>        80/TCP    2m
test        mynginx      10.0.0.4     <none>        80/TCP    2m
test        webserver    10.0.0.39    <none>        80/TCP    2m
~~


Kubernetes and all running containers can be uncerimoniously stopped by
running the `kill_all_containers.sh` script.

~~~
$ ./kill_all_containers.sh
~~~

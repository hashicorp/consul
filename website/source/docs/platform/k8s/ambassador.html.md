---
layout: "docs"
page_title: "Ambassador Integration - Kubernetes"
sidebar_current: "docs-platform-k8s-ambassador"
description: |-
    Ambassador is a Kubernetes-native API gateway and ingress controller that 
    integrates well with Consul Connect.
---

# Ambassador Integration with Consul Connect

In addition to enabling Kubernetes services to discover and securely connect to each other,
Connect also can help route traffic into a Kubernetes cluster from outside, when paired with 
an [ingress controller] like DataWire's Ambassador.
 
[Ambassador] is a popular Kubernetes-native service that acts as an ingress controller
or API gateway. It supports an optional integration with Consul that allows it to
route incoming traffic to the [proxies] for your Connect-enabled services. 

This means you can have **end-to-end encryption** from the browser, to Ambassador,
to your Kubernetes services. 


## Installation 

Before you start, [install Consul] and [enable Connect] on the agents inside the cluster. Decide
whether you will enable [service sync] or manually register your services with
Consul.

Once you have tested and verified that everything is working, you can proceed with
the Ambassador installation. Full instructions are available on the [Ambassador
site][install], but a summary of the steps is as follows:

If you are deploying to GKE, create a `RoleBinding` to grant you cluster admin
rights:

```bash
kubectl create clusterrolebinding my-cluster-admin-binding \
        --clusterrole=cluster-admin \
        --user=$(gcloud info --format="value(config.account)")    
```

Install Ambassador and a `LoadBalancer` service for it:

```bash
kubectl apply -f https://www.getambassador.io/yaml/ambassador/ambassador-rbac.yaml
kubectl apply -f https://www.getambassador.io/yaml/ambassador/ambassador-service.yaml 
```

Install the Ambassador Consul Connector:

```bash
kubectl apply -f https://www.getambassador.io/yaml/consul/ambassador-consul-connector.yaml
```

Add `TLSContext` and `Mapping` annotations to your existing services, directing 
HTTPS traffic to port 20000, which is opened by the Connect proxy. Here is an 
example of doing this for the `static-server` example used in the documentation
for the [Connect sidecar]:

```yaml
apiVersion: v1
kind: Service
metadata:
    name: static-service
    annotations:
    getambassador.io/config: |
        ---
        apiVersion: ambassador/v1
        kind: TLSContext
        name: ambassador-consul
        hosts: []
        secret: ambassador-consul-connect
        ---
        apiVersion: ambassador/v1
        kind:  Mapping
        name:  static-service_mapping
        prefix: /echo/
        tls: ambassador-consul
        service: https://static-server:443
spec:
    type: NodePort
    selector:
    app: http-echo
    ports:
    - port: 443
    name: https-echo
    targetPort: 20000            
```

Once Ambassador finishes deploying, you should have a new `LoadBalancer` service
with a public-facing IP address. Connecting to the HTTP port on this address
should display the output from the static service.

```bash
kubectl describe service ambassador

```


## Enabling end-to-end TLS

The Ambassador service definition provided in their documentation currently does not
serve pages over HTTPS. To enable HTTPS for full end-to-end encryption, follow
these steps.

First, upload your public SSL certificate and private key as a Kubernetes secret. 

```bash
kubectl create secret tls ambassador-certs --cert=fullchain.pem --key=privkey.pem
```

Download a copy of the [ambassador-service.yaml] file from Ambassador. Replace
the `metadata` section with one that includes an Ambassador TLS configuration block,
using the secret name you created in the previous step. Then add an entry for port 443
to the `LoadBalancer` spec. Here is a complete example:

```yaml
apiVersion: v1
kind: Service
metadata:
name: ambassador
annotations: 
    getambassador.io/config: |
    ---
    apiVersion: ambassador/v1
    kind: Module
    name: tls
    config:
    server:
        enabled: True
        secret: ambassador-certs
spec:
    type: LoadBalancer
    externalTrafficPolicy: Local
    ports:
    - port: 80
        targetPort: http
        protocol: TCP
        name: http
    - port: 443
        targetPort: https
        protocol: TCP
        name: https   
    selector:
    service: ambassador
```

Update the service definition by applying it with `kubectl`:

```bash
kubectl apply -f ambassador-service.yaml
```

You should now be able to test the SSL connection from your browser.


## Troubleshooting

When Ambassador is unable to establish an authenticated connection to the Connect proxy servers, browser connections will display this message:

        upstream connect error or disconnect/reset before headers

This error can have a number of different causes. Here are some things to check and troubleshooting steps you can take.

### Check intentions between Ambassador and your upstream service

If you followed the above installation guide, Consul should have registered a service called "ambassador". Make sure you create an intention to allow it to connect to your own services.

To check whether Ambassador is allowed to connect, use the [`intention check`][intention-check] subcommand.

    $ consul intention check ambassador http-echo
    Allowed

### Confirm upstream proxy sidecar is running

First, find the name of the pod that contains your service.

    $ kubectl get pods -l app=http-echo,role=server
    NAME                         READY     STATUS    RESTARTS   AGE
    http-echo-7fb79566d6-jmccp   2/2       Running   0          1d

Then describe the pod to make sure that the sidecar is present and running.

    $ kubectl describe pod http-echo-7fb79566d6-jmccp
    [...]
    Containers:
        consul-connect-envoy-sidecar:
            [...]
            State:          Running
            Ready:          True    

### Start up a downstream proxy and try connecting to it

Log into one of your Consul server pods (or any pod that has a Consul binary in it).

    $ kubectl exec -ti consul-server-0 -- /bin/sh

Once inside the pod, try starting a test proxy. Use the name of your service in place of `http-echo`.

    # consul connect proxy -service=ambassador -upstream http-echo:1234
    ==> Consul Connect proxy starting...
    Configuration mode: Flags
               Service: http-echo-client
              Upstream: http-echo => :1234
       Public listener: Disabled

If the proxy starts successfully, try connecting to it. Verify the output is as you expect.

    # curl localhost:1234
    "hello world"

Don't forget to kill the test proxy when you're done.

    # kill %1
    ==> Consul Connect proxy shutdown
    
    # exit

### Check Ambassador Connect sidecar logs

Find the name of the Connect Integration pod and make sure it is running.

    $ kubectl get pods -l app=ambassador-pro,component=consul-connect
    NAME                                                        READY     STATUS    RESTARTS   AGE
    ambassador-pro-consul-connect-integration-f88fcb99f-hxk75   1/1       Running   0          1d

Dump the logs from the integration pod. If the service is running correctly, there won't be much in there.

    $ kubectl logs ambassador-pro-consul-connect-integration-f88fcb99f-hxk75
    
    time="2019-03-13T19:42:12Z" level=info msg="Starting Consul Connect Integration" consul_host=10.142.0.21 consul_port=8500 version=0.2.3
    2019/03/13 19:42:12 Watching CA leaf for ambassador
    time="2019-03-13T19:42:12Z" level=debug msg="Computed kubectl command and arguments" args="[kubectl apply -f -]"
    time="2019-03-13T19:42:14Z" level=info msg="Updating TLS certificate secret" namespace= secret=ambassador-consul-connect

### Check Ambassador logs

Make sure the Ambassador pod itself is running.

    $ kubectl get pods -l service=ambassador
    NAME                          READY     STATUS    RESTARTS   AGE
    ambassador-655875b5d9-vpc2v   2/2       Running   0          1d

Finally, check the logs for the main Ambassador pod. 

    $ kubectl logs ambassador-655875b5d9-vpc2v

### Check Ambassador admin interface

Forward the admin port from the Ambassador pod to your local machine.

    $ kubectl port-forward pods/ambassador-655875b5d9-vpc2v 8877:8877

You should then be able to open http://localhost:8877/ambassador/v0/diag/ in your browser and view Ambassador's routing table. The table lists each URL mapping that has been set up. Service names will appear in green if Ambassador believes they are healthy, and red otherwise.

From this interface, you can also enable debug logging via the yellow "Set Debug On" button, which might give you a better idea of what's happening when requests fail.

### Getting support

If you have tried the above troubleshooting steps and are still stuck, DataWire provides support for Ambassador via the popular Slack chat app. You can [request access] and then join the `#ambassador` room to get help.


[ambassador]: https://www.getambassador.io/
[ingress controller]: https://blog.getambassador.io/kubernetes-ingress-nodeport-load-balancers-and-ingress-controllers-6e29f1c44f2d
[proxies]: https://www.consul.io/docs/connect/proxies.html
[service sync]: https://www.consul.io/docs/platform/k8s/service-sync.html
[connect sidecar]: https://www.consul.io/docs/platform/k8s/connect.html
[install]: https://www.getambassador.io/user-guide/consul-connect-ambassador/
[ambassador-service.yaml]: https://www.getambassador.io/yaml/ambassador/ambassador-service.yaml
[request access]: https://d6e.co/slack
[intention-check]: https://www.consul.io/docs/commands/intention/check.html
[install consul]: https://www.consul.io/docs/install/index.html
[enable connect]: https://www.consul.io/docs/connect/index.html#getting-started-with-connect


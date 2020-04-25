In this guide you will use Consul to configure F5 BIG-IP nodes and server pools.
You will set up a basic F5 BIG-IP AS3 declaration that generates the load
balancer backend-server-pool configuration based on the available service
instances registered in Consul service discovery.

## Prerequisites

To complete this guide, you will need previous experience with F5 BIG-IP and
Consul. You can either manually deploy the necessary infrastructure, or use the
terraform demo code.

### Watch the Video - Optional

Consul's intetgration with F5 was demonstrated in a webinar. If you would prefer
to lear about the integration but aren't ready to try it out, you can [watch the
webinar recording
instead](https://www.hashicorp.com/resources/zero-touch-application-delivery-with-f5-big-ip-terraform-and-consul).

### Manually deploy your infrastructure

You should configure the following infrastructure.

- A single Consul datacenter with server and client nodes, and the configuration
directory for Consul agents at `/etc/consul.d/`.

- A running instance of the F5 BIG-IP platform. If you don’t already have one
you can use a [hosted AWS
instance](https://aws.amazon.com/marketplace/pp/B079C44MFH) for this guide.

- The AS3 package version 3.7.0
[installed](https://clouddocs.f5.com/products/extensions/f5-appsvcs-extension/latest/userguide/installation.html)
on your F5 BIG-IP platform.

- Standard web server running on a node, listening on HTTP port 80. We will use
NGINX in this guide.

### Deploy a demo using Terraform - Optional

You can set up the prerequisites on your own, or use the terraform
configuration in [this
repository](https://github.com/hashicorp/f5-terraform-consul-sd-webinar) to set
up a testing environment.

Once your environment is set up, you'll be able to visit the F5 GUI at
`<F5_IP>:8443/tmui/login.jsp` where `<F5_IP>` is the  address provided in your
Terraform output. Login with the username `admin` and the password from your
Terraform output.

### Verify your environment

Check your environment to ensure  you have a healthy Consul datacenter by
checking your datacenter members. You can do this by running the `consul
members` command on the machine where Consul is running, or by accessing the
Consul web UI at the IP address of your consul instances, on port 8500.

```shell
$ consul memberss
Node           Address          Status  Type    Build  Protocol  DC   Segment
consul         10.0.0.100:8301  alive   server  1.5.3  2         dc1  <all>
nginx          10.0.0.109:8301  alive   client  1.5.3  2         dc1  <default>
```

In this sample environment we have one Consul server node, and one web server
node with a Consul client.

## Register a Web Service

To register the web service on one of your client nodes with Consul, create a
service definition in Consul's config directory `/etc/consul.d/` named
`nginx-service.json`. Paste in the following configuration, which includes a tcp
check for the web server so that Consul can monitor its health.

```json
{
  "service": {
    "name": "nginx",
    "port": 80,
    "checks": [
      {
        "id": "nginx",
        "name": "nginx TCP Check",
        "tcp": "localhost:80",
        "interval": "5s",
        "timeout": "1s"
      }
    ]
  }
}
```

Reload the client to read the new service definition.

```shell
$ consul reload
```

In a broswer window, visit the services page of the Consul web UI at
`<your-consul-ip>:8500/ui/dc1/services/nginx`.

![Consul UI with NGINX registered](/static/img/consul-f5-nginx.png 'Consul web
UI with a healthy NGINX service')

You should notice your instance of the nginx service listed and healthy.

## Apply an AS3 Declaration

Next you will configure BIG-IP to use Consul Service discovery with an AS3
declaration. You will use cURL to apply the declaration to the BIG-IP Instance.

First construct an authorization header to authenticate our API call with
BIG-IP. You will need to use a username and password for your instance. Below is
an example for username “admin”, and password “password”.

```shell
$ echo -n 'admin:password' | base64
YWRtaW46YWRtaW4=
```

Now use cURL to send the authorized declaration to the BIG-IP Instance. Use the
value you created above for your BIG-IP instance in the authorization header.
Remember t o  replace `<your-BIG-IP-mgmt-ip>` with the real IP address.

```shell
$ curl -X POST \
  https://<your-BIG-IP-mgmt-ip>/mgmt/shared/appsvcs/declare \
  -H 'authorization: Basic <your-authorization-header>' \
  -d '{
    "class": "ADC",
    "schemaVersion": "3.7.0",
    "id": "Consul_SD",
        "controls": {
        "class": "Controls",
        "trace": true,
        "logLevel": "debug"
    },
    "Consul_SD": {
      "class": "Tenant",
      "Nginx": {
        "class": "Application",
        "template": "http",
        "serviceMain": {
          "class": "Service_HTTP",
          "virtualPort": 8080,
          "virtualAddresses": [
            "<your-BIG-IP-virtual-ip>"
          ],
          "pool": "web_pool"
        },
        "web_pool": {
          "class": "Pool",
          "monitors": [
            "http"
          ],
          "members": [
            {
              "servicePort": 80,
              "addressDiscovery": "consul",
              "updateInterval": 5,
              "uri": "http://<your-consul-ip>:8500/v1/catalog/service/nginx"
            }
          ]
        }
      }
    }
}
'
```

You should get a similar output to the following after you’ve applied your
declaration.

```json
{
  "results": [
    {
      "message": "success",
      "lineCount": 26,
      "code": 200,
      "host": "localhost",
      "tenant": "Consul_SD",
      "runTime": 3939
    }
  ],
  "declaration": {
    "class": "ADC",
    "schemaVersion": "3.7.0",
    "id": "Consul_SD",
    "controls": {
      "class": "Controls",
      "trace": true,
      "logLevel": "debug",
      "archiveTimestamp": "2019-09-06T03:12:06.641Z"
    },
    "Consul_SD": {
      "class": "Tenant",
      "Nginx": {
        "class": "Application",
        "template": "http",
        "serviceMain": {
          "class": "Service_HTTP",
          "virtualPort": 8080,
          "virtualAddresses": [
            "10.0.0.200"
          ],
          "pool": "web_pool"
        },
        "web_pool": {
          "class": "Pool",
          "monitors": [
            "http"
          ],
          "members": [
            {
              "servicePort": 80,
              "addressDiscovery": "consul",
              "updateInterval": 5,
              "uri": "http://10.0.0.100:8500/v1/catalog/service/nginx"
            }
          ]
        }
      }
    },
    "updateMode": "selective"
  }
}
```

The above declaration does the following:

- Creates a partition (tenant) named `Consul_SD`.

- Defines a virtual server named `serviceMain` in `Consul_SD` partition with:

  - A pool named web_pool monitored by the  http health monitor.

  - NGINX Pool members autodiscovered via Consul's [catalog HTTP API
    endpoint](https://www.consul.io/api/catalog.html#list-nodes-for-service).
    For the `virtualAddresses` make sure to substitute your BIG-IP Virtual
    Server.

  - A URI specific to your Consul environment for the scheme, host, and port of
    your consul address discovery. This could be a single server, load balanced
    endpoint, or co-located agent, depending on your requirements. Make sure to
    replace the `uri` in your configuration with the IP of your Consul client.

You can find more information on Consul SD declarations in [F5’s Consul service
discovery
documentation](https://clouddocs.f5.com/products/extensions/f5-appsvcs-extension/latest/declarations/discovery.html#service-discovery-using-hashicorp-consul)

You can read more about composing AS3 declarations in the [F5 documentation](https://clouddocs.f5.com/products/extensions/f5-appsvcs-extension/latest/userguide/composing-a-declaration.html). The Terraform provider for BIG-IP [also supports AS3 resources](https://www.terraform.io/docs/providers/bigip/r/bigip_as3.html).

## Verify BIG-IP Consul Communication

Use the `consul monitor` command on the consul agent specified in the AS3 URI to
verify that you are receiving catalog requests from the BIG-IP instance.

```sh
$ consul monitor -log-level=debug
2019/09/06 03:16:50 [DEBUG] http: Request GET /v1/catalog/service/nginx (103.796µs) from=10.0.0.200:29487
2019/09/06 03:16:55 [DEBUG] http: Request GET /v1/catalog/service/nginx (104.95µs) from=10.0.0.200:42079
2019/09/06 03:17:00 [DEBUG] http: Request GET /v1/catalog/service/nginx (98.652µs) from=10.0.0.200:45536
2019/09/06 03:17:05 [DEBUG] http: Request GET /v1/catalog/service/nginx (101.242µs) from=10.0.0.200:45940
```

Check that the interval matches the value you supplied in your AS3 declaration.

## Verify the BIG-IP Dynamic Pool

Check the network map of the BIG-IP instance to make sure that the NGINX
instances registered in Consul are also in your BIG-IP dynamic pool.

To check the network map, open a browser window and navigate to
`https://<your-big-IP-mgmt-ip>/tmui/tmui/locallb/network_map/app/?xui=false#!/?p=Consul_SD`.
Remember to replace the IP address.

![NGINX instances in BIG-IP](/static/img/consul-f5-partition.png 'NGINX
instances listed in the BIG-IP web graphical user interface')

You can read more about the network map in the [F5
documentation](https://support.f5.com/csp/article/K20448153#accessing%20map).

## Test the BIG-IP Virtual Server

Now that you have a healthy virtual service, you can use it to access your web
server.

```shell
$ curl <your-BIG-IP-virtual-ip>:8080
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```
## Summary

The F5 BIG-IP AS3 service discovery integration with Consul queries Consul's
catalog on a regular, configurable basis to get updates about changes for a
given service, and adjusts the node pools dynamically without operator
intervention.

In this guide you configured an F5 BIG-IP instance to natively integrate with
Consul for service discovery. You were able to monitor dynamic node registration
for a web server pool member, and test it with a virtual server.

As a follow up, you can add or reemove web server nodes reegistered with Consul
and validate that the network map on the F5 BIG-IP updates automatically.

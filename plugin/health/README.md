# health

## Name

*health* - enables a health check endpoint.

## Description

Enabled process wide health endpoint. When CoreDNS is up and running this returns a 200 OK HTTP
status code. The health is exported, by default, on port 8080/health .

## Syntax

~~~
health [ADDRESS]
~~~

Optionally takes an address; the default is `:8080`. The health path is fixed to `/health`. The
health endpoint returns a 200 response code and the word "OK" when this server is healthy.

An extra option can be set with this extended syntax:

~~~
health [ADDRESS] {
    lameduck DURATION
}
~~~

* Where `lameduck` will make the process unhealthy then *wait* for **DURATION** before the process
  shuts down.

If you have multiple Server Blocks, *health* can only be enabled in one of them (as it is process
wide). If you really need multiple endpoints, you must run health endpoints on different ports:

~~~ corefile
com {
    whoami
    health :8080
}

net {
    erratic
    health :8081
}
~~~

Doing this is supported but both endponts ":8080" and ":8081" will export the exact same health.

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metric is exported:

 *  `coredns_health_request_duration_seconds{}` - duration to process a HTTP query to the local
    `/health` endpoint. As this a local operation it should be fast. A (large) increase in this
    duration indicates the CoreDNS process is having trouble keeping up with its query load.

Note that this metric *does not* have a `server` label, because being overloaded is a symptom of
the running process, *not* a specific server.

## Examples

Run another health endpoint on http://localhost:8091.

~~~ corefile
. {
    health localhost:8091
}
~~~

Set a lameduck duration of 1 second:

~~~ corefile
. {
    health localhost:8092 {
        lameduck 1s
    }
}
~~~

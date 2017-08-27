# health

This module enables a simple health check endpoint. By default it will listen on port 8080.

## Syntax

~~~
health [ADDRESS]
~~~

Optionally takes an address; the default is `:8080`. The health path is fixed to `/health`. The
health endpoint returns a 200 response code and the word "OK" when CoreDNS is healthy. It returns
a 503. *health* periodically (1s) polls middleware that exports health information. If any of the
middleware signals that it is unhealthy, the server will go unhealthy too. Each middleware that
supports health checks has a section "Health" in their README.

## Examples

Run another health endpoint on http://localhost:8091.

~~~
health localhost:8091
~~~

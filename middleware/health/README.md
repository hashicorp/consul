# health

This module enables a simple health check.

By default it will listen on port 9180.

Restarting CoreDNS will stop the health checking. This is a bug. Also [this upstream
Caddy bug](https://github.com/mholt/caddy/issues/675).

## Syntax

~~~
health
~~~

It optionally takes an address, the default is `:8080`. The health path is fixed to `/health`. It
will just return "OK", when CoreDNS is healthy.

This middleware only needs to be enabled once.

## Examples

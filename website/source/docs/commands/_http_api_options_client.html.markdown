* `-http-addr=<addr>` - Address of the Consul agent with the port. This can be
  an IP address or DNS address, but it must include the port. This can also be
  specified via the `CONSUL_HTTP_ADDR` environment variable. The default value is
  127.0.0.1:8500.

* `-token=<value>` - ACL token to use in the request. This can also be specified
  via the `CONSUL_HTTP_TOKEN` environment variable. If unspecified, the query
  will default to the token of the Consul agent at the HTTP address.

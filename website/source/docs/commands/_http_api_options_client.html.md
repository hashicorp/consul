* `-ca-file=<value>` - Path to a CA file to use for TLS when communicating with Consul.
  This can also be specified via the `CONSUL_CACERT` environment variable.

* `-ca-path=<value>` - Path to a directory of CA certificates to use for TLS when
  communicating with Consul. This can also be specified via the `CONSUL_CAPATH`
  environment variable.

* `-client-cert=<value>` - Path to a client cert file to use for TLS when
  `verify_incoming` is enabled. This can also be specified via the `CONSUL_CLIENT_CERT`
  environment variable.

* `-client-key=<value>` - Path to a client key file to use for TLS when
  `verify_incoming` is enabled. This can also be specified via the `CONSUL_CLIENT_KEY`
  environment variable.

* `-http-addr=<addr>` - Address of the Consul agent with the port. This can be
  an IP address or DNS address, but it must include the port. This can also be
  specified via the `CONSUL_HTTP_ADDR` environment variable. In Consul 0.8 and
  later, the default value is http://127.0.0.1:8500, and https can optionally
  be used instead. The scheme can also be set to HTTPS by setting the
  environment variable `CONSUL_HTTP_SSL=true`.

* `-tls-server-name=<value>` - The server name to use as the SNI host when
  connecting via TLS. This can also be specified via the `CONSUL_TLS_SERVER_NAME`
  environment variable.

* `-token=<value>` - ACL token to use in the request. This can also be specified
  via the `CONSUL_HTTP_TOKEN` environment variable. If unspecified, the query
  will default to the token of the Consul agent at the HTTP address.

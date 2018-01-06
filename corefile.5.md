## Name

*corefile* - configuration file for CoreDNS

## Description

A *corefile* specifies the (internal) servers CoreDNS should run and what plugins each of these
should chain. The syntax is as follows:

~~~ txt
[SCHEME://]ZONE [[SCHEME://]ZONE]...[:PORT] {
    [PLUGIN]...
}
~~~

The **ZONE** defines for which name this server should be called, multiple zones are allowed and
should be *white space* separated. You can use a "reverse" syntax to specify a reverse zone (i.e.
ip6.arpa and in-addr.arpa), but using an IP address in the CIDR notation. The optional **SCHEME**
defaults to `dns://`, but can also be `tls://` (DNS over TLS) or `grpc://` (DNS over gRPC).

Specifying a **ZONE** *and* **PORT** combination multiple time for *different* servers will lead to
an error on startup.

When a query comes in it is matched again all zones for all servers, the server with the longest
match on the query name will receive the query.

The optional **PORT** controls on which port the server will bind, this default to 53. If you use
a port number here, you *can't* override it with `-dns.port` (coredns(1)).

**PLUGIN** defines the plugin(s) we want to load into this server. This is optional as well, but as
server with no plugins will just return SERVFAIL for all queries. Each plugin can have a number of
properties than can have arguments, see documentation for each plugin.

Comments begin with an unquoted hash `#` and continue to the end of the line. Comments may be
started anywhere on a line.

Enviroment variables are supported and either the Unix or Windows form may be used: `{$ENV_VAR_1}`
or `{%ENV_VAR_2%}`.

You can use the `import` "plugin" to include parts of other files, see <https://coredns.io/explugins/import>.

If CoreDNS can’t find a Corefile to load it loads the following builtin one:

~~~ Corefile
. {
    whoami
}
~~~

## Examples

The **ZONE** is root zone `.`, the **PLUGIN** is chaos. The chaos plugin takes an argument:
`CoreDNS-001`. This text is returned on a CH class query: `dig CH txt version.bind @localhost`.

~~~ Corefile
. {
   chaos CoreDNS-001
}
~~~

When defining a new zone, you either create a new server, or add it to an existing one. Here we
define one server that handles two zones; that potentially chain different plugins:

~~~ Corefile
example.org {
    whoami
}
org {
    whoami
}
~~~

Is identical to:

~~~ Corefile
example.org org {
    whoami
}
~~~

Reverse zones can be specified as domain names:

~~~ Corefile
0.0.10.in-addr.arpa {
    whoami
}
~~~

or by just using the CIDR notation:

~~~ Corefile
10.0.0.0/24 {
    whoami
}
~~~

This also works on a non octet boundary:

~~~ Corefile
10.0.0.0/27 {
    whoami
}
~~~

## Authors

CoreDNS Authors.

## Copyright

Apache License 2.0

## See Also

The manual page for CoreDNS: coredns(1) and more documentation on <https://coredns.io>.

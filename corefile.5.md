## Name

*corefile* - configuration file for CoreDNS.

## Description

A *corefile* specifies the internal servers CoreDNS should run and what plugins each of these
should chain. The syntax is as follows:

~~~ txt
[SCHEME://]ZONE [[SCHEME://]ZONE]...[:PORT] {
    [PLUGIN]...
}
~~~

The **ZONE** defines for which name this server should be called, multiple zones are allowed and
should be *white space* separated. You can use a "reverse" syntax to specify a reverse zone (i.e.
ip6.arpa and in-addr.arpa), by using an IP address in the CIDR notation.

The optional **SCHEME** defaults to `dns://`, but can also be `tls://` (DNS over TLS), `grpc://`
(DNS over gRPC) or `https://` (DNS over HTTP/2).

The optional **PORT** controls on which port the server will bind, this default to 53. If you use
a port number here, you *can't* override it with `-dns.port` (coredns(1)), also see coredns-bind(7).

Specifying a **ZONE** *and* **PORT** combination multiple times for *different* servers will lead to
an error on startup.

When a query comes in, it is matched again all zones for all servers, the server with the longest
match on the query name will receive the query.

**PLUGIN** defines the plugin(s) we want to load into this server. This is optional as well, but as
server with no plugins will just return SERVFAIL for all queries. Each plugin can have a number of
properties than can have arguments, see the documentation for each plugin.

Comments are allowed and begin with an unquoted hash `#` and continue to the end of the line.
Comments may be started anywhere on a line.

Environment variables are supported and either the Unix or Windows form may be used: `{$ENV_VAR_1}`
or `{%ENV_VAR_2%}`.

You can use the `import` "plugin" (See coredns-import(7)) to include parts of other files.

If CoreDNS can’t find a Corefile to load it loads the following builtin one:

~~~ corefile
. {
    whoami
    log
}
~~~

## Import

You can use the `import` "plugin" to include parts of other files, see
<https://coredns.io/plugins/import>, and coredns-import(7).

## Snippets

If you want to reuse a snippet you can define one with and then use it with *import*.

~~~ corefile
(mysnippet) {
    log
    whoami
}

. {
    import mysnippet
}
~~~

## Examples

The **ZONE** is root zone `.`, the **PLUGIN** is *chaos*. The *chaos* plugin takes an (optional) argument:
`CoreDNS-001`. This text is returned on a CH class query: `dig CH TXT version.bind @localhost`.

~~~ corefile
. {
   chaos CoreDNS-001
}
~~~

When defining a new zone, you either create a new server, or add it to an existing one. Here we
define one server that handles two zones; that potentially chain different plugins:

~~~ corefile
example.org {
    whoami
}
org {
    whoami
}
~~~

Is identical to:

~~~ corefile
example.org org {
    whoami
}
~~~

Reverse zones can be specified as domain names:

~~~ corefile
0.0.10.in-addr.arpa {
    whoami
}
~~~

or by just using the CIDR notation:

~~~ corefile
10.0.0.0/24 {
    whoami
}
~~~

This also works on a non octet boundary:

~~~ corefile
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
Also see the [*import*](https://coredns.io/plugins/import)'s documentation and all the manual pages
for the plugins.

# chaos

The `chaos` middleware allows CoreDNS to response to TXT queries in CH class.
Useful for retrieving version or author information from the server.

## Syntax

~~~
chaos [version] [authors...]
~~~

* `version` the version to return, defaults to CoreDNS.
* `authors` what authors to return. No default.

Note this middleware can only be specified for a zone once. This is because it hijacks
the zones `version.bind`, `version.server`, `authors.bind`, `hostname.bind` and
`id.server`, which means it can only be routed to one middleware.

## Examples

~~~
chaos CoreDNS-001 "Miek Gieben" miek@miek.nl
~~~
